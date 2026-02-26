package responses

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/rhuss/antwort/pkg/provider"
)

// ResponsesProvider implements provider.Provider for backends that support
// the OpenAI Responses API (/v1/responses). It forwards inference requests
// using the Responses API wire format and consumes native SSE events.
type ResponsesProvider struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	caps       provider.ProviderCapabilities
}

// Ensure ResponsesProvider implements provider.Provider at compile time.
var _ provider.Provider = (*ResponsesProvider)(nil)

// Config holds configuration for the Responses API provider.
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// New creates a new ResponsesProvider. It validates that the backend supports
// the Responses API by probing the /v1/responses endpoint at startup (FR-013).
func New(cfg Config) (*ResponsesProvider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("responses: BaseURL is required")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second
	}

	p := &ResponsesProvider{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		caps: provider.ProviderCapabilities{
			Streaming:   true,
			ToolCalling: true,
			Vision:      true,
		},
	}

	// Probe the backend to verify Responses API support (FR-013).
	if err := p.probeEndpoint(); err != nil {
		return nil, err
	}

	return p, nil
}

// probeEndpoint sends a lightweight request to /v1/responses to verify the
// backend supports the Responses API. Connection errors and plain 404s (path
// not found) indicate the endpoint is unavailable. A JSON-formatted 404 from
// the API (e.g., "model not found") means the endpoint exists but rejected
// our probe, which is acceptable.
func (p *ResponsesProvider) probeEndpoint() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send a minimal request that will be rejected but proves the endpoint exists.
	probe := []byte(`{"model":"_probe","input":"probe","store":false}`)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/responses", bytes.NewReader(probe))
	if err != nil {
		return fmt.Errorf("responses: probe request creation failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("responses: backend at %s is not reachable: %w", p.baseURL, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		// Distinguish between "path not found" (endpoint missing) and
		// "model not found" (endpoint exists, model name invalid). vLLM
		// returns 404 with a JSON body for model-not-found errors.
		if isAPIError(respBody) {
			// The endpoint exists, it just rejected our probe model.
			slog.Info("responses provider: backend probe successful (endpoint exists)",
				"url", p.baseURL+"/v1/responses",
				"status", resp.StatusCode,
			)
			return nil
		}
		return fmt.Errorf("responses: backend at %s does not support the Responses API (/v1/responses returned 404)", p.baseURL)
	}

	slog.Info("responses provider: backend probe successful",
		"url", p.baseURL+"/v1/responses",
		"status", resp.StatusCode,
	)
	return nil
}

// isAPIError checks if a response body is a JSON API error (as opposed to a
// plain text "Not Found" from a web framework). vLLM returns JSON errors like
// {"object":"error","message":"The model '_probe' does not exist.","code":404}
// when the endpoint exists but the request is invalid.
func isAPIError(body []byte) bool {
	var obj struct {
		Object  string `json:"object"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &obj) == nil && obj.Message != "" {
		return true
	}
	return false
}

// Name returns the provider identifier.
func (p *ResponsesProvider) Name() string {
	return "vllm-responses"
}

// Capabilities returns what this provider supports.
func (p *ResponsesProvider) Capabilities() provider.ProviderCapabilities {
	return p.caps
}

// Complete performs non-streaming inference via POST /v1/responses.
func (p *ResponsesProvider) Complete(ctx context.Context, req *provider.ProviderRequest) (*provider.ProviderResponse, error) {
	req.Stream = false
	rr, err := translateRequest(req)
	if err != nil {
		return nil, fmt.Errorf("responses: translate request: %w", err)
	}

	body, err := json.Marshal(rr)
	if err != nil {
		return nil, fmt.Errorf("responses: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("responses: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	slog.Debug("request", "debug", "providers", "method", "POST",
		"url", p.baseURL+"/v1/responses", "model", req.Model, "stream", false)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("responses: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("responses: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var apiErr responsesError
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("responses: backend error (%d): %s", resp.StatusCode, apiErr.Message)
		}
		return nil, fmt.Errorf("responses: backend returned %d: %s", resp.StatusCode, string(respBody))
	}

	var rResp responsesResponse
	if err := json.Unmarshal(respBody, &rResp); err != nil {
		return nil, fmt.Errorf("responses: unmarshal response: %w", err)
	}

	return translateResponse(&rResp)
}

// Stream performs streaming inference via POST /v1/responses with stream=true.
// Returns a channel of ProviderEvents parsed from the backend's SSE stream.
func (p *ResponsesProvider) Stream(ctx context.Context, req *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
	req.Stream = true
	rr, err := translateRequest(req)
	if err != nil {
		return nil, fmt.Errorf("responses: translate request: %w", err)
	}

	body, err := json.Marshal(rr)
	if err != nil {
		return nil, fmt.Errorf("responses: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("responses: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	slog.Debug("request", "debug", "providers", "method", "POST",
		"url", p.baseURL+"/v1/responses", "model", req.Model, "stream", true)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("responses: HTTP request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("responses: backend returned %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan provider.ProviderEvent, 32)
	go func() {
		defer resp.Body.Close()
		parseSSEStream(resp.Body, ch)
	}()

	return ch, nil
}

// ListModels queries the backend's /v1/models endpoint.
func (p *ResponsesProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("responses: create request: %w", err)
	}
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("responses: list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("responses: list models returned %d", resp.StatusCode)
	}

	var result struct {
		Data []provider.ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("responses: decode models: %w", err)
	}
	return result.Data, nil
}

// Close releases provider resources.
func (p *ResponsesProvider) Close() error {
	p.httpClient.CloseIdleConnections()
	return nil
}
