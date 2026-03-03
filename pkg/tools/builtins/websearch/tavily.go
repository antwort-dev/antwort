package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// TavilyAdapter implements SearchAdapter using the Tavily Search API.
type TavilyAdapter struct {
	APIKey     string
	HTTPClient *http.Client
}

// Compile-time check.
var _ SearchAdapter = (*TavilyAdapter)(nil)

// NewTavily creates a Tavily Search adapter with the given API key.
func NewTavily(apiKey string) *TavilyAdapter {
	return &TavilyAdapter{
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
	}
}

// tavilyRequest is the JSON body for Tavily's search endpoint.
type tavilyRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

// tavilyResponse represents the JSON response from Tavily Search API.
type tavilyResponse struct {
	Results []tavilyResult `json:"results"`
}

type tavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// Search queries the Tavily Search API and returns results.
func (t *TavilyAdapter) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	body := tavilyRequest{
		Query:      query,
		MaxResults: maxResults,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("tavily: marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("tavily: creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.APIKey)

	resp, err := t.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily: search service unavailable: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("tavily: API key is invalid or expired. Check your configuration")
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("tavily: rate limit exceeded. Try again later")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tavily: search returned status %d", resp.StatusCode)
	}

	var tr tavilyResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, fmt.Errorf("tavily: decoding response: %w", err)
	}

	results := make([]SearchResult, 0, len(tr.Results))
	for _, r := range tr.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: strings.TrimSpace(r.Content),
		})
	}

	return results, nil
}
