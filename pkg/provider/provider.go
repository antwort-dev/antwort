package provider

import (
	"context"

	"github.com/rhuss/antwort/pkg/api"
)

// Provider abstracts an LLM inference backend. The interface is
// protocol-agnostic: each adapter handles its own backend protocol
// (Chat Completions, Responses API, etc.) internally.
//
// Implementations must be safe for concurrent use by multiple goroutines.
type Provider interface {
	// Name returns the provider identifier (e.g., "vllm", "litellm").
	Name() string

	// Capabilities returns what this provider supports.
	Capabilities() ProviderCapabilities

	// Complete performs non-streaming inference.
	Complete(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)

	// Stream performs streaming inference. The returned channel receives
	// ProviderEvent values and is closed by the provider when the stream
	// completes or errors.
	Stream(ctx context.Context, req *ProviderRequest) (<-chan ProviderEvent, error)

	// ListModels returns available models from the backend.
	ListModels(ctx context.Context) ([]ModelInfo, error)

	// Close releases provider resources (HTTP clients, connections).
	Close() error
}

// Translator converts between OpenResponses request types and provider
// request types. Each adapter may implement its own translation or embed
// a shared translator.
type Translator interface {
	// TranslateRequest converts an OpenResponses request to a provider request.
	TranslateRequest(ctx context.Context, req *api.CreateResponseRequest) (*ProviderRequest, error)

	// TranslateResponse converts a provider response to OpenResponses items.
	TranslateResponse(ctx context.Context, resp *ProviderResponse) ([]api.Item, *api.Usage, error)
}
