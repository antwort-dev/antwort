package storage

import "errors"

// Sentinel errors for storage operations.
var (
	// ErrNotFound is returned when a response does not exist or has been deleted.
	ErrNotFound = errors.New("response not found")

	// ErrConflict is returned when a response with the given ID already exists.
	ErrConflict = errors.New("response already exists")
)
