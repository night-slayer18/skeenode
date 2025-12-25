package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"skeenode/pkg/models"
	"skeenode/pkg/storage"
)

type PostgresStore struct {
	db *gorm.DB
}

// NewPostgresStore initializes GORM connection and AutoMigrates schemas.
func NewPostgresStore(connString string) (*PostgresStore, error) {
	// Use GORM configuration for "Pro" logging and performance
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		PrepareStmt: true, // Cache prepared statements for performance
	}

	db, err := gorm.Open(postgres.Open(connString), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Optimize Connection Pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)


	err = db.AutoMigrate(&models.Job{}, &models.Execution{}, &models.Dependency{})
	if err != nil {
		return nil, fmt.Errorf("schema migration failed: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// CreateJob persists a new job using GORM.
func (s *PostgresStore) CreateJob(ctx context.Context, job *models.Job) error {
	result := s.db.WithContext(ctx).Create(job)
	if result.Error != nil {
		return fmt.Errorf("failed to create job: %w", result.Error)
	}
	return nil
}

// GetJob retrieves a job by ID.
func (s *PostgresStore) GetJob(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	var job models.Job
	result := s.db.WithContext(ctx).First(&job, "id = ?", id)
	
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, storage.ErrNotFound
		}
		return nil, result.Error
	}
	return &job, nil
}

// ListAllJobs returns all active jobs with pagination.
func (s *PostgresStore) ListAllJobs(ctx context.Context, limit, offset int) ([]models.Job, error) {
	var jobs []models.Job
	
	result := s.db.WithContext(ctx).
		Where("status != ?", models.JobStatusArchived).
		Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&jobs)
		
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", result.Error)
	}
	return jobs, nil
}

// ListDueJobs finds jobs that need to be run using fluent API.
func (s *PostgresStore) ListDueJobs(ctx context.Context, limit int) ([]models.Job, error) {
	var jobs []models.Job
	
	// SELECT * FROM jobs WHERE status = 'ACTIVE' AND next_run_at <= NOW() ORDER BY next_run_at ASC LIMIT ?
	result := s.db.WithContext(ctx).
		Where("status = ?", models.JobStatusActive).
		Where("next_run_at <= ?", time.Now()).
		Order("next_run_at asc").
		Limit(limit).
		Find(&jobs)
		
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list due jobs: %w", result.Error)
	}
	return jobs, nil
}

// UpdateNextRun updates the scheduling timestamp.
func (s *PostgresStore) UpdateNextRun(ctx context.Context, id uuid.UUID, nextRun time.Time) error {
	result := s.db.WithContext(ctx).
		Model(&models.Job{}).
		Where("id = ?", id).
		Update("next_run_at", nextRun)
		
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// CreateExecution records a new execution run.
func (s *PostgresStore) CreateExecution(ctx context.Context, exec *models.Execution) error {
	result := s.db.WithContext(ctx).Create(exec)
	if result.Error != nil {
		return fmt.Errorf("failed to create execution: %w", result.Error)
	}
	return nil
}

// UpdateRunState marks an execution as running with the assigned node.
func (s *PostgresStore) UpdateRunState(ctx context.Context, id uuid.UUID, nodeID string, startedAt time.Time) error {
	result := s.db.WithContext(ctx).
		Model(&models.Execution{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     models.ExecutionRunning,
			"node_id":    nodeID,
			"started_at": startedAt,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update run state: %w", result.Error)
	}
	return nil
}

// UpdateResult marks an execution as finished.
func (s *PostgresStore) UpdateResult(ctx context.Context, id uuid.UUID, status models.ExecutionStatus, exitCode int, outputURI string) error {
	now := time.Now()
	result := s.db.WithContext(ctx).
		Model(&models.Execution{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       status,
			"exit_code":    exitCode,
			"output_uri":   outputURI,
			"completed_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update result: %w", result.Error)
	}
	return nil
}

// MarkOrphansAsFailed updates executions stuck in RUNNING state on dead nodes.
func (s *PostgresStore) MarkOrphansAsFailed(ctx context.Context, activeNodeIDs []string) (int64, error) {
	// If no nodes are active, ALL running jobs are orphans.
	// However, we must be careful not to mark jobs that just started before the node registered (race condition).
	// But usually, node registers BEFORE taking jobs.
	
	query := s.db.WithContext(ctx).
		Model(&models.Execution{}).
		Where("status = ?", models.ExecutionRunning)

	if len(activeNodeIDs) > 0 {
		query = query.Where("node_id NOT IN ?", activeNodeIDs)
	}
	
	result := query.Updates(map[string]interface{}{
		"status":       models.ExecutionFailed,
		"exit_code":    -1,
		"completed_at": time.Now(),
	})
	return result.RowsAffected, result.Error
}

// ListRecentFailures returns executions that failed since a given time.
func (s *PostgresStore) ListRecentFailures(ctx context.Context, since time.Time, limit int) ([]models.Execution, error) {
	var execs []models.Execution
	result := s.db.WithContext(ctx).
		Where("status = ?", models.ExecutionFailed).
		Where("completed_at >= ?", since).
		Order("completed_at desc").
		Limit(limit).
		Find(&execs)
		
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list recent failures: %w", result.Error)
	}
	return execs, nil
}

// --- DependencyStore Implementation ---

// CreateDependency adds a new job dependency relationship.
func (s *PostgresStore) CreateDependency(ctx context.Context, dep *models.Dependency) error {
	result := s.db.WithContext(ctx).Create(dep)
	if result.Error != nil {
		return fmt.Errorf("failed to create dependency: %w", result.Error)
	}
	return nil
}

// GetDependencies returns all dependencies where the given job is the child.
func (s *PostgresStore) GetDependencies(ctx context.Context, childJobID uuid.UUID) ([]models.Dependency, error) {
	var deps []models.Dependency
	result := s.db.WithContext(ctx).
		Where("child_job_id = ?", childJobID).
		Find(&deps)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get dependencies: %w", result.Error)
	}
	return deps, nil
}

// GetDependents returns all jobs that depend on the given job (children).
func (s *PostgresStore) GetDependents(ctx context.Context, parentJobID uuid.UUID) ([]models.Dependency, error) {
	var deps []models.Dependency
	result := s.db.WithContext(ctx).
		Where("parent_job_id = ?", parentJobID).
		Find(&deps)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get dependents: %w", result.Error)
	}
	return deps, nil
}

// DeleteDependency removes a specific dependency relationship.
func (s *PostgresStore) DeleteDependency(ctx context.Context, parentJobID, childJobID uuid.UUID) error {
	result := s.db.WithContext(ctx).
		Where("parent_job_id = ? AND child_job_id = ?", parentJobID, childJobID).
		Delete(&models.Dependency{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete dependency: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return storage.ErrNotFound
	}
	return nil
}
