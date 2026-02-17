package transport

import (
	"context"
	"sync"
)

// InFlightRegistry tracks in-flight streaming responses for explicit
// cancellation. It maps response IDs to their cancel functions, allowing
// a DELETE request to cancel a streaming response that is still in progress.
//
// All methods are safe for concurrent access.
type InFlightRegistry struct {
	mu      sync.Mutex
	entries map[string]context.CancelFunc
}

// NewInFlightRegistry creates a new empty registry.
func NewInFlightRegistry() *InFlightRegistry {
	return &InFlightRegistry{
		entries: make(map[string]context.CancelFunc),
	}
}

// Register adds an in-flight response to the registry. The cancel function
// will be called if the response is explicitly cancelled via DELETE.
func (r *InFlightRegistry) Register(id string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[id] = cancel
}

// Cancel cancels an in-flight response by calling its cancel function.
// Returns true if the response was found and cancelled, false if the ID
// was not registered (either already completed or never existed).
func (r *InFlightRegistry) Cancel(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	cancel, ok := r.entries[id]
	if !ok {
		return false
	}
	cancel()
	delete(r.entries, id)
	return true
}

// Remove removes a response from the registry without cancelling it.
// Called when a streaming response completes normally.
func (r *InFlightRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, id)
}
