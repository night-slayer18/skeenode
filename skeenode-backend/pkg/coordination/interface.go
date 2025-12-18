package coordination

import (
	"context"
)

// Coordinator handles distributed coordination tasks.
type Coordinator interface {
	// NewElection creates a new election instance for a given campaign name.
	NewElection(name string) Election
	
	// Close terminates the coordinator connection.
	Close() error
}

// Election represents a single leader election campaign.
type Election interface {
	// Campaign starts the process of trying to become leader.
	// It blocks until leadership is acquired or an error occurs.
	Campaign(ctx context.Context, value string) error
	
	// Resign releases leadership.
	Resign(ctx context.Context) error
	
	// Leader returns the current leader's value (if any).
	Leader(ctx context.Context) (string, error)
}
