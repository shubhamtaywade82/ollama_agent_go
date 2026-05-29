package reliability

import (
	"sync"
	"time"
)

// State of the circuit breaker.
type State int

const (
	StateClosed   State = iota // normal — requests pass through
	StateOpen                  // tripped — requests blocked
	StateHalfOpen              // probe — one request allowed to test recovery
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the standard three-state breaker pattern.
type CircuitBreaker struct {
	name          string
	failureThresh int
	successThresh int
	openTimeout   time.Duration

	mu        sync.Mutex
	state     State
	failures  int
	successes int
	openedAt  time.Time
}

// NewCircuitBreaker creates a breaker with the given thresholds.
// openTimeout is how long to wait in Open state before probing.
// successThresh is how many consecutive successes in HalfOpen state reset to Closed.
func NewCircuitBreaker(name string, failureThresh int, openTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:          name,
		failureThresh: failureThresh,
		successThresh: 1,
		openTimeout:   openTimeout,
	}
}

// Allow returns false when the circuit is Open (and the probe window hasn't expired).
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.openedAt) >= cb.openTimeout {
			cb.state = StateHalfOpen
			cb.successes = 0
			return true // allow the probe request
		}
		return false
	case StateHalfOpen:
		return true
	}
	return true
}

// RecordSuccess should be called after a successful call.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		cb.failures = 0
	case StateHalfOpen:
		cb.successes++
		if cb.successes >= cb.successThresh {
			cb.state = StateClosed
			cb.failures = 0
		}
	}
}

// RecordFailure should be called after a failed call.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		cb.failures++
		if cb.failures >= cb.failureThresh {
			cb.state = StateOpen
			cb.openedAt = time.Now()
		}
	case StateHalfOpen:
		// Probe failed — reopen.
		cb.state = StateOpen
		cb.openedAt = time.Now()
	}
}

// CurrentState returns the current breaker state.
func (cb *CircuitBreaker) CurrentState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}
