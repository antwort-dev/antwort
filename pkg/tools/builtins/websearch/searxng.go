package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// htmlTagRegex matches HTML tags for stripping from snippets.
var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

// SearXNGAdapter implements SearchAdapter using a SearXNG instance.
type SearXNGAdapter struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewSearXNG creates a SearXNG adapter with the given base URL.
func NewSearXNG(baseURL string) *SearXNGAdapter {
	return &SearXNGAdapter{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: http.DefaultClient,
	}
}

// searxngResponse represents the JSON response from SearXNG.
type searxngResponse struct {
	Results []searxngResult `json:"results"`
}

// searxngResult represents a single result from SearXNG.
type searxngResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// Search queries the SearXNG instance and returns search results.
func (s *SearXNGAdapter) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("%s/search?q=%s&format=json&categories=general",
		s.BaseURL, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search backend returned status %d", resp.StatusCode)
	}

	var sr searxngResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}

	results := make([]SearchResult, 0, min(len(sr.Results), maxResults))
	for i, r := range sr.Results {
		if i >= maxResults {
			break
		}
		results = append(results, SearchResult{
			Title:   stripHTML(r.Title),
			URL:     r.URL,
			Snippet: stripHTML(r.Content),
		})
	}

	return results, nil
}

// stripHTML removes HTML tags from text.
func stripHTML(s string) string {
	return strings.TrimSpace(htmlTagRegex.ReplaceAllString(s, ""))
}
