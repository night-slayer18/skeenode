package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "skeenode/pkg/resilience"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker("test", DefaultCircuitBreakerConfig())
	
	if cb.State() != CircuitClosed {
		t.Errorf("expected initial state to be Closed, got %v", cb.State())
	}
}

func TestCircuitBreaker_OpensAfterFailures(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
		MaxRequests:      1,
	}
	cb := NewCircuitBreaker("test", config)
	
	// Cause failures
	for i := 0; i < 3; i++ {
		_ = cb.Execute(context.Background(), func() error {
			return errors.New("test error")
		})
	}
	
	if cb.State() != CircuitOpen {
		t.Errorf("expected state to be Open after %d failures, got %v", config.FailureThreshold, cb.State())
	}
}

func TestCircuitBreaker_RejectsWhenOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          1 * time.Second,
		MaxRequests:      1,
	}
	cb := NewCircuitBreaker("test", config)
	
	// Open the circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("test error")
	})
	
	// Attempt another request
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	
	if err != ErrCircuitOpen {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
		MaxRequests:      1,
	}
	cb := NewCircuitBreaker("test", config)
	
	// Open the circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("test error")
	})
	
	// Wait for timeout
	time.Sleep(60 * time.Millisecond)
	
	// Should be half-open now
	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected state to be HalfOpen after timeout, got %v", cb.State())
	}
}

func TestCircuitBreaker_ClosesAfterSuccessInHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          50 * time.Millisecond,
		MaxRequests:      2,
	}
	cb := NewCircuitBreaker("test", config)
	
	// Open the circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("test error")
	})
	
	// Wait for timeout
	time.Sleep(60 * time.Millisecond)
	
	// Success in half-open should close
	_ = cb.Execute(context.Background(), func() error {
		return nil
	})
	
	if cb.State() != CircuitClosed {
		t.Errorf("expected state to be Closed after success in HalfOpen, got %v", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          1 * time.Second,
		MaxRequests:      1,
	}
	cb := NewCircuitBreaker("test", config)
	
	// Open the circuit
	_ = cb.Execute(context.Background(), func() error {
		return errors.New("test error")
	})
	
	cb.Reset()
	
	if cb.State() != CircuitClosed {
		t.Errorf("expected state to be Closed after Reset, got %v", cb.State())
	}
}

func TestCircuitBreaker_Metrics(t *testing.T) {
	cb := NewCircuitBreaker("test-metrics", DefaultCircuitBreakerConfig())
	
	metrics := cb.Metrics()
	
	if metrics["name"] != "test-metrics" {
		t.Errorf("expected name to be 'test-metrics', got %v", metrics["name"])
	}
	if metrics["state"] != "closed" {
		t.Errorf("expected state to be 'closed', got %v", metrics["state"])
	}
}
