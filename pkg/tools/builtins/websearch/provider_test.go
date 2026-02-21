package websearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/tools"
)

// newMockSearXNG starts an httptest.Server that mimics SearXNG responses.
// The handler function lets each test control the response.
func newMockSearXNG(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// searxngHandler returns a handler that serves a fixed set of SearXNG results.
func searxngHandler(results []searxngResult) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := searxngResponse{Results: results}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func TestWebSearch_Execute(t *testing.T) {
	server := newMockSearXNG(searxngHandler([]searxngResult{
		{Title: "Go Programming", URL: "https://go.dev", Content: "The Go programming language"},
		{Title: "Go Tutorial", URL: "https://go.dev/tour", Content: "A tour of Go"},
		{Title: "Go Docs", URL: "https://go.dev/doc", Content: "Documentation for Go"},
	}))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend":     "searxng",
		"url":         server.URL,
		"max_results": 5,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	result, err := provider.Execute(context.Background(), tools.ToolCall{
		ID:        "call_1",
		Name:      "web_search",
		Arguments: `{"query":"golang"}`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Output)
	}
	if result.CallID != "call_1" {
		t.Errorf("CallID = %q, want %q", result.CallID, "call_1")
	}

	// Verify formatted output contains all 3 results.
	if !strings.Contains(result.Output, `Search results for "golang"`) {
		t.Errorf("output missing header, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "1. Go Programming") {
		t.Errorf("output missing result 1, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "https://go.dev") {
		t.Errorf("output missing URL, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "3. Go Docs") {
		t.Errorf("output missing result 3, got: %s", result.Output)
	}
}

func TestWebSearch_EmptyQuery(t *testing.T) {
	server := newMockSearXNG(searxngHandler(nil))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend": "searxng",
		"url":     server.URL,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	result, err := provider.Execute(context.Background(), tools.ToolCall{
		ID:        "call_empty",
		Name:      "web_search",
		Arguments: `{"query":""}`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError = true for empty query")
	}
	if !strings.Contains(result.Output, "must not be empty") {
		t.Errorf("expected empty query error message, got: %s", result.Output)
	}
}

func TestWebSearch_BackendFailure(t *testing.T) {
	server := newMockSearXNG(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend": "searxng",
		"url":     server.URL,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	result, err := provider.Execute(context.Background(), tools.ToolCall{
		ID:        "call_fail",
		Name:      "web_search",
		Arguments: `{"query":"test"}`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError = true for backend failure")
	}
	if !strings.Contains(result.Output, "search failed") {
		t.Errorf("expected search failure message, got: %s", result.Output)
	}
}

func TestWebSearch_NoResults(t *testing.T) {
	server := newMockSearXNG(searxngHandler([]searxngResult{}))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend": "searxng",
		"url":     server.URL,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	result, err := provider.Execute(context.Background(), tools.ToolCall{
		ID:        "call_none",
		Name:      "web_search",
		Arguments: `{"query":"xyznonexistent"}`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected IsError = false for empty results, got error: %s", result.Output)
	}
	if !strings.Contains(result.Output, "No results found") {
		t.Errorf("expected no results message, got: %s", result.Output)
	}
}

func TestWebSearch_Tools(t *testing.T) {
	server := newMockSearXNG(searxngHandler(nil))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend": "searxng",
		"url":     server.URL,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	defs := provider.Tools()
	if len(defs) != 1 {
		t.Fatalf("Tools() returned %d definitions, want 1", len(defs))
	}

	td := defs[0]
	if td.Type != "function" {
		t.Errorf("Type = %q, want %q", td.Type, "function")
	}
	if td.Name != "web_search" {
		t.Errorf("Name = %q, want %q", td.Name, "web_search")
	}
	if td.Description == "" {
		t.Error("Description should not be empty")
	}
	if len(td.Parameters) == 0 {
		t.Error("Parameters should not be empty")
	}

	// Verify parameters schema has the expected structure.
	var params map[string]interface{}
	if err := json.Unmarshal(td.Parameters, &params); err != nil {
		t.Fatalf("failed to parse parameters JSON: %v", err)
	}
	if params["type"] != "object" {
		t.Errorf("parameters type = %v, want 'object'", params["type"])
	}
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("parameters.properties is not an object")
	}
	if _, ok := props["query"]; !ok {
		t.Error("parameters.properties missing 'query'")
	}
}

func TestWebSearch_CanExecute(t *testing.T) {
	server := newMockSearXNG(searxngHandler(nil))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend": "searxng",
		"url":     server.URL,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if !provider.CanExecute("web_search") {
		t.Error("expected CanExecute('web_search') = true")
	}
	if provider.CanExecute("other_tool") {
		t.Error("expected CanExecute('other_tool') = false")
	}
	if provider.CanExecute("") {
		t.Error("expected CanExecute('') = false")
	}
}

func TestWebSearch_Name(t *testing.T) {
	server := newMockSearXNG(searxngHandler(nil))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend": "searxng",
		"url":     server.URL,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if name := provider.Name(); name != "web_search" {
		t.Errorf("Name() = %q, want %q", name, "web_search")
	}
}

func TestWebSearch_MaxResults(t *testing.T) {
	// Backend returns 5 results but maxResults is 2.
	server := newMockSearXNG(searxngHandler([]searxngResult{
		{Title: "Result 1", URL: "https://example.com/1", Content: "First result"},
		{Title: "Result 2", URL: "https://example.com/2", Content: "Second result"},
		{Title: "Result 3", URL: "https://example.com/3", Content: "Third result"},
		{Title: "Result 4", URL: "https://example.com/4", Content: "Fourth result"},
		{Title: "Result 5", URL: "https://example.com/5", Content: "Fifth result"},
	}))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend":     "searxng",
		"url":         server.URL,
		"max_results": 2,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	result, err := provider.Execute(context.Background(), tools.ToolCall{
		ID:        "call_max",
		Name:      "web_search",
		Arguments: `{"query":"test"}`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Output)
	}

	// Should contain results 1 and 2 but not 3, 4, or 5.
	if !strings.Contains(result.Output, "1. Result 1") {
		t.Errorf("output missing result 1, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "2. Result 2") {
		t.Errorf("output missing result 2, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "3. Result 3") {
		t.Errorf("output should not contain result 3 (max_results=2), got: %s", result.Output)
	}
}

func TestWebSearch_NewMissingURL(t *testing.T) {
	_, err := New(map[string]interface{}{
		"backend": "searxng",
	})
	if err == nil {
		t.Fatal("expected error for missing url")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("expected error about url, got: %v", err)
	}
}

func TestWebSearch_NewUnknownBackend(t *testing.T) {
	_, err := New(map[string]interface{}{
		"backend": "google",
	})
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
	if !strings.Contains(err.Error(), "unknown backend") {
		t.Errorf("expected error about unknown backend, got: %v", err)
	}
}

func TestWebSearch_HTMLStripping(t *testing.T) {
	server := newMockSearXNG(searxngHandler([]searxngResult{
		{
			Title:   "<b>Bold Title</b>",
			URL:     "https://example.com",
			Content: "Some <em>emphasized</em> text with <a href='url'>link</a>",
		},
	}))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend": "searxng",
		"url":     server.URL,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	result, err := provider.Execute(context.Background(), tools.ToolCall{
		ID:        "call_html",
		Name:      "web_search",
		Arguments: `{"query":"html test"}`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Execute() returned error result: %s", result.Output)
	}

	if strings.Contains(result.Output, "<b>") || strings.Contains(result.Output, "</b>") {
		t.Errorf("output should not contain HTML tags, got: %s", result.Output)
	}
	if strings.Contains(result.Output, "<em>") || strings.Contains(result.Output, "<a") {
		t.Errorf("output should not contain HTML tags, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "Bold Title") {
		t.Errorf("output missing stripped title text, got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "emphasized") {
		t.Errorf("output missing stripped content text, got: %s", result.Output)
	}
}

func TestWebSearch_InvalidArguments(t *testing.T) {
	server := newMockSearXNG(searxngHandler(nil))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend": "searxng",
		"url":     server.URL,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	result, err := provider.Execute(context.Background(), tools.ToolCall{
		ID:        "call_invalid",
		Name:      "web_search",
		Arguments: `not valid json`,
	})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError = true for invalid arguments")
	}
	if !strings.Contains(result.Output, "invalid arguments") {
		t.Errorf("expected invalid arguments error, got: %s", result.Output)
	}
}

func TestWebSearch_RoutesReturnsNil(t *testing.T) {
	server := newMockSearXNG(searxngHandler(nil))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend": "searxng",
		"url":     server.URL,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if routes := provider.Routes(); routes != nil {
		t.Errorf("Routes() = %v, want nil", routes)
	}
}

func TestWebSearch_Collectors(t *testing.T) {
	server := newMockSearXNG(searxngHandler(nil))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend": "searxng",
		"url":     server.URL,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	collectors := provider.Collectors()
	if len(collectors) != 2 {
		t.Errorf("Collectors() returned %d collectors, want 2", len(collectors))
	}
}

func TestWebSearch_CloseIsNoop(t *testing.T) {
	server := newMockSearXNG(searxngHandler(nil))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"backend": "searxng",
		"url":     server.URL,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if err := provider.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestWebSearch_DefaultSettings(t *testing.T) {
	server := newMockSearXNG(searxngHandler(nil))
	defer server.Close()

	// Only provide required 'url', rely on defaults for everything else.
	provider, err := New(map[string]interface{}{
		"url": server.URL,
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if provider.backend != "searxng" {
		t.Errorf("default backend = %q, want %q", provider.backend, "searxng")
	}
	if provider.maxResults != 5 {
		t.Errorf("default maxResults = %d, want 5", provider.maxResults)
	}
}
