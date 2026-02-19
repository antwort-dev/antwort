package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

// Client performs HTTP requests against an OpenAI-compatible Chat Completions
// backend. It uses the shared translate/response/stream/errors code.
//
// Provider adapters embed this Client and delegate their Complete/Stream/
// ListModels calls to it.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string

	// ModelMapper is an optional function that transforms the model name
	// before sending it to the backend. If nil, the model name is used as-is.
	ModelMapper func(string) string
}

// NewClient creates a new Client for an OpenAI-compatible backend.
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	// Normalize: remove trailing slash from base URL.
	baseURL = strings.TrimRight(baseURL, "/")

	if timeout == 0 {
		timeout = 120 * time.Second
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// Complete performs non-streaming inference against the Chat Completions endpoint.
func (c *Client) Complete(ctx context.Context, req *provider.ProviderRequest) (*provider.ProviderResponse, error) {
	// Ensure we are not in streaming mode for Complete.
	reqCopy := *req
	reqCopy.Stream = false

	// Apply model mapping if configured.
	if c.ModelMapper != nil {
		reqCopy.Model = c.ModelMapper(reqCopy.Model)
	}

	// Translate to Chat Completions format.
	chatReq := TranslateToChat(&reqCopy)

	// Marshal request body.
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, api.NewServerError(fmt.Sprintf("failed to marshal request: %s", err.Error()))
	}

	// Build HTTP request.
	url := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, api.NewServerError(fmt.Sprintf("failed to create HTTP request: %s", err.Error()))
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Send request.
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, MapNetworkError(err)
	}
	defer httpResp.Body.Close()

	// Check for error status codes.
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, MapHTTPError(httpResp)
	}

	// Parse response.
	var chatResp ChatCompletionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&chatResp); err != nil {
		return nil, api.NewServerError(fmt.Sprintf("failed to parse backend response: %s", err.Error()))
	}

	// Translate to ProviderResponse.
	return TranslateResponse(&chatResp), nil
}

// Stream performs streaming inference against the Chat Completions endpoint.
// It returns a channel of ProviderEvents. The channel is closed when the
// stream completes, errors, or the context is cancelled.
//
// The HTTP client timeout is not applied for streaming requests because a
// stream can legitimately last longer than any fixed timeout. Lifecycle
// control relies on context cancellation instead.
func (c *Client) Stream(ctx context.Context, req *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
	// Force streaming mode.
	reqCopy := *req
	reqCopy.Stream = true

	// Apply model mapping if configured.
	if c.ModelMapper != nil {
		reqCopy.Model = c.ModelMapper(reqCopy.Model)
	}

	// Translate to Chat Completions format (includes stream_options).
	chatReq := TranslateToChat(&reqCopy)

	// Marshal request body.
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, api.NewServerError(fmt.Sprintf("failed to marshal request: %s", err.Error()))
	}

	// Build HTTP request.
	url := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, api.NewServerError(fmt.Sprintf("failed to create HTTP request: %s", err.Error()))
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Use a client without timeout for streaming. The context controls
	// the request lifetime instead.
	streamClient := &http.Client{
		Transport: c.httpClient.Transport,
	}

	// Send request.
	httpResp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, MapNetworkError(err)
	}

	// Check for error status codes before starting the stream.
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		httpResp.Body.Close()
		return nil, MapHTTPError(httpResp)
	}

	// Create the event channel and spawn a goroutine to parse the stream.
	ch := make(chan provider.ProviderEvent, 16)

	go func() {
		defer close(ch)
		defer httpResp.Body.Close()
		ParseSSEStream(ctx, httpResp.Body, ch)
	}()

	return ch, nil
}

// ListModels returns available models from the backend by querying
// the /v1/models endpoint.
func (c *Client) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	url := c.baseURL + "/v1/models"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, api.NewServerError(fmt.Sprintf("failed to create HTTP request: %s", err.Error()))
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, MapNetworkError(err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, MapHTTPError(httpResp)
	}

	var modelsResp ChatModelsResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&modelsResp); err != nil {
		return nil, api.NewServerError(fmt.Sprintf("failed to parse models response: %s", err.Error()))
	}

	var models []provider.ModelInfo
	for _, m := range modelsResp.Data {
		models = append(models, provider.ModelInfo{
			ID:      m.ID,
			Object:  m.Object,
			OwnedBy: m.OwnedBy,
		})
	}

	return models, nil
}

// Close releases client resources.
func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
