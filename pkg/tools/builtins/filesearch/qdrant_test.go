package filesearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQdrant_CreateCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/collections/test-collection" {
			t.Errorf("expected path /collections/test-collection, got %s", r.URL.Path)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		vectors, ok := body["vectors"].(map[string]interface{})
		if !ok {
			t.Fatal("expected 'vectors' in request body")
		}
		if size, ok := vectors["size"].(float64); !ok || int(size) != 384 {
			t.Errorf("expected vectors.size = 384, got %v", vectors["size"])
		}
		if dist, ok := vectors["distance"].(string); !ok || dist != "Cosine" {
			t.Errorf("expected vectors.distance = Cosine, got %v", vectors["distance"])
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":true,"status":"ok"}`))
	}))
	defer server.Close()

	q := NewQdrant(server.URL)
	err := q.CreateCollection(context.Background(), "test-collection", 384)
	if err != nil {
		t.Fatalf("CreateCollection() returned error: %v", err)
	}
}

func TestQdrant_CreateCollectionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"status":{"error":"Collection already exists"}}`))
	}))
	defer server.Close()

	q := NewQdrant(server.URL)
	err := q.CreateCollection(context.Background(), "existing", 384)
	if err == nil {
		t.Fatal("expected error for conflicting collection")
	}
}

func TestQdrant_DeleteCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/collections/test-collection" {
			t.Errorf("expected path /collections/test-collection, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":true,"status":"ok"}`))
	}))
	defer server.Close()

	q := NewQdrant(server.URL)
	err := q.DeleteCollection(context.Background(), "test-collection")
	if err != nil {
		t.Fatalf("DeleteCollection() returned error: %v", err)
	}
}

func TestQdrant_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/collections/docs/points/search" {
			t.Errorf("expected path /collections/docs/points/search, got %s", r.URL.Path)
		}

		var req qdrantSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode search request: %v", err)
		}
		if !req.WithPayload {
			t.Error("expected with_payload = true")
		}
		if req.Limit != 5 {
			t.Errorf("expected limit = 5, got %d", req.Limit)
		}

		resp := qdrantSearchResponse{
			Result: []qdrantSearchResult{
				{
					ID:    "doc-1",
					Score: 0.95,
					Payload: map[string]interface{}{
						"content":  "Go is a statically typed language.",
						"filename": "intro.md",
					},
				},
				{
					ID:    "doc-2",
					Score: 0.82,
					Payload: map[string]interface{}{
						"content":  "Go supports concurrency with goroutines.",
						"filename": "concurrency.md",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	q := NewQdrant(server.URL)
	matches, err := q.Search(context.Background(), "docs", []float32{0.1, 0.2, 0.3}, 5)
	if err != nil {
		t.Fatalf("Search() returned error: %v", err)
	}

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	if matches[0].DocumentID != "doc-1" {
		t.Errorf("match[0].DocumentID = %q, want %q", matches[0].DocumentID, "doc-1")
	}
	if matches[0].Score != 0.95 {
		t.Errorf("match[0].Score = %f, want 0.95", matches[0].Score)
	}
	if matches[0].Content != "Go is a statically typed language." {
		t.Errorf("match[0].Content = %q, want %q", matches[0].Content, "Go is a statically typed language.")
	}
	if matches[0].Metadata["filename"] != "intro.md" {
		t.Errorf("match[0].Metadata[filename] = %q, want %q", matches[0].Metadata["filename"], "intro.md")
	}

	if matches[1].DocumentID != "doc-2" {
		t.Errorf("match[1].DocumentID = %q, want %q", matches[1].DocumentID, "doc-2")
	}
	if matches[1].Score != 0.82 {
		t.Errorf("match[1].Score = %f, want 0.82", matches[1].Score)
	}
}

func TestQdrant_SearchError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"status":{"error":"Collection not found"}}`))
	}))
	defer server.Close()

	q := NewQdrant(server.URL)
	_, err := q.Search(context.Background(), "nonexistent", []float32{0.1}, 5)
	if err == nil {
		t.Fatal("expected error for missing collection")
	}
}

func TestQdrant_TrailingSlashInURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/collections/test" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":true}`))
	}))
	defer server.Close()

	// URL with trailing slash should be handled correctly.
	q := NewQdrant(server.URL + "/")
	err := q.DeleteCollection(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
