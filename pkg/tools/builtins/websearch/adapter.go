package websearch

import "context"

// SearchResult holds a single search result.
type SearchResult struct {
	Title   string
	URL     string
	Snippet string
}

// SearchAdapter is the interface for pluggable search backends.
type SearchAdapter interface {
	Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error)
}
