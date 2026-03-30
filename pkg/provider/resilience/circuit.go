package resilience

import (
	"sync/atomic"
	"time"
)

// Circuit breaker states.
const (
	StateClosed   int32 = 0
	StateOpen     int32 = 1
	StateHalfOpen int32 = 2
)

// StateName returns a human-readable name for a circuit breaker state.
func StateName(state int32) string {
	switch state {
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

// CircuitBreaker implements a three-state circuit breaker using atomic operations
// for lock-free concurrency.
//
// States:
//   - Closed: normal operation, requests pass through
//   - Open: backend is down, requests fail immediately
//   - Half-Open: one probe request allowed to test recovery
type CircuitBreaker struct {
	state           atomic.Int32
	failures        atomic.Int64
	lastFailureTime atomic.Int64 // unix nanoseconds

	threshold    int64
	resetTimeout time.Duration
}

// NewCircuitBreaker creates a circuit breaker with the given failure threshold
// and reset timeout.
func NewCircuitBreaker(threshold int64, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

// Allow checks whether a request is allowed to proceed.
//
// In closed state, all requests are allowed.
// In open state, requests are rejected unless the reset timeout has elapsed,
// in which case the circuit transitions to half-open.
// In half-open state, only one probe request is allowed (via CAS).
func (cb *CircuitBreaker) Allow() bool {
	state := cb.state.Load()

	switch state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if reset timeout has elapsed.
		lastFail := cb.lastFailureTime.Load()
		if time.Since(time.Unix(0, lastFail)) < cb.resetTimeout {
			return false
		}
		// Try to transition to half-open (only one goroutine wins).
		if cb.state.CompareAndSwap(StateOpen, StateHalfOpen) {
			return true
		}
		// Another goroutine already transitioned; check new state.
		return cb.state.Load() == StateClosed

	case StateHalfOpen:
		// Only one probe request in half-open. Other requests fail fast.
		return false

	default:
		return false
	}
}

// RecordSuccess records a successful request. In closed state, it resets
// the consecutive failure count. In half-open state, it closes the circuit.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.failures.Store(0)
	// If half-open, transition to closed.
	cb.state.CompareAndSwap(StateHalfOpen, StateClosed)
}

// RecordFailure records a failed request. In closed state, it increments
// the failure count and opens the circuit if the threshold is reached.
// In half-open state, it reopens the circuit.
func (cb *CircuitBreaker) RecordFailure() {
	cb.lastFailureTime.Store(time.Now().UnixNano())

	state := cb.state.Load()
	switch state {
	case StateClosed:
		count := cb.failures.Add(1)
		if count >= cb.threshold {
			cb.state.CompareAndSwap(StateClosed, StateOpen)
		}

	case StateHalfOpen:
		// Probe failed, reopen.
		cb.state.CompareAndSwap(StateHalfOpen, StateOpen)
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() int32 {
	return cb.state.Load()
}

// ConsecutiveFailures returns the current consecutive failure count.
func (cb *CircuitBreaker) ConsecutiveFailures() int64 {
	return cb.failures.Load()
}

