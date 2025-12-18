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
}

// ExecutionStore defines the data access layer for Execution history.
type ExecutionStore interface {
	CreateExecution(ctx context.Context, exec *models.Execution) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.ExecutionStatus, exitCode int) error
}
