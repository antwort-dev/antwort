// Package qdrant implements the vectorstore.Backend interface using the Qdrant HTTP API.
package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rhuss/antwort/pkg/vectorstore"
)

// Backend implements vectorstore.Backend using the Qdrant HTTP API.
type Backend struct {
	BaseURL    string
	HTTPClient *http.Client
}

// Compile-time check.
var _ vectorstore.Backend = (*Backend)(nil)

// New creates a new Qdrant backend that communicates via HTTP.
func New(url string) *Backend {
	return &Backend{
		BaseURL:    strings.TrimRight(url, "/"),
		HTTPClient: &http.Client{},
	}
}

func (q *Backend) CreateCollection(ctx context.Context, name string, dimensions int) error {
	body := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     dimensions,
			"distance": "Cosine",
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling create collection request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s", q.BaseURL, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant create collection request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant create collection returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (q *Backend) DeleteCollection(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/collections/%s", q.BaseURL, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := q.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant delete collection request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant delete collection returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

type searchRequest struct {
	Vector      []float32 `json:"vector"`
	Limit       int       `json:"limit"`
	WithPayload bool      `json:"with_payload"`
}

type searchResponse struct {
	Result []searchResult `json:"result"`
}

type searchResult struct {
	ID      interface{}            `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

func (q *Backend) Search(ctx context.Context, collection string, vector []float32, maxResults int) ([]vectorstore.SearchMatch, error) {
	searchReq := searchRequest{
		Vector:      vector,
		Limit:       maxResults,
		WithPayload: true,
	}

	data, err := json.Marshal(searchReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling search request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/search", q.BaseURL, collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("creating search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qdrant search request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading search response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qdrant search returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var searchResp searchResponse
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}

	matches := make([]vectorstore.SearchMatch, 0, len(searchResp.Result))
	for _, r := range searchResp.Result {
		match := vectorstore.SearchMatch{
			DocumentID: fmt.Sprintf("%v", r.ID),
			Score:      r.Score,
			Metadata:   make(map[string]string),
		}

		if content, ok := r.Payload["content"].(string); ok {
			match.Content = content
		}
		for k, v := range r.Payload {
			if k == "content" {
				continue
			}
			if s, ok := v.(string); ok {
				match.Metadata[k] = s
			}
		}

		matches = append(matches, match)
	}

	return matches, nil
}

type point struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

func (q *Backend) UpsertPoints(ctx context.Context, collection string, points []vectorstore.VectorPoint) error {
	qPoints := make([]point, len(points))
	for i, p := range points {
		payload := make(map[string]interface{}, len(p.Metadata))
		for k, v := range p.Metadata {
			payload[k] = v
		}
		qPoints[i] = point{
			ID:      p.ID,
			Vector:  p.Vector,
			Payload: payload,
		}
	}

	body := map[string]interface{}{
		"points": qPoints,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling upsert request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points", q.BaseURL, collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating upsert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant upsert request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant upsert returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (q *Backend) DeletePointsByFile(ctx context.Context, collection string, fileID string) error {
	body := map[string]interface{}{
		"filter": map[string]interface{}{
			"must": []map[string]interface{}{
				{
					"key":   "file_id",
					"match": map[string]interface{}{"value": fileID},
				},
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling delete request: %w", err)
	}

	url := fmt.Sprintf("%s/collections/%s/points/delete", q.BaseURL, collection)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating delete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant delete points request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qdrant delete points returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
