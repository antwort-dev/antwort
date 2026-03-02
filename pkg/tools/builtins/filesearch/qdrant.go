package filesearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rhuss/antwort/pkg/files"
)

// QdrantBackend implements VectorStoreBackend using the Qdrant HTTP API.
type QdrantBackend struct {
	BaseURL    string
	HTTPClient *http.Client
}

// Compile-time checks.
var (
	_ VectorStoreBackend  = (*QdrantBackend)(nil)
	_ files.VectorIndexer = (*QdrantBackend)(nil)
)

// NewQdrant creates a new QdrantBackend that communicates with Qdrant via HTTP.
func NewQdrant(url string) *QdrantBackend {
	return &QdrantBackend{
		BaseURL:    strings.TrimRight(url, "/"),
		HTTPClient: &http.Client{},
	}
}

// CreateCollection creates a new vector collection in Qdrant.
// PUT /collections/{name} with {"vectors": {"size": dims, "distance": "Cosine"}}
func (q *QdrantBackend) CreateCollection(ctx context.Context, name string, dimensions int) error {
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

// DeleteCollection removes a vector collection from Qdrant.
// DELETE /collections/{name}
func (q *QdrantBackend) DeleteCollection(ctx context.Context, name string) error {
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

// qdrantSearchRequest is the JSON body for Qdrant's search endpoint.
type qdrantSearchRequest struct {
	Vector      []float32 `json:"vector"`
	Limit       int       `json:"limit"`
	WithPayload bool      `json:"with_payload"`
}

// qdrantSearchResponse represents Qdrant's search response.
type qdrantSearchResponse struct {
	Result []qdrantSearchResult `json:"result"`
}

type qdrantSearchResult struct {
	ID      interface{}            `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

// Search performs a nearest-neighbor search in the named collection.
// POST /collections/{name}/points/search
func (q *QdrantBackend) Search(ctx context.Context, collection string, vector []float32, maxResults int) ([]SearchMatch, error) {
	searchReq := qdrantSearchRequest{
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

	var searchResp qdrantSearchResponse
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}

	matches := make([]SearchMatch, 0, len(searchResp.Result))
	for _, r := range searchResp.Result {
		match := SearchMatch{
			DocumentID: fmt.Sprintf("%v", r.ID),
			Score:      r.Score,
			Metadata:   make(map[string]string),
		}

		// Extract content and metadata from payload.
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

// qdrantPoint represents a point for Qdrant's upsert endpoint.
type qdrantPoint struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

// UpsertPoints inserts or updates vector points in the named collection.
// PUT /collections/{name}/points
func (q *QdrantBackend) UpsertPoints(ctx context.Context, collection string, points []files.VectorPoint) error {
	qPoints := make([]qdrantPoint, len(points))
	for i, p := range points {
		payload := make(map[string]interface{}, len(p.Metadata))
		for k, v := range p.Metadata {
			payload[k] = v
		}
		qPoints[i] = qdrantPoint{
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

// DeletePointsByFile removes all points with the given file_id from the collection.
// POST /collections/{name}/points/delete with filter
func (q *QdrantBackend) DeletePointsByFile(ctx context.Context, collection string, fileID string) error {
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
