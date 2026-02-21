package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/tools"
	"github.com/rhuss/antwort/pkg/tools/registry"
)

const toolName = "web_search"

// toolParametersJSON is the JSON Schema for the web_search tool parameters.
var toolParametersJSON = json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Search query"}},"required":["query"]}`)

// WebSearchProvider implements registry.FunctionProvider for web search.
type WebSearchProvider struct {
	adapter    SearchAdapter
	maxResults int
	backend    string
	queries    *prometheus.CounterVec
	results    *prometheus.HistogramVec
}

// Compile-time check that WebSearchProvider implements FunctionProvider.
var _ registry.FunctionProvider = (*WebSearchProvider)(nil)

// New creates a WebSearchProvider from a settings map.
//
// Supported settings:
//   - "backend" (string, default "searxng"): search backend to use
//   - "url" (string, required for searxng): base URL of the SearXNG instance
//   - "max_results" (int/float64, default 5): maximum number of results to return
func New(settings map[string]interface{}) (*WebSearchProvider, error) {
	backend := "searxng"
	if v, ok := settings["backend"]; ok {
		if s, ok := v.(string); ok && s != "" {
			backend = s
		}
	}

	maxResults := 5
	if v, ok := settings["max_results"]; ok {
		switch n := v.(type) {
		case int:
			maxResults = n
		case float64:
			maxResults = int(n)
		}
	}

	var adapter SearchAdapter
	switch backend {
	case "searxng":
		rawURL, ok := settings["url"]
		if !ok {
			return nil, fmt.Errorf("web_search: 'url' is required for searxng backend")
		}
		urlStr, ok := rawURL.(string)
		if !ok || urlStr == "" {
			return nil, fmt.Errorf("web_search: 'url' must be a non-empty string")
		}
		adapter = NewSearXNG(urlStr)
	default:
		return nil, fmt.Errorf("web_search: unknown backend %q", backend)
	}

	queries := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_websearch_queries_total",
			Help: "Total web search queries",
		},
		[]string{"backend", "status"},
	)

	results := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "antwort_websearch_results_returned",
			Help:    "Number of web search results returned",
			Buckets: []float64{0, 1, 2, 3, 5, 10, 20},
		},
		[]string{"backend"},
	)

	return &WebSearchProvider{
		adapter:    adapter,
		maxResults: maxResults,
		backend:    backend,
		queries:    queries,
		results:    results,
	}, nil
}

// Name returns the provider identifier.
func (p *WebSearchProvider) Name() string {
	return toolName
}

// Tools returns the tool definitions contributed by this provider.
func (p *WebSearchProvider) Tools() []api.ToolDefinition {
	return []api.ToolDefinition{
		{
			Type:        "function",
			Name:        toolName,
			Description: "Search the web for current information",
			Parameters:  toolParametersJSON,
		},
	}
}

// CanExecute reports whether this provider handles the named tool.
func (p *WebSearchProvider) CanExecute(name string) bool {
	return name == toolName
}

// Execute runs the web_search tool call and returns the result.
func (p *WebSearchProvider) Execute(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	// Parse arguments to extract the query.
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		p.queries.WithLabelValues(p.backend, "error").Inc()
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  fmt.Sprintf("invalid arguments: %v", err),
			IsError: true,
		}, nil
	}

	if strings.TrimSpace(args.Query) == "" {
		p.queries.WithLabelValues(p.backend, "error").Inc()
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  "query must not be empty",
			IsError: true,
		}, nil
	}

	// Execute the search.
	results, err := p.adapter.Search(ctx, args.Query, p.maxResults)
	if err != nil {
		p.queries.WithLabelValues(p.backend, "error").Inc()
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  fmt.Sprintf("search failed: %v", err),
			IsError: true,
		}, nil
	}

	// Record metrics.
	p.queries.WithLabelValues(p.backend, "success").Inc()
	p.results.WithLabelValues(p.backend).Observe(float64(len(results)))

	// Format results as structured text.
	output := formatResults(args.Query, results)

	return &tools.ToolResult{
		CallID: call.ID,
		Output: output,
	}, nil
}

// Routes returns nil (no management API endpoints).
func (p *WebSearchProvider) Routes() []registry.Route {
	return nil
}

// Collectors returns the custom Prometheus metrics for this provider.
func (p *WebSearchProvider) Collectors() []prometheus.Collector {
	return []prometheus.Collector{p.queries, p.results}
}

// Close is a no-op for this provider.
func (p *WebSearchProvider) Close() error {
	return nil
}

// formatResults builds a human-readable text block from search results.
func formatResults(query string, results []SearchResult) string {
	if len(results) == 0 {
		return fmt.Sprintf("No results found for %q.", query)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Search results for %q:\n", query)

	for i, r := range results {
		fmt.Fprintf(&b, "\n%d. %s\n   URL: %s\n   %s\n", i+1, r.Title, r.URL, r.Snippet)
	}

	return b.String()
}
