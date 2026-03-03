// Package vectorstore defines the unified interface for vector database backends.
//
// All vector store operations (collection management, search, upsert, delete)
// are behind a single Backend interface. Adapters for specific databases
// (Qdrant, pgvector, in-memory) live in sub-packages.
package vectorstore

import "context"

// Backend is the unified interface for vector store operations.
// It combines read operations (search) and write operations (upsert, delete)
// that were previously split across two interfaces.
type Backend interface {
	// CreateCollection creates a new vector collection with the given name and dimensions.
	CreateCollection(ctx context.Context, name string, dimensions int) error

	// DeleteCollection removes a vector collection by name.
	DeleteCollection(ctx context.Context, name string) error

	// Search performs a nearest-neighbor search in the named collection.
	Search(ctx context.Context, collection string, vector []float32, maxResults int) ([]SearchMatch, error)

	// UpsertPoints inserts or updates vector points in the named collection.
	UpsertPoints(ctx context.Context, collection string, points []VectorPoint) error

	// DeletePointsByFile removes all points associated with the given file ID
	// from the named collection.
	DeletePointsByFile(ctx context.Context, collection string, fileID string) error
}

// SearchMatch represents a single search result from the vector store.
type SearchMatch struct {
	DocumentID string
	Score      float32
	Content    string
	Metadata   map[string]string
}

// VectorPoint represents a chunk prepared for vector store insertion.
type VectorPoint struct {
	ID       string
	Vector   []float32
	Metadata map[string]string
}
