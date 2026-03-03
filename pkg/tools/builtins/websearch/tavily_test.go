package websearch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTavilyAdapter_Search(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      interface{}
		wantCount int
		wantErr   string
	}{
		{
			name:   "successful search",
			status: 200,
			body: tavilyResponse{
				Results: []tavilyResult{
					{Title: "Go Programming", URL: "https://go.dev", Content: "The Go programming language", Score: 0.95},
					{Title: "Go Tour", URL: "https://go.dev/tour", Content: "A tour of Go", Score: 0.85},
				},
			},
			wantCount: 2,
		},
		{
			name:      "empty results",
			status:    200,
			body:      tavilyResponse{Results: []tavilyResult{}},
			wantCount: 0,
		},
		{
			name:    "auth error 401",
			status:  401,
			body:    map[string]string{"error": "unauthorized"},
			wantErr: "API key is invalid",
		},
		{
			name:    "rate limit 429",
			status:  429,
			body:    map[string]string{"error": "rate limited"},
			wantErr: "rate limit exceeded",
		},
		{
			name:    "server error 500",
			status:  500,
			body:    map[string]string{"error": "internal"},
			wantErr: "status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify method.
				if r.Method != http.MethodPost {
					t.Errorf("method: got %s, want POST", r.Method)
				}
				// Verify auth header.
				auth := r.Header.Get("Authorization")
				if auth != "Bearer test-key" {
					t.Errorf("auth header: got %q, want 'Bearer test-key'", auth)
				}
				// Verify content type.
				if r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing application/json content type")
				}
				w.WriteHeader(tt.status)
				json.NewEncoder(w).Encode(tt.body)
			}))
			defer srv.Close()

			adapter := NewTavily("test-key")
			adapter.HTTPClient = &http.Client{
				Transport: &rewriteTransport{target: srv.URL},
			}

			results, err := adapter.Search(context.Background(), "golang", 5)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(results) != tt.wantCount {
				t.Errorf("got %d results, want %d", len(results), tt.wantCount)
			}
		})
	}
}

func TestTavilyAdapter_RequestBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req tavilyRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("invalid request body: %v", err)
		}
		if req.Query != "test query" {
			t.Errorf("query: got %q, want 'test query'", req.Query)
		}
		if req.MaxResults != 3 {
			t.Errorf("max_results: got %d, want 3", req.MaxResults)
		}
		json.NewEncoder(w).Encode(tavilyResponse{Results: []tavilyResult{}})
	}))
	defer srv.Close()

	adapter := NewTavily("key")
	adapter.HTTPClient = &http.Client{
		Transport: &rewriteTransport{target: srv.URL},
	}

	_, err := adapter.Search(context.Background(), "test query", 3)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTavilyAdapter_FieldMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(tavilyResponse{
			Results: []tavilyResult{
				{Title: "Test Title", URL: "https://test.com", Content: "  Test content  ", Score: 0.9},
			},
		})
	}))
	defer srv.Close()

	adapter := NewTavily("key")
	adapter.HTTPClient = &http.Client{
		Transport: &rewriteTransport{target: srv.URL},
	}

	results, err := adapter.Search(context.Background(), "test", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
	r := results[0]
	if r.Title != "Test Title" {
		t.Errorf("title: got %q", r.Title)
	}
	if r.URL != "https://test.com" {
		t.Errorf("url: got %q", r.URL)
	}
	if r.Snippet != "Test content" {
		t.Errorf("snippet: got %q (should be trimmed)", r.Snippet)
	}
}
