package codeinterpreter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SandboxClient calls the sandbox server's REST API to execute code.
type SandboxClient struct {
	httpClient *http.Client
}

// NewSandboxClient creates a new sandbox HTTP client.
func NewSandboxClient() *SandboxClient {
	return &SandboxClient{
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Overall HTTP timeout (execution timeout is enforced by the sandbox).
		},
	}
}

// Execute sends a code execution request to the sandbox server and returns the result.
func (c *SandboxClient) Execute(ctx context.Context, sandboxURL string, req *SandboxRequest) (*SandboxResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, sandboxURL+"/execute", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sandbox request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("sandbox at capacity (HTTP 429)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sandbox returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var sandboxResp SandboxResponse
	if err := json.Unmarshal(respBody, &sandboxResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &sandboxResp, nil
}
