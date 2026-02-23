package filesearch

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/storage"
	"github.com/rhuss/antwort/pkg/tools"
	"github.com/rhuss/antwort/pkg/tools/registry"
)

const toolName = "file_search"

// toolParametersJSON is the JSON Schema for the file_search tool parameters.
var toolParametersJSON = json.RawMessage(`{
	"type": "object",
	"properties": {
		"query": {
			"type": "string",
			"description": "The search query to find relevant documents"
		},
		"vector_store_ids": {
			"type": "array",
			"items": {"type": "string"},
			"description": "IDs of vector stores to search. If omitted, all stores for the tenant are searched."
		}
	},
	"required": ["query"]
}`)

// FileSearchProvider implements registry.FunctionProvider for vector-based document search.
// All compute (embedding, vector search) happens externally; this provider orchestrates
// the query flow and manages vector store metadata.
type FileSearchProvider struct {
	backend    VectorStoreBackend
	embedding  EmbeddingClient
	metadata   *MetadataStore
	maxResults int

	// Prometheus metrics.
	searchLatency *prometheus.HistogramVec
	embedLatency  *prometheus.HistogramVec
	searchCount   *prometheus.CounterVec
}

// Compile-time check that FileSearchProvider implements FunctionProvider.
var _ registry.FunctionProvider = (*FileSearchProvider)(nil)

// New creates a FileSearchProvider from a settings map.
//
// Supported settings:
//   - "backend" (string, default "qdrant"): vector store backend to use
//   - "backend_url" (string, required for qdrant): URL of the Qdrant instance
//   - "embedding_url" (string, required): URL of the embedding service
//   - "embedding_model" (string, default "text-embedding-ada-002"): embedding model name
//   - "max_results" (int/float64, default 10): maximum search results per store
func New(settings map[string]interface{}) (*FileSearchProvider, error) {
	backendType := "qdrant"
	if v, ok := settings["backend"]; ok {
		if s, ok := v.(string); ok && s != "" {
			backendType = s
		}
	}

	maxResults := 10
	if v, ok := settings["max_results"]; ok {
		switch n := v.(type) {
		case int:
			maxResults = n
		case float64:
			maxResults = int(n)
		}
	}

	// Create vector store backend.
	var backend VectorStoreBackend
	switch backendType {
	case "qdrant":
		rawURL, ok := settings["backend_url"]
		if !ok {
			return nil, fmt.Errorf("file_search: 'backend_url' is required for qdrant backend")
		}
		urlStr, ok := rawURL.(string)
		if !ok || urlStr == "" {
			return nil, fmt.Errorf("file_search: 'backend_url' must be a non-empty string")
		}
		backend = NewQdrant(urlStr)
	default:
		return nil, fmt.Errorf("file_search: unknown backend %q", backendType)
	}

	// Create embedding client.
	rawEmbURL, ok := settings["embedding_url"]
	if !ok {
		return nil, fmt.Errorf("file_search: 'embedding_url' is required")
	}
	embURL, ok := rawEmbURL.(string)
	if !ok || embURL == "" {
		return nil, fmt.Errorf("file_search: 'embedding_url' must be a non-empty string")
	}

	embModel := "text-embedding-ada-002"
	if v, ok := settings["embedding_model"]; ok {
		if s, ok := v.(string); ok && s != "" {
			embModel = s
		}
	}

	embedding := NewOpenAIEmbeddingClient(embURL, embModel)

	searchLatency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "antwort_filesearch_search_duration_seconds",
			Help:    "File search vector query duration",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		},
		[]string{"status"},
	)

	embedLatency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "antwort_filesearch_embed_duration_seconds",
			Help:    "File search embedding duration",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2},
		},
		[]string{"status"},
	)

	searchCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_filesearch_queries_total",
			Help: "Total file search queries",
		},
		[]string{"status"},
	)

	return &FileSearchProvider{
		backend:       backend,
		embedding:     embedding,
		metadata:      NewMetadataStore(),
		maxResults:    maxResults,
		searchLatency: searchLatency,
		embedLatency:  embedLatency,
		searchCount:   searchCount,
	}, nil
}

// newWithDeps creates a FileSearchProvider with injected dependencies (for testing).
func newWithDeps(backend VectorStoreBackend, embedding EmbeddingClient, maxResults int) *FileSearchProvider {
	return &FileSearchProvider{
		backend:    backend,
		embedding:  embedding,
		metadata:   NewMetadataStore(),
		maxResults: maxResults,
		searchLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "antwort_filesearch_search_duration_seconds",
				Help:    "File search vector query duration",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
			},
			[]string{"status"},
		),
		embedLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "antwort_filesearch_embed_duration_seconds",
				Help:    "File search embedding duration",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2},
			},
			[]string{"status"},
		),
		searchCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "antwort_filesearch_queries_total",
				Help: "Total file search queries",
			},
			[]string{"status"},
		),
	}
}

