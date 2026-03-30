package resilience

import (
	"context"
	"testing"
	"time"
)

func TestComputeBackoff(t *testing.T) {
	base := 100 * time.Millisecond
	max := 2 * time.Second

	// Attempt 1: should be in [100ms, 200ms) (base + jitter in [0, base)).
	for i := 0; i < 20; i++ {
		d := computeBackoff(1, base, max)
		if d < base || d >= 2*base {
			t.Errorf("attempt 1: backoff = %v, want [100ms, 200ms)", d)
		}
	}

	// Attempt 3: base * 4 = 400ms, range [400ms, 500ms).
	for i := 0; i < 20; i++ {
		d := computeBackoff(3, base, max)
		if d < 400*time.Millisecond || d >= 500*time.Millisecond {
			t.Errorf("attempt 3: backoff = %v, want [400ms, 500ms)", d)
		}
	}

	// Attempt 10: should be capped at max (2s).
	for i := 0; i < 20; i++ {
		d := computeBackoff(10, base, max)
		if d > max {
			t.Errorf("attempt 10: backoff = %v, exceeds max %v", d, max)
		}
	}
}

func TestSleepWithContext_Normal(t *testing.T) {
	start := time.Now()
	err := sleepWithContext(context.Background(), 50*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("sleepWithContext() error = %v, want nil", err)
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("sleepWithContext() returned too early: %v", elapsed)
	}
}

func TestSleepWithContext_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 20ms.
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := sleepWithContext(ctx, 5*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("sleepWithContext() expected error on cancelled context")
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("sleepWithContext() took too long after cancel: %v", elapsed)
	}
}

func TestSleepWithContext_ZeroDuration(t *testing.T) {
	err := sleepWithContext(context.Background(), 0)
	if err != nil {
		t.Fatalf("sleepWithContext(0) error = %v, want nil", err)
	}
}

func TestContextRemainingTime(t *testing.T) {
	// No deadline.
	_, ok := contextRemainingTime(context.Background())
	if ok {
		t.Fatal("contextRemainingTime(no deadline) ok = true, want false")
	}

	// With deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	remaining, ok := contextRemainingTime(ctx)
	if !ok {
		t.Fatal("contextRemainingTime(with deadline) ok = false, want true")
	}
	if remaining < 4*time.Second || remaining > 6*time.Second {
		t.Errorf("remaining = %v, want ~5s", remaining)
	}
}
