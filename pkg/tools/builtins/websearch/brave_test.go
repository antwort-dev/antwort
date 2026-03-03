package websearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBraveAdapter_Search(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       interface{}
		wantCount  int
		wantErr    string
	}{
		{
			name:   "successful search",
			status: 200,
			body: braveResponse{
				Web: braveWebResults{
					Results: []braveResult{
						{Title: "Go Programming", URL: "https://go.dev", Description: "The Go programming language"},
						{Title: "Go Tour", URL: "https://go.dev/tour", Description: "A tour of Go"},
					},
				},
			},
			wantCount: 2,
		},
		{
			name:   "empty results",
			status: 200,
			body: braveResponse{
				Web: braveWebResults{Results: []braveResult{}},
			},
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
				// Verify auth header.
				if r.Header.Get("X-Subscription-Token") != "test-key" {
					t.Error("missing or wrong X-Subscription-Token header")
				}
				// Verify query parameter.
				if r.URL.Query().Get("q") == "" {
					t.Error("missing q parameter")
				}
				w.WriteHeader(tt.status)
				json.NewEncoder(w).Encode(tt.body)
			}))
			defer srv.Close()

			adapter := NewBrave("test-key")
			adapter.HTTPClient = srv.Client()

			// Override the URL by wrapping with a test transport.
			adapter.HTTPClient = &http.Client{
				Transport: &rewriteTransport{base: srv.Client().Transport, target: srv.URL},
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

func TestBraveAdapter_MaxResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := r.URL.Query().Get("count")
		if count != "2" {
			t.Errorf("count param: got %q, want 2", count)
		}
		json.NewEncoder(w).Encode(braveResponse{
			Web: braveWebResults{
				Results: []braveResult{
					{Title: "A", URL: "https://a.com", Description: "first"},
					{Title: "B", URL: "https://b.com", Description: "second"},
					{Title: "C", URL: "https://c.com", Description: "third"},
				},
			},
		})
	}))
	defer srv.Close()

	adapter := NewBrave("key")
	adapter.HTTPClient = &http.Client{
		Transport: &rewriteTransport{base: srv.Client().Transport, target: srv.URL},
	}

	results, err := adapter.Search(context.Background(), "test", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("got %d results, want 2 (maxResults limit)", len(results))
	}
}

func TestBraveAdapter_FieldMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(braveResponse{
			Web: braveWebResults{
				Results: []braveResult{
					{Title: "Test Title", URL: "https://test.com", Description: "  Test description  "},
				},
			},
		})
	}))
	defer srv.Close()

	adapter := NewBrave("key")
	adapter.HTTPClient = &http.Client{
		Transport: &rewriteTransport{base: srv.Client().Transport, target: srv.URL},
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
	if r.Snippet != "Test description" {
		t.Errorf("snippet: got %q (should be trimmed)", r.Snippet)
	}
}

// rewriteTransport redirects requests to a test server URL.
type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.target, "http://")
	if t.base != nil {
		return t.base.RoundTrip(req)
	}
	return http.DefaultTransport.RoundTrip(req)
}
