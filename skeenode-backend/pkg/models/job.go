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
	Attempt       int             `json:"attempt" gorm:"default:1"`
	ExitCode      int             `json:"exit_code"`
	OutputURI     string          `json:"output_uri"`
	
	// Transient field for transport to Executor (not stored in DB execution table)
	JobCommand    string          `json:"command" gorm:"-"` 
}

func (e *Execution) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return
}

// DependencyType defines the relationship strength between jobs.
type DependencyType string

const (
	DependencyTypeHard        DependencyType = "HARD"        // Child waits for parent success
	DependencyTypeSoft        DependencyType = "SOFT"        // Child waits for parent completion (any status)
	DependencyTypeConditional DependencyType = "CONDITIONAL" // Child runs based on parent outcome
)

// Dependency represents a relationship between two jobs where the child
// depends on the parent's execution. Used for DAG-based job scheduling.
type Dependency struct {
	ParentJobID     uuid.UUID      `json:"parent_job_id" gorm:"type:uuid;primaryKey"`
	ChildJobID      uuid.UUID      `json:"child_job_id" gorm:"type:uuid;primaryKey"`
	Type            DependencyType `json:"type" gorm:"type:varchar(20);not null;default:'HARD'"`
	ConfidenceScore float64        `json:"confidence_score" gorm:"default:1.0"` // 1.0 for manual, <1.0 for ML-detected
	IsAutoDetected  bool           `json:"is_auto_detected" gorm:"default:false"`
	CreatedAt       time.Time      `json:"created_at"`

	// Foreign key relationships
	ParentJob *Job `json:"-" gorm:"foreignKey:ParentJobID;constraint:OnDelete:CASCADE"`
	ChildJob  *Job `json:"-" gorm:"foreignKey:ChildJobID;constraint:OnDelete:CASCADE"`
}

