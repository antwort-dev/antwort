package files

import (
	"context"

	"github.com/rhuss/antwort/pkg/vectorstore"
)

// VectorIndexer is an alias for vectorstore.Backend.
// Kept for backward compatibility with existing code in this package.
type VectorIndexer = vectorstore.Backend

// VectorPoint is an alias for vectorstore.VectorPoint.
type VectorPoint = vectorstore.VectorPoint

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
