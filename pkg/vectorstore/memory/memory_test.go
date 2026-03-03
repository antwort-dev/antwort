package memory

import (
	"context"
	"testing"

	"github.com/rhuss/antwort/pkg/vectorstore"
)

func TestBackend_CreateAndSearch(t *testing.T) {
	b := New()
	ctx := context.Background()

	if err := b.CreateCollection(ctx, "test", 3); err != nil {
		t.Fatal(err)
	}

	// Upsert some points.
	points := []vectorstore.VectorPoint{
		{ID: "p1", Vector: []float32{1, 0, 0}, Metadata: map[string]string{"content": "first", "file_id": "f1"}},
		{ID: "p2", Vector: []float32{0, 1, 0}, Metadata: map[string]string{"content": "second", "file_id": "f2"}},
		{ID: "p3", Vector: []float32{0.9, 0.1, 0}, Metadata: map[string]string{"content": "similar to first", "file_id": "f1"}},
	}
	if err := b.UpsertPoints(ctx, "test", points); err != nil {
		t.Fatal(err)
	}

	// Search for vector close to [1,0,0] should return p1 first.
	results, err := b.Search(ctx, "test", []float32{1, 0, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].DocumentID != "p1" {
		t.Errorf("expected p1 first (exact match), got %s", results[0].DocumentID)
	}
	if results[0].Score < 0.99 {
		t.Errorf("expected score ~1.0 for exact match, got %f", results[0].Score)
	}
}

func TestBackend_DeletePointsByFile(t *testing.T) {
	b := New()
	ctx := context.Background()

	b.CreateCollection(ctx, "test", 3)
	b.UpsertPoints(ctx, "test", []vectorstore.VectorPoint{
		{ID: "p1", Vector: []float32{1, 0, 0}, Metadata: map[string]string{"file_id": "f1"}},
		{ID: "p2", Vector: []float32{0, 1, 0}, Metadata: map[string]string{"file_id": "f2"}},
		{ID: "p3", Vector: []float32{0, 0, 1}, Metadata: map[string]string{"file_id": "f1"}},
	})

	// Delete points for f1.
	if err := b.DeletePointsByFile(ctx, "test", "f1"); err != nil {
		t.Fatal(err)
	}

	// Only p2 should remain.
	results, err := b.Search(ctx, "test", []float32{0, 1, 0}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result after delete, got %d", len(results))
	}
	if results[0].DocumentID != "p2" {
		t.Errorf("expected p2, got %s", results[0].DocumentID)
	}
}

func TestBackend_DeleteCollection(t *testing.T) {
	b := New()
	ctx := context.Background()

	b.CreateCollection(ctx, "test", 3)
	b.DeleteCollection(ctx, "test")

	_, err := b.Search(ctx, "test", []float32{1, 0, 0}, 1)
	if err == nil {
		t.Error("expected error searching deleted collection")
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []float32
		want float32
	}{
		{"identical", []float32{1, 0, 0}, []float32{1, 0, 0}, 1.0},
		{"orthogonal", []float32{1, 0, 0}, []float32{0, 1, 0}, 0.0},
		{"opposite", []float32{1, 0, 0}, []float32{-1, 0, 0}, -1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if got < tt.want-0.01 || got > tt.want+0.01 {
				t.Errorf("cosineSimilarity(%v, %v) = %f, want %f", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
