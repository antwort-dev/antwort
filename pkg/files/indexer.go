package files

import "context"

// VectorIndexer writes and deletes vector points in a collection.
// It complements the read-only VectorStoreBackend from filesearch.
type VectorIndexer interface {
	// UpsertPoints inserts or updates vector points in the named collection.
	UpsertPoints(ctx context.Context, collection string, points []VectorPoint) error

	// DeletePointsByFile removes all points associated with the given file ID
	// from the named collection.
	DeletePointsByFile(ctx context.Context, collection string, fileID string) error
}

// Embedder converts text into embedding vectors. This mirrors filesearch.EmbeddingClient
// to avoid an import cycle between pkg/files and pkg/tools/builtins/filesearch.
type Embedder interface {
	// Embed converts a batch of text strings into embedding vectors.
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the dimensionality of the embedding vectors.
	Dimensions() int
}

// VectorStoreLookup retrieves the collection name for a given vector store ID.
// This avoids importing filesearch's MetadataStore directly.
type VectorStoreLookup func(vsID string) (collectionName string, err error)
