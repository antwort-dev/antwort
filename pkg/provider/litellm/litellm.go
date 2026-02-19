package litellm

import (
	"context"
	"fmt"
	"time"

	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/provider/openaicompat"
)

// LiteLLMProvider implements provider.Provider for LiteLLM proxy servers.
// It delegates HTTP communication to the shared openaicompat.Client and
// supports model name mapping for multi-provider routing.
type LiteLLMProvider struct {
	cfg    Config
	client *openaicompat.Client
	caps   provider.ProviderCapabilities
}

// Ensure LiteLLMProvider implements provider.Provider at compile time.
var _ provider.Provider = (*LiteLLMProvider)(nil)

// New creates a new LiteLLMProvider with the given configuration.
// Returns an error if the configuration is invalid.
func New(cfg Config) (*LiteLLMProvider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("litellm: BaseURL is required")
	}

	// Apply default timeout if not set.
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second
	}

	client := openaicompat.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Timeout)

	// If model mapping is configured, set a mapper on the client.
	if len(cfg.ModelMapping) > 0 {
		mapping := cfg.ModelMapping
		client.ModelMapper = func(model string) string {
			if mapped, ok := mapping[model]; ok {
				return mapped
			}
			return model
		}
	}

	return &LiteLLMProvider{
		cfg:    cfg,
		client: client,
		caps: provider.ProviderCapabilities{
			Streaming:   true,
			ToolCalling: true,
		},
	}, nil
}

// Name returns the provider identifier.
func (p *LiteLLMProvider) Name() string {
	return "litellm"
}

// Capabilities returns what this provider supports.
func (p *LiteLLMProvider) Capabilities() provider.ProviderCapabilities {
	return p.caps
}

// Complete performs non-streaming inference against the Chat Completions endpoint.
func (p *LiteLLMProvider) Complete(ctx context.Context, req *provider.ProviderRequest) (*provider.ProviderResponse, error) {
	return p.client.Complete(ctx, req)
}

// Stream performs streaming inference against the Chat Completions endpoint.
// It returns a channel of ProviderEvents. The channel is closed when the
// stream completes, errors, or the context is cancelled.
func (p *LiteLLMProvider) Stream(ctx context.Context, req *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
	return p.client.Stream(ctx, req)
}

// ListModels returns available models from the backend by querying
// the /v1/models endpoint.
func (p *LiteLLMProvider) ListModels(ctx context.Context) ([]provider.ModelInfo, error) {
	return p.client.ListModels(ctx)
}

// Close releases provider resources.
func (p *LiteLLMProvider) Close() error {
	return p.client.Close()
}
