package resilience

import (
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	// Should start closed.
	if cb.State() != StateClosed {
		t.Fatalf("initial state = %s, want closed", StateName(cb.State()))
	}

	// Record failures up to threshold.
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("Allow() = false at failure %d, want true (closed)", i)
		}
		cb.RecordFailure()
	}

	// Should now be open.
	if cb.State() != StateOpen {
		t.Fatalf("state after 3 failures = %s, want open", StateName(cb.State()))
	}

	// Should reject requests.
	if cb.Allow() {
		t.Fatal("Allow() = true when open, want false")
	}
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure() // trip to open
	if cb.State() != StateOpen {
		t.Fatalf("state = %s, want open", StateName(cb.State()))
	}

	// Wait for reset timeout.
	time.Sleep(60 * time.Millisecond)

	// Should transition to half-open and allow one request.
	if !cb.Allow() {
		t.Fatal("Allow() = false after reset timeout, want true (half-open probe)")
	}
	if cb.State() != StateHalfOpen {
		t.Fatalf("state = %s, want half-open", StateName(cb.State()))
	}
}

func TestCircuitBreaker_HalfOpenToClosedOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure() // trip to open
	time.Sleep(60 * time.Millisecond)
	cb.Allow() // transition to half-open

	cb.RecordSuccess()

	if cb.State() != StateClosed {
		t.Fatalf("state after success in half-open = %s, want closed", StateName(cb.State()))
	}
	if cb.ConsecutiveFailures() != 0 {
		t.Fatalf("failures = %d, want 0 after success", cb.ConsecutiveFailures())
	}
}

func TestCircuitBreaker_HalfOpenToOpenOnFailure(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure() // trip to open
	time.Sleep(60 * time.Millisecond)
	cb.Allow() // transition to half-open

	cb.RecordFailure() // probe failed

	if cb.State() != StateOpen {
		t.Fatalf("state after failure in half-open = %s, want open", StateName(cb.State()))
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(5, time.Second)

	// Record some failures (below threshold).
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.ConsecutiveFailures() != 2 {
		t.Fatalf("failures = %d, want 2", cb.ConsecutiveFailures())
	}

	// Success resets.
	cb.RecordSuccess()
	if cb.ConsecutiveFailures() != 0 {
		t.Fatalf("failures after success = %d, want 0", cb.ConsecutiveFailures())
	}
}

func TestCircuitBreaker_HalfOpenRejectsSecondRequest(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure() // trip to open
	time.Sleep(60 * time.Millisecond)

	// First request transitions to half-open.
	if !cb.Allow() {
		t.Fatal("first Allow() after timeout = false, want true")
	}

	// Second request should be rejected (only one probe allowed).
	if cb.Allow() {
		t.Fatal("second Allow() in half-open = true, want false")
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(100, time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.Allow()
			cb.RecordFailure()
			cb.Allow()
			cb.RecordSuccess()
		}()
	}
	wg.Wait()

	// Should not panic; state should be valid.
	state := cb.State()
	if state != StateClosed && state != StateOpen && state != StateHalfOpen {
		t.Fatalf("invalid state %d after concurrent access", state)
	}
}

func TestStateName(t *testing.T) {
	tests := []struct {
		state int32
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{99, "unknown"},
	}
	for _, tt := range tests {
		got := StateName(tt.state)
		if got != tt.want {
			t.Errorf("StateName(%d) = %q, want %q", tt.state, got, tt.want)
		}
	}
}