// Name returns the provider identifier.
func (p *FileSearchProvider) Name() string {
	return toolName
}

// Tools returns the tool definitions contributed by this provider.
func (p *FileSearchProvider) Tools() []api.ToolDefinition {
	return []api.ToolDefinition{
		{
			Type:        "function",
			Name:        toolName,
			Description: "Search documents in vector stores for relevant content",
			Parameters:  toolParametersJSON,
		},
	}
}

// CanExecute reports whether this provider handles the named tool.
func (p *FileSearchProvider) CanExecute(name string) bool {
	return name == toolName
}

// fileSearchArgs represents the parsed arguments for the file_search tool.
type fileSearchArgs struct {
	Query          string   `json:"query"`
	VectorStoreIDs []string `json:"vector_store_ids"`
}

// Execute runs the file_search tool call and returns the result.
func (p *FileSearchProvider) Execute(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	// Parse arguments.
	var args fileSearchArgs
	if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
		p.searchCount.WithLabelValues("error").Inc()
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  fmt.Sprintf("invalid arguments: %v", err),
			IsError: true,
		}, nil
	}

	if strings.TrimSpace(args.Query) == "" {
		p.searchCount.WithLabelValues("error").Inc()
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  "query must not be empty",
			IsError: true,
		}, nil
	}

	// Determine which stores to search.
	var stores []*VectorStore
	if len(args.VectorStoreIDs) > 0 {
		for _, id := range args.VectorStoreIDs {
			vs, err := p.metadata.Get(id)
			if err != nil {
				continue // Skip unknown stores.
			}
			// Tenant isolation.
			tenantID := storage.GetTenant(ctx)
			if tenantID != "" && vs.TenantID != tenantID {
				continue
			}
			stores = append(stores, vs)
		}
	} else {
		// Search all stores for the tenant.
		tenantID := storage.GetTenant(ctx)
		stores = p.metadata.List(tenantID)
	}

	if len(stores) == 0 {
		p.searchCount.WithLabelValues("success").Inc()
		return &tools.ToolResult{
			CallID: call.ID,
			Output: "No vector stores available to search.",
		}, nil
	}

	// Embed the query.
	vectors, err := p.embedding.Embed(ctx, []string{args.Query})
	if err != nil {
		p.searchCount.WithLabelValues("error").Inc()
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  fmt.Sprintf("embedding failed: %v", err),
			IsError: true,
		}, nil
	}

	if len(vectors) == 0 || len(vectors[0]) == 0 {
		p.searchCount.WithLabelValues("error").Inc()
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  "embedding returned no vectors",
			IsError: true,
		}, nil
	}

	queryVector := vectors[0]

	// Search each store's collection and collect results.
	var allMatches []SearchMatch
	for _, vs := range stores {
		matches, err := p.backend.Search(ctx, vs.CollectionName, queryVector, p.maxResults)
		if err != nil {
			// Log but continue with other stores.
			continue
		}
		allMatches = append(allMatches, matches...)
	}

	// Sort by score descending and limit to maxResults.
	sort.Slice(allMatches, func(i, j int) bool {
		return allMatches[i].Score > allMatches[j].Score
	})
	if len(allMatches) > p.maxResults {
		allMatches = allMatches[:p.maxResults]
	}

	// Format results as text.
	output := formatSearchResults(args.Query, allMatches)

	p.searchCount.WithLabelValues("success").Inc()
	return &tools.ToolResult{
		CallID: call.ID,
		Output: output,
	}, nil
}

// Routes returns the HTTP routes for the vector store management API.
func (p *FileSearchProvider) Routes() []registry.Route {
	return []registry.Route{
		{Method: "POST", Pattern: "/vector_stores", Handler: p.handleCreateStore},
		{Method: "GET", Pattern: "/vector_stores", Handler: p.handleListStores},
		{Method: "GET", Pattern: "/vector_stores/{store_id}", Handler: p.handleGetStore},
		{Method: "DELETE", Pattern: "/vector_stores/{store_id}", Handler: p.handleDeleteStore},
	}
}

// Collectors returns the custom Prometheus metrics for this provider.
func (p *FileSearchProvider) Collectors() []prometheus.Collector {
	return []prometheus.Collector{p.searchLatency, p.embedLatency, p.searchCount}
}

// Close is a no-op for this provider.
func (p *FileSearchProvider) Close() error {
	return nil
}

// formatSearchResults builds a human-readable text block from search matches.
func formatSearchResults(query string, matches []SearchMatch) string {
	if len(matches) == 0 {
		return fmt.Sprintf("No results found for %q.", query)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Search results for %q:\n", query)

	for i, m := range matches {
		fmt.Fprintf(&b, "\n%d. [Score: %.4f]", i+1, m.Score)
		if m.DocumentID != "" {
			fmt.Fprintf(&b, " (doc: %s)", m.DocumentID)
		}
		fmt.Fprintf(&b, "\n   %s\n", m.Content)
	}

	return b.String()
}
