package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/tools"
	"github.com/rhuss/antwort/pkg/tools/builtins/websearch"
)

// TestSearXNGWebSearch tests the SearXNG search adapter with a mock backend.
func TestSearXNGWebSearch(t *testing.T) {
	// Start a mock SearXNG server.
	mockSearXNG := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if query == "" {
			http.Error(w, "missing query", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"title":   "Test Result 1",
					"url":     "https://example.com/1",
					"content": "This is the first test result snippet.",
				},
				{
					"title":   "Test Result 2",
					"url":     "https://example.com/2",
					"content": "This is the second test result snippet.",
				},
			},
		})
	}))
	defer mockSearXNG.Close()

	provider, err := websearch.New(map[string]interface{}{
		"backend":     "searxng",
		"url":         mockSearXNG.URL,
		"max_results": 5,
	})
	if err != nil {
		t.Fatalf("creating SearXNG provider: %v", err)
	}

	if !provider.CanExecute("web_search") {
		t.Error("expected CanExecute('web_search') to return true")
	}

	result, err := provider.Execute(context.Background(), tools.ToolCall{
		ID:        "call_searxng_1",
		Name:      "web_search",
		Arguments: `{"query":"test search"}`,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Errorf("expected no error, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Test Result 1") {
		t.Errorf("expected output to contain 'Test Result 1', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "https://example.com/1") {
		t.Errorf("expected output to contain URL, got: %s", result.Output)
	}
}

// TestBraveWebSearch tests the Brave search adapter with a mock backend.
func TestBraveWebSearch(t *testing.T) {
	// Start a mock Brave Search API server.
	mockBrave := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the API key header.
		token := r.Header.Get("X-Subscription-Token")
		if token != "test-brave-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"web": map[string]any{
				"results": []map[string]any{
					{
						"title":       "Brave Result",
						"url":         "https://brave.example.com/1",
						"description": "A brave search result.",
					},
				},
			},
		})
	}))
	defer mockBrave.Close()

	// The Brave adapter uses a hardcoded URL (https://api.search.brave.com/...).
	// We test the adapter directly instead.
	adapter := websearch.NewBrave("test-brave-key")
	adapter.HTTPClient = mockBrave.Client()

	// Override the HTTP client transport to redirect requests to our mock.
	adapter.HTTPClient = &http.Client{
		Transport: &rewriteTransport{
			base:    http.DefaultTransport,
			rewrite: mockBrave.URL,
		},
	}

	results, err := adapter.Search(context.Background(), "test query", 5)
	if err != nil {
		t.Fatalf("Brave Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Brave Result" {
		t.Errorf("title = %q, want %q", results[0].Title, "Brave Result")
	}
	if results[0].URL != "https://brave.example.com/1" {
		t.Errorf("url = %q, want %q", results[0].URL, "https://brave.example.com/1")
	}
}

// TestTavilyWebSearch tests the Tavily search adapter with a mock backend.
func TestTavilyWebSearch(t *testing.T) {
	// Start a mock Tavily Search API server.
	mockTavily := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-tavily-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req struct {
			Query      string `json:"query"`
			MaxResults int    `json:"max_results"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"title":   "Tavily Result",
					"url":     "https://tavily.example.com/1",
					"content": "A tavily search result.",
					"score":   0.95,
				},
			},
		})
	}))
	defer mockTavily.Close()

	adapter := websearch.NewTavily("test-tavily-key")
	adapter.HTTPClient = &http.Client{
		Transport: &rewriteTransport{
			base:    http.DefaultTransport,
			rewrite: mockTavily.URL,
		},
	}

	results, err := adapter.Search(context.Background(), "test query", 5)
	if err != nil {
		t.Fatalf("Tavily Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Tavily Result" {
		t.Errorf("title = %q, want %q", results[0].Title, "Tavily Result")
	}
}

// TestWebSearchProviderEmptyQuery verifies that an empty query returns an error result.
func TestWebSearchProviderEmptyQuery(t *testing.T) {
	mockSearXNG := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer mockSearXNG.Close()

	provider, err := websearch.New(map[string]interface{}{
		"backend": "searxng",
		"url":     mockSearXNG.URL,
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	result, err := provider.Execute(context.Background(), tools.ToolCall{
		ID:        "call_empty",
		Name:      "web_search",
		Arguments: `{"query":""}`,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for empty query")
	}
}

// TestWebSearchProviderTools verifies the tool definitions are correct.
func TestWebSearchProviderTools(t *testing.T) {
	mockSearXNG := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer mockSearXNG.Close()

	provider, err := websearch.New(map[string]interface{}{
		"backend": "searxng",
		"url":     mockSearXNG.URL,
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	toolDefs := provider.Tools()
	if len(toolDefs) != 1 {
		t.Fatalf("expected 1 tool definition, got %d", len(toolDefs))
	}
	if toolDefs[0].Name != "web_search" {
		t.Errorf("tool name = %q, want %q", toolDefs[0].Name, "web_search")
	}
	if toolDefs[0].Type != "function" {
		t.Errorf("tool type = %q, want %q", toolDefs[0].Type, "function")
	}
}

// rewriteTransport redirects all requests to the mock server URL.
type rewriteTransport struct {
	base    http.RoundTripper
	rewrite string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the scheme+host with the mock server URL.
	req.URL.Scheme = "http"
	// Parse mock URL to extract host.
	mockURL := strings.TrimPrefix(t.rewrite, "http://")
	req.URL.Host = mockURL
	return t.base.RoundTrip(req)
}
