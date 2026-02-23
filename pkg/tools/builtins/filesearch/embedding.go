package filesearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// EmbeddingClient embeds text via an external service.
type EmbeddingClient interface {
	// Embed converts a batch of text strings into embedding vectors.
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the dimensionality of the embedding vectors.
	// Returns 0 until the first successful Embed call.
	Dimensions() int
}

// OpenAIEmbeddingClient calls any OpenAI-compatible /v1/embeddings endpoint.
type OpenAIEmbeddingClient struct {
	URL        string
	Model      string
	HTTPClient *http.Client

	mu   sync.RWMutex
	dims int
}

// NewOpenAIEmbeddingClient creates a new embedding client for an OpenAI-compatible endpoint.
func NewOpenAIEmbeddingClient(url, model string) *OpenAIEmbeddingClient {
	return &OpenAIEmbeddingClient{
		URL:        url,
		Model:      model,
		HTTPClient: &http.Client{},
	}
}

// embeddingRequest is the JSON request body for the embeddings API.
type embeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

// embeddingResponse is the JSON response from the embeddings API.
type embeddingResponse struct {
	Data []embeddingData `json:"data"`
}

type embeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// Embed sends texts to the embeddings endpoint and returns the vectors.
func (c *OpenAIEmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Build the endpoint URL.
	endpoint := c.URL
	if !strings.HasSuffix(endpoint, "/v1/embeddings") {
		endpoint = strings.TrimRight(endpoint, "/") + "/v1/embeddings"
	}

	reqBody := embeddingRequest{
		Input: texts,
		Model: c.Model,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading embedding response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("parsing embedding response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("embedding response contained no data")
	}

	// Order results by index.
	vectors := make([][]float32, len(texts))
	for _, d := range embResp.Data {
		if d.Index < 0 || d.Index >= len(texts) {
			return nil, fmt.Errorf("embedding response index %d out of range [0, %d)", d.Index, len(texts))
		}
		vectors[d.Index] = d.Embedding
	}

	// Set dimensions from the first successful response.
	if len(vectors[0]) > 0 {
		c.mu.Lock()
		if c.dims == 0 {
			c.dims = len(vectors[0])
		}
		c.mu.Unlock()
	}

	return vectors, nil
}

// Dimensions returns the dimensionality of the embedding vectors.
// Returns 0 until the first successful Embed call.
func (c *OpenAIEmbeddingClient) Dimensions() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dims
}
