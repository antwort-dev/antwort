package resilience

import (
	"context"
	"math"
	"math/rand/v2"
	"time"
)

// RetryPolicy defines retry behavior for a request.
type RetryPolicy struct {
	MaxAttempts   int
	BackoffBase   time.Duration
	BackoffMax    time.Duration
	RetryAfterMax time.Duration
}

// computeBackoff calculates the wait duration for a given attempt (1-indexed).
// Uses exponential backoff with jitter: min(base * 2^(attempt-1) + jitter, max).
func computeBackoff(attempt int, base, max time.Duration) time.Duration {
	exp := math.Pow(2, float64(attempt-1))
	wait := time.Duration(float64(base) * exp)

	// Add jitter in [0, base).
	if base > 0 {
		jitter := time.Duration(rand.N(int64(base)))
		wait += jitter
	}

	if wait > max {
		wait = max
	}
	return wait
}

// sleepWithContext sleeps for the given duration or until the context is cancelled.
// Returns ctx.Err() if the context is cancelled before the sleep completes.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// contextRemainingTime returns the time remaining before the context deadline.
// Returns 0 and false if the context has no deadline.
func contextRemainingTime(ctx context.Context) (time.Duration, bool) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0, false
	}
	return time.Until(deadline), true
}
