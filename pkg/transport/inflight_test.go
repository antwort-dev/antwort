package transport

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestInFlightRegistryRegisterAndCancel(t *testing.T) {
	r := NewInFlightRegistry()

	cancelled := false
	r.Register("resp_abc123", func() { cancelled = true })

	ok := r.Cancel("resp_abc123")
	if !ok {
		t.Error("Cancel should return true for registered ID")
	}
	if !cancelled {
		t.Error("cancel function should have been called")
	}

	// Second cancel should return false (already removed).
	ok = r.Cancel("resp_abc123")
	if ok {
		t.Error("Cancel should return false after already cancelled")
	}
}

func TestInFlightRegistryCancelUnknown(t *testing.T) {
	r := NewInFlightRegistry()

	ok := r.Cancel("resp_nonexistent")
	if ok {
		t.Error("Cancel should return false for unknown ID")
	}
}

func TestInFlightRegistryRemove(t *testing.T) {
	r := NewInFlightRegistry()

	cancelled := false
	r.Register("resp_abc123", func() { cancelled = true })

	r.Remove("resp_abc123")

	ok := r.Cancel("resp_abc123")
	if ok {
		t.Error("Cancel should return false after Remove")
	}
	if cancelled {
		t.Error("cancel function should not have been called by Remove")
	}
}

func TestInFlightRegistryRemoveUnknown(t *testing.T) {
	r := NewInFlightRegistry()
	// Should not panic.
	r.Remove("resp_nonexistent")
}

func TestInFlightRegistryConcurrentAccess(t *testing.T) {
	r := NewInFlightRegistry()
	var cancelCount atomic.Int64
	const numEntries = 100

	// Register entries concurrently.
	var wg sync.WaitGroup
	for i := 0; i < numEntries; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			r.Register(id, func() { cancelCount.Add(1) })
		}(idForIndex(i))
	}
	wg.Wait()

	// Cancel half concurrently.
	for i := 0; i < numEntries/2; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			r.Cancel(id)
		}(idForIndex(i))
	}
	wg.Wait()

	if cancelCount.Load() != numEntries/2 {
		t.Errorf("expected %d cancellations, got %d", numEntries/2, cancelCount.Load())
	}

	// Remove the other half concurrently.
	for i := numEntries / 2; i < numEntries; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			r.Remove(id)
		}(idForIndex(i))
	}
	wg.Wait()
}

func idForIndex(i int) string {
	return "resp_" + string(rune('A'+i%26)) + string(rune('0'+i/26))
}
