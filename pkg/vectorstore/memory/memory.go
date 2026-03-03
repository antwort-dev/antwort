// Package memory implements an in-memory vectorstore.Backend for testing.
// It uses brute-force cosine similarity search with no index structure.
package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/rhuss/antwort/pkg/vectorstore"
)

// Backend is an in-memory vector store for testing and development.
type Backend struct {
	mu          sync.RWMutex
	collections map[string]*collection
}

type collection struct {
	dimensions int
	points     map[string]*storedPoint
}

type storedPoint struct {
	vector   []float32
	metadata map[string]string
}

// Compile-time check.
var _ vectorstore.Backend = (*Backend)(nil)

// New creates a new in-memory vector backend.
func New() *Backend {
	return &Backend{
		collections: make(map[string]*collection),
	}
}

func (b *Backend) CreateCollection(_ context.Context, name string, dimensions int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.collections[name]; exists {
		return fmt.Errorf("collection %q already exists", name)
	}

	b.collections[name] = &collection{
		dimensions: dimensions,
		points:     make(map[string]*storedPoint),
	}
	return nil
}

func (b *Backend) DeleteCollection(_ context.Context, name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.collections, name)
	return nil
}

func (b *Backend) Search(_ context.Context, collectionName string, vector []float32, maxResults int) ([]vectorstore.SearchMatch, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	coll, ok := b.collections[collectionName]
	if !ok {
		return nil, fmt.Errorf("collection %q not found", collectionName)
	}

	type scored struct {
		id    string
		score float32
		point *storedPoint
	}

	var results []scored
	for id, pt := range coll.points {
		score := cosineSimilarity(vector, pt.vector)
		results = append(results, scored{id: id, score: score, point: pt})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	matches := make([]vectorstore.SearchMatch, len(results))
	for i, r := range results {
		matches[i] = vectorstore.SearchMatch{
			DocumentID: r.id,
			Score:      r.score,
			Content:    r.point.metadata["content"],
			Metadata:   copyMetadata(r.point.metadata),
		}
	}

	return matches, nil
}

func (b *Backend) UpsertPoints(_ context.Context, collectionName string, points []vectorstore.VectorPoint) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	coll, ok := b.collections[collectionName]
	if !ok {
		return fmt.Errorf("collection %q not found", collectionName)
	}

	for _, p := range points {
		coll.points[p.ID] = &storedPoint{
			vector:   p.Vector,
			metadata: copyMetadata(p.Metadata),
		}
	}
	return nil
}

func (b *Backend) DeletePointsByFile(_ context.Context, collectionName string, fileID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	coll, ok := b.collections[collectionName]
	if !ok {
		return nil // No collection means no points to delete.
	}

	for id, pt := range coll.points {
		if pt.metadata["file_id"] == fileID {
			delete(coll.points, id)
		}
	}
	return nil
}

// cosineSimilarity computes the cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

func copyMetadata(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
