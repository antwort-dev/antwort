package filesearch

import "context"

// VectorStoreBackend is the pluggable interface for vector databases.
// All vector compute (storage, indexing, search) happens externally;
// this interface abstracts the specific vector DB implementation.
type VectorStoreBackend interface {
	// CreateCollection creates a new vector collection with the given name and dimensions.
	CreateCollection(ctx context.Context, name string, dimensions int) error

	// DeleteCollection removes a vector collection by name.
	DeleteCollection(ctx context.Context, name string) error

	// Search performs a nearest-neighbor search in the named collection.
	Search(ctx context.Context, collection string, vector []float32, maxResults int) ([]SearchMatch, error)
}

// SearchMatch represents a single search result from the vector store.
type SearchMatch struct {
	DocumentID string
	Score      float32
	Content    string
	Metadata   map[string]string
}
