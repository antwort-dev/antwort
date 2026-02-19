package vllm

import (
	"context"
	"fmt"
	"time"

	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/provider/openaicompat"
)

// VLLMProvider implements provider.Provider for vLLM and OpenAI-compatible
// Chat Completions backends. It delegates HTTP communication to the shared
// openaicompat.Client.
type VLLMProvider struct {
	cfg    Config
	client *openaicompat.Client
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

	// Apply default timeout if not set.
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second
	}

	client := openaicompat.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Timeout)

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
	return p.client.Complete(ctx, req)
}

// Stream performs streaming inference against the Chat Completions endpoint.
// It returns a channel of ProviderEvents. The channel is closed when the
// stream completes, errors, or the context is cancelled.
func (p *VLLMProvider) Stream(ctx context.Context, req *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
	return p.client.Stream(ctx, req)
}

// ListModels returns available models from the backend by querying
// the /v1/models endpoint.
func (p *VLLMProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	return p.client.ListModels(ctx)
}

// Close releases provider resources.
func (p *VLLMProvider) Close() error {
	return p.client.Close()
}
