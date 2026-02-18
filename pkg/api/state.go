package api

import "fmt"

// ValidateResponseTransition checks whether a response status transition is valid.
// An empty "from" status represents the initial state before any status has been set.
// Terminal states (completed, failed, cancelled) do not allow outgoing transitions.
func ValidateResponseTransition(from, to ResponseStatus) *APIError {
	valid := map[ResponseStatus][]ResponseStatus{
		"":                       {ResponseStatusQueued, ResponseStatusInProgress},
		ResponseStatusQueued:     {ResponseStatusInProgress},
		ResponseStatusInProgress:     {ResponseStatusCompleted, ResponseStatusFailed, ResponseStatusCancelled, ResponseStatusRequiresAction},
		ResponseStatusRequiresAction: {}, // terminal
	}

	allowed, exists := valid[from]
	if !exists {
		return NewInvalidRequestError("status",
			fmt.Sprintf("invalid transition from %s to %s", from, to))
	}

	for _, s := range allowed {
		if s == to {
			return nil
		}
	}

	return NewInvalidRequestError("status",
		fmt.Sprintf("invalid transition from %s to %s", from, to))
}

// ValidateItemTransition checks whether an item status transition is valid.
// An empty "from" status represents the initial state before any status has been set.
// Terminal states (completed, incomplete, failed) do not allow outgoing transitions.
func ValidateItemTransition(from, to ItemStatus) *APIError {
	valid := map[ItemStatus][]ItemStatus{
		"":                   {ItemStatusInProgress},
		ItemStatusInProgress: {ItemStatusCompleted, ItemStatusIncomplete, ItemStatusFailed},
	}

	allowed, exists := valid[from]
	if !exists {
		return NewInvalidRequestError("status",
			fmt.Sprintf("invalid transition from %s to %s", from, to))
	}

	for _, s := range allowed {
		if s == to {
			return nil
		}
	}

	return NewInvalidRequestError("status",
		fmt.Sprintf("invalid transition from %s to %s", from, to))
}
