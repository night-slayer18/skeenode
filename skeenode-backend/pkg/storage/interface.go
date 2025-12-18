package storage

import (
	"context"
	"errors"
	"time"

	"skeenode/pkg/models"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("record already exists")
)

// JobStore defines the data access layer for Job management.
type JobStore interface {
	// CreateJob persists a new job.
	CreateJob(ctx context.Context, job *models.Job) error
	
	// GetJob retrieves a job by ID.
	GetJob(ctx context.Context, id uuid.UUID) (*models.Job, error)
	
	// ListDueJobs finds jobs that need to be scheduled (NextRunAt <= Now).
	ListDueJobs(ctx context.Context, limit int) ([]models.Job, error)
	
	// UpdateNextRun sets the next execution time for a job.
	UpdateNextRun(ctx context.Context, id uuid.UUID, nextRun time.Time) error
}

// Queue defines the mechanism for dispatching jobs to executors.
type Queue interface {
	// Push adds a job to the pending queue.
	Push(ctx context.Context, execution *models.Execution) error

	// Pop retrieves a job from the queue for a specific consumer group.
	Pop(ctx context.Context, group string, consumer string) (string, *models.Execution, error)

	// Ack acknowledges a job execution as processed.
	Ack(ctx context.Context, group string, msgID string) error

	// EnsureGroup ensures the consumer group exists.
	EnsureGroup(ctx context.Context, group string) error
}

// ExecutionStore defines the data access layer for Execution history.
type ExecutionStore interface {
	CreateExecution(ctx context.Context, exec *models.Execution) error
	
	// UpdateRunState marks an execution as running.
	UpdateRunState(ctx context.Context, id uuid.UUID, startedAt time.Time) error
	
	// UpdateResult marks an execution as finished.
	UpdateResult(ctx context.Context, id uuid.UUID, status models.ExecutionStatus, exitCode int, outputURI string) error

	// MarkOrphansAsFailed updates executions stuck in RUNNING state on dead nodes.
	MarkOrphansAsFailed(ctx context.Context, activeNodeIDs []string) (int64, error)
	
	// ListRecentFailures returns executions that failed since a given time.
	ListRecentFailures(ctx context.Context, since time.Time, limit int) ([]models.Execution, error)
}
