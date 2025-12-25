package resilience

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures before opening the circuit
	FailureThreshold int
	// SuccessThreshold is the number of successes needed to close the circuit from half-open
	SuccessThreshold int
	// Timeout is the duration the circuit stays open before transitioning to half-open
	Timeout time.Duration
	// MaxRequests is the max number of requests allowed through in half-open state
	MaxRequests int
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
		MaxRequests:      3,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name     string
	config   CircuitBreakerConfig
	state    CircuitState
	failures int
	successes int
	halfOpenRequests int
	lastFailure time.Time
	mu       sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the given name and config
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:   name,
		config: config,
		state:  CircuitClosed,
	}
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.currentState()
}

// currentState returns the current state, transitioning if needed (must hold lock)
func (cb *CircuitBreaker) currentState() CircuitState {
	switch cb.state {
	case CircuitOpen:
		// Check if timeout has passed
		if time.Since(cb.lastFailure) >= cb.config.Timeout {
			return CircuitHalfOpen
		}
		return CircuitOpen
	default:
		return cb.state
	}
}

// Execute runs the given function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	// Check if we should allow the request
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	// Execute the function
	err := fn()

	// Record the result
	cb.afterRequest(err)

	return err
}

// beforeRequest checks if the request should be allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.currentState()

	switch state {
	case CircuitClosed:
		return nil
	case CircuitOpen:
		return ErrCircuitOpen
	case CircuitHalfOpen:
		// Allow limited requests through
		if cb.halfOpenRequests >= cb.config.MaxRequests {
			return ErrCircuitOpen
		}
		cb.halfOpenRequests++
		// Transition state if this is the first half-open request
		if cb.state == CircuitOpen {
			cb.state = CircuitHalfOpen
			cb.halfOpenRequests = 1
		}
		return nil
	default:
		return nil
	}
}

// afterRequest records the result of the request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onFailure handles a failed request
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.successes = 0
	cb.lastFailure = time.Now()

	switch cb.currentState() {
	case CircuitClosed:
		if cb.failures >= cb.config.FailureThreshold {
			cb.state = CircuitOpen
			cb.halfOpenRequests = 0
		}
	case CircuitHalfOpen:
		// Any failure in half-open reopens the circuit
		cb.state = CircuitOpen
		cb.halfOpenRequests = 0
	}
}

// onSuccess handles a successful request
func (cb *CircuitBreaker) onSuccess() {
	switch cb.currentState() {
	case CircuitClosed:
		cb.failures = 0
	case CircuitHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.state = CircuitClosed
			cb.failures = 0
			cb.successes = 0
			cb.halfOpenRequests = 0
		}
	}
}

// Reset resets the circuit breaker to its initial state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenRequests = 0
}

// Metrics returns current circuit breaker metrics
func (cb *CircuitBreaker) Metrics() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return map[string]interface{}{
		"name":        cb.name,
		"state":       cb.currentState().String(),
		"failures":    cb.failures,
		"successes":   cb.successes,
		"lastFailure": cb.lastFailure,
	}
}
