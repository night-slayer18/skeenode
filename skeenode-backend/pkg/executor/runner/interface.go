package runner

import (
	"context"
	"time"
)

// Result captures the outcome of a job execution.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	Error    error // detailed go error if any
}

// JobRunner defines the interface for executing a single job.
type JobRunner interface {
	// Run executes the command with the given arguments within the context.
	// It returns a Result containing exit code and logs.
	Run(ctx context.Context, cmd string, args []string) Result
}
