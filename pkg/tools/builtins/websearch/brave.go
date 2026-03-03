package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// BraveAdapter implements SearchAdapter using the Brave Search API.
type BraveAdapter struct {
	APIKey     string
	HTTPClient *http.Client
}

// Compile-time check.
var _ SearchAdapter = (*BraveAdapter)(nil)

// NewBrave creates a Brave Search adapter with the given API key.
func NewBrave(apiKey string) *BraveAdapter {
	return &BraveAdapter{
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
	}
}

// braveResponse represents the JSON response from Brave Web Search API.
type braveResponse struct {
	Web braveWebResults `json:"web"`
}

type braveWebResults struct {
	Results []braveResult `json:"results"`
}

type braveResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// Search queries the Brave Search API and returns results.
func (b *BraveAdapter) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=%d",
		url.QueryEscape(query), maxResults)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("brave: creating request: %w", err)
	}
	req.Header.Set("X-Subscription-Token", b.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := b.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("brave: search service unavailable: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("brave: API key is invalid or expired. Check your configuration")
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("brave: rate limit exceeded. Try again later")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("brave: search returned status %d", resp.StatusCode)
	}

	var br braveResponse
	if err := json.NewDecoder(resp.Body).Decode(&br); err != nil {
		return nil, fmt.Errorf("brave: decoding response: %w", err)
	}

	results := make([]SearchResult, 0, min(len(br.Web.Results), maxResults))
	for i, r := range br.Web.Results {
		if i >= maxResults {
			break
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: strings.TrimSpace(r.Description),
		})
	}

	return results, nil
}
