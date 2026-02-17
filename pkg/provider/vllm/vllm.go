package vllm

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

// VLLMProvider implements provider.Provider for vLLM and OpenAI-compatible
// Chat Completions backends.
type VLLMProvider struct {
	cfg    Config
	client *http.Client
	caps   provider.ProviderCapabilities
}

// Ensure VLLMProvider implements provider.Provider at compile time.
var _ provider.Provider = (*VLLMProvider)(nil)

// New creates a new VLLMProvider with the given configuration.
// Returns an error if the configuration is invalid.
func New(cfg Config) (*VLLMProvider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("vllm: BaseURL is required")
	}

	// Normalize: remove trailing slash from base URL.
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	// Apply default timeout if not set.
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	return &VLLMProvider{
		cfg:    cfg,
		client: client,
		caps: provider.ProviderCapabilities{
			Streaming:   true,
			ToolCalling: true,
		},
	}, nil
}

// NewWithCapabilities creates a new VLLMProvider with custom capabilities.
func NewWithCapabilities(cfg Config, caps provider.ProviderCapabilities) (*VLLMProvider, error) {
	p, err := New(cfg)
	if err != nil {
		return nil, err
	}
	p.caps = caps
	return p, nil
}

// Name returns the provider identifier.
func (p *VLLMProvider) Name() string {
	return "vllm"
}

// Capabilities returns what this provider supports.
func (p *VLLMProvider) Capabilities() provider.ProviderCapabilities {
	return p.caps
}

// Complete performs non-streaming inference against the Chat Completions endpoint.
func (p *VLLMProvider) Complete(ctx context.Context, req *provider.ProviderRequest) (*provider.ProviderResponse, error) {
	// Ensure we are not in streaming mode for Complete.
	reqCopy := *req
	reqCopy.Stream = false

	// Translate to Chat Completions format.
	chatReq := translateToChat(&reqCopy)

	// Marshal request body.
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, api.NewServerError(fmt.Sprintf("failed to marshal request: %s", err.Error()))
	}

	// Build HTTP request.
	url := p.cfg.BaseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, api.NewServerError(fmt.Sprintf("failed to create HTTP request: %s", err.Error()))
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	// Send request.
	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, mapNetworkError(err)
	}
	defer httpResp.Body.Close()

	// Check for error status codes.
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, mapHTTPError(httpResp)
	}

	// Parse response.
	var chatResp chatCompletionResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&chatResp); err != nil {
		return nil, api.NewServerError(fmt.Sprintf("failed to parse backend response: %s", err.Error()))
	}

	// Translate to ProviderResponse.
	return translateResponse(&chatResp), nil
}

// Stream performs streaming inference against the Chat Completions endpoint.
// It returns a channel of ProviderEvents. The channel is closed when the
// stream completes, errors, or the context is cancelled.
//
// The HTTP client timeout is not applied for streaming requests because a
// stream can legitimately last longer than any fixed timeout. Lifecycle
// control relies on context cancellation instead.
func (p *VLLMProvider) Stream(ctx context.Context, req *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
	// Force streaming mode.
	reqCopy := *req
	reqCopy.Stream = true

	// Translate to Chat Completions format (includes stream_options).
	chatReq := translateToChat(&reqCopy)

	// Marshal request body.
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, api.NewServerError(fmt.Sprintf("failed to marshal request: %s", err.Error()))
	}

	// Build HTTP request.
	url := p.cfg.BaseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, api.NewServerError(fmt.Sprintf("failed to create HTTP request: %s", err.Error()))
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	// Use a client without timeout for streaming. The context controls
	// the request lifetime instead.
	streamClient := &http.Client{
		Transport: p.client.Transport,
	}

	// Send request.
	httpResp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, mapNetworkError(err)
	}

	// Check for error status codes before starting the stream.
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		httpResp.Body.Close()
		return nil, mapHTTPError(httpResp)
	}

	// Create the event channel and spawn a goroutine to parse the stream.
	ch := make(chan provider.ProviderEvent, 16)

	go func() {
		defer close(ch)
		defer httpResp.Body.Close()
		parseSSEStream(ctx, httpResp.Body, ch)
	}()

	return ch, nil
}

// ListModels returns available models from the backend. Stub for now.
func (p *VLLMProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	return nil, nil
}

// Close releases provider resources.
func (p *VLLMProvider) Close() error {
	p.client.CloseIdleConnections()
	return nil
}
