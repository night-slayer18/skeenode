package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JobType defines the execution environment of a job.
type JobType string

const (
	JobTypeShell  JobType = "SHELL"
	JobTypeDocker JobType = "DOCKER"
	JobTypeHTTP   JobType = "HTTP"
)

// JobStatus represents the state of a job in the system.
type JobStatus string

const (
	JobStatusActive   JobStatus = "ACTIVE"
	JobStatusPaused   JobStatus = "PAUSED"
	JobStatusArchived JobStatus = "ARCHIVED"
)

// JSONB structures need to implement Scanner/Valuer for GORM

type RetryPolicy struct {
	MaxRetries      int    `json:"max_retries"`
	BackoffStrategy string `json:"backoff_strategy"`
	InitialInterval string `json:"initial_interval"`
	MaxInterval     string `json:"max_interval"`
}

func (r *RetryPolicy) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, r)
}

func (r RetryPolicy) Value() (driver.Value, error) {
	return json.Marshal(r)
}

type ResourceConstraints struct {
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	Timeout string `json:"timeout"`
}

func (c *ResourceConstraints) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, c)
}

func (c ResourceConstraints) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Job represents a scheduled unit of work.
// Using GORM keys (primaryKey, type:uuid)
type Job struct {
	ID          uuid.UUID           `json:"id" gorm:"type:uuid;primaryKey"`
	Name        string              `json:"name" gorm:"not null"`
	Schedule    string              `json:"schedule" gorm:"not null"`
	Command     string              `json:"command" gorm:"not null"`
	Type        JobType             `json:"type" gorm:"type:varchar(20);not null"`
	OwnerID     string              `json:"owner_id"`
	RetryPolicy RetryPolicy         `json:"retry_policy" gorm:"type:jsonb"`
	Constraints ResourceConstraints `json:"constraints" gorm:"type:jsonb"`
	Status      JobStatus           `json:"status" gorm:"type:varchar(20);default:'ACTIVE'"`
	NextRunAt   *time.Time          `json:"next_run_at" gorm:"index"` // Index for fast polling
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	DeletedAt   gorm.DeletedAt      `json:"-" gorm:"index"` // Soft Delete support
}

// BeforeCreate hook to generate UUID if not present
func (j *Job) BeforeCreate(tx *gorm.DB) (err error) {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	return
}

type ExecutionStatus string

const (
	ExecutionPending   ExecutionStatus = "PENDING"
	ExecutionRunning   ExecutionStatus = "RUNNING"
	ExecutionSuccess   ExecutionStatus = "SUCCESS"
	ExecutionFailed    ExecutionStatus = "FAILED"
	ExecutionCancelled ExecutionStatus = "CANCELLED"
)

type Execution struct {
	ID            uuid.UUID       `json:"id" gorm:"type:uuid;primaryKey"`
	JobID         uuid.UUID       `json:"job_id" gorm:"type:uuid;not null;index:idx_job_scheduled,unique"`
	NodeID        *string         `json:"node_id"`
	ScheduledAt   time.Time       `json:"scheduled_at" gorm:"not null;index:idx_job_scheduled,unique"`
	StartedAt     *time.Time      `json:"started_at"`
	CompletedAt   *time.Time      `json:"completed_at"`
	Status        ExecutionStatus `json:"status" gorm:"type:varchar(20);default:'PENDING'"`
	ExitCode      int             `json:"exit_code"`
	OutputURI     string          `json:"output_uri"`
	// ResourceUsage map[string]any is trickier in GORM without a wrapper, simplifying for now or use JSONB wrapper like above
}

func (e *Execution) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return
}
