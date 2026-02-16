# Spec 08: Provider - LiteLLM

**Branch**: `spec/08-provider-litellm`
**Dependencies**: Spec 01 (Core Protocol), Spec 03 (Provider Abstraction)
**Package**: `github.com/rhuss/antwort/pkg/provider/litellm`

## Purpose

Implement a LiteLLM provider adapter using the provider interface defined in Spec 03. LiteLLM exposes an OpenAI-compatible API and supports 100+ LLM backends, making it a high-value second provider that validates the interface design and opens access to a broad model ecosystem.

## Scope

### In Scope
- LiteLLM adapter implementing `provider.Provider`
- Request/response translation (OpenResponses -> LiteLLM OpenAI-compatible API)
- Streaming support via LiteLLM's SSE endpoint
- Model routing and discovery via LiteLLM's model list
- LiteLLM-specific features (fallback, load balancing, cost tracking)
- Configuration for connecting to a LiteLLM proxy instance

### Out of Scope
- Running LiteLLM itself (antwort connects to an existing LiteLLM proxy)
- LiteLLM SDK embedding (we use the HTTP API only)

## Why LiteLLM as Second Provider

Both vLLM and LiteLLM expose OpenAI-compatible APIs, which validates a key design decision: the provider interface's `Translator` abstraction should handle the OpenAI chat completions format as a shared base, with provider-specific extensions layered on top.

```
                    ┌──────────────────┐
                    │ OpenAI-compat    │
                    │ Base Translator  │
                    └────────┬─────────┘
                             │
                 ┌───────────┼───────────┐
                 │           │           │
           ┌─────▼────┐ ┌───▼─────┐ ┌───▼──────┐
           │  vLLM     │ │ LiteLLM │ │ Future   │
           │  Adapter  │ │ Adapter │ │ OAI-compat│
           └──────────┘ └─────────┘ └──────────┘
```

## Shared OpenAI-Compatible Base

Since both vLLM and LiteLLM use the same wire format, we extract a shared translator:

```go
// OpenAICompatTranslator handles the common OpenAI chat completions format.
// Both VLLMProvider and LiteLLMProvider embed this.
type OpenAICompatTranslator struct {
    // Override points for provider-specific behavior
    mapModel     func(string) string
    extraParams  func(*api.CreateResponseRequest) map[string]any
}

// TranslateRequest converts OpenResponses -> OpenAI ChatCompletion format.
// This is the shared implementation used by vLLM and LiteLLM.
func (t *OpenAICompatTranslator) TranslateRequest(
    ctx context.Context,
    req *api.CreateResponseRequest,
) (*ProviderRequest, error)

// TranslateResponse converts OpenAI ChatCompletion -> OpenResponses items.
func (t *OpenAICompatTranslator) TranslateResponse(
    ctx context.Context,
    resp *ProviderResponse,
) ([]api.Item, *api.Usage, error)
```

This refactoring happens as part of Spec 08 and retroactively simplifies the vLLM adapter from Spec 03.

## LiteLLM Adapter

```go
// LiteLLMProvider connects to a LiteLLM proxy instance.
type LiteLLMProvider struct {
    client     *http.Client
    baseURL    string
    apiKey     string
    translator *OpenAICompatTranslator
    caps       ProviderCapabilities
}

func NewLiteLLMProvider(cfg LiteLLMConfig) (*LiteLLMProvider, error)

func (p *LiteLLMProvider) Name() string { return "litellm" }

func (p *LiteLLMProvider) Capabilities() ProviderCapabilities {
    return ProviderCapabilities{
        Streaming:   true,
        ToolCalling: true,
        Vision:      true,  // depends on backend model
        Reasoning:   true,  // depends on backend model
        // Capabilities are dynamic: LiteLLM routes to many backends
        // with different capabilities per model.
    }
}
```

### LiteLLM-Specific Features

LiteLLM adds capabilities beyond plain OpenAI compatibility:

```go
// LiteLLM-specific request parameters passed via Extra/Extensions:
//
// "litellm:fallbacks"     - List of fallback models if primary fails
// "litellm:metadata"      - Cost tracking metadata (user, team, project)
// "litellm:cache"         - Enable/disable LiteLLM response caching
// "litellm:mock_response" - Return mock response (testing)
// "litellm:num_retries"   - Override retry count

// These map to LiteLLM's extra_body parameters:
type LiteLLMExtras struct {
    Fallbacks    []string          `json:"fallbacks,omitempty"`
    Metadata     map[string]string `json:"metadata,omitempty"`
    CacheEnabled *bool             `json:"cache,omitempty"`
    NumRetries   *int              `json:"num_retries,omitempty"`
}
```

### Model Discovery

LiteLLM provides a model list endpoint that returns all configured models:

```go
// ListModels queries LiteLLM's /v1/models endpoint.
// LiteLLM returns models from all configured providers,
// so the list may be large and heterogeneous.
func (p *LiteLLMProvider) ListModels(ctx context.Context) ([]ModelInfo, error)

// ModelInfo includes LiteLLM-specific metadata:
// - Which underlying provider serves this model
// - Cost per token (input/output)
// - Rate limits
// - Supported features (vision, function calling, etc.)
```

### Differences from vLLM Adapter

| Aspect | vLLM | LiteLLM |
|---|---|---|
| Models | Single model per instance | Many models, many providers |
| Capabilities | Static (known at startup) | Dynamic (varies per model) |
| Error format | Standard OpenAI | Extended with provider context |
| Extra params | Guided decoding, sampling | Fallbacks, caching, cost metadata |
| Health | Single endpoint | Multi-backend health |
| Model names | Direct pass-through | May need prefix (e.g., `anthropic/claude-3`) |

### Configuration

```go
type LiteLLMConfig struct {
    // BaseURL is the LiteLLM proxy URL (e.g., "http://litellm:4000").
    BaseURL string `json:"base_url" env:"ANTWORT_LITELLM_URL"`

    // APIKey for LiteLLM virtual key authentication.
    APIKey string `json:"api_key" env:"ANTWORT_LITELLM_API_KEY"`

    // Timeout for individual requests.
    Timeout time.Duration `json:"timeout" env:"ANTWORT_LITELLM_TIMEOUT"`

    // DefaultModel to use when the request doesn't specify one.
    DefaultModel string `json:"default_model" env:"ANTWORT_LITELLM_DEFAULT_MODEL"`

    // ModelMapping maps OpenResponses model names to LiteLLM model identifiers.
    // Example: {"gpt-4": "openai/gpt-4", "claude": "anthropic/claude-3-sonnet"}
    ModelMapping map[string]string `json:"model_mapping,omitempty"`
}
```

## Multi-Provider Routing (Future Consideration)

With two providers, the question of routing emerges. While full multi-provider routing is out of scope, the interface should accommodate it:

```go
// ProviderRouter selects a provider for a given request.
// Initial implementation: static config (one provider per model name).
// Future: capability-based, cost-based, or latency-based routing.
type ProviderRouter interface {
    Route(ctx context.Context, req *api.CreateResponseRequest) (Provider, error)
}

// StaticRouter maps model names to providers via configuration.
type StaticRouter struct {
    providers map[string]Provider // model prefix -> provider
    fallback  Provider
}
```

## Extension Points

- **Model mapping**: `ModelMapping` config translates user-facing model names to provider-specific identifiers
- **LiteLLM extensions**: Provider-prefixed fields (`litellm:fallbacks`, `litellm:metadata`) flow through the Extensions map without modifying core types
- **Per-model capabilities**: Dynamic capability checking via LiteLLM's model info endpoint
- **Provider routing**: `ProviderRouter` interface enables future multi-provider selection logic

## Open Questions

- Should antwort query LiteLLM's `/model/info` endpoint to get per-model capabilities dynamically?
- How to handle model name normalization (user says "claude-3" but LiteLLM wants "anthropic/claude-3-sonnet-20240229")?
- Should the `OpenAICompatTranslator` refactoring be done in Spec 03 or deferred to Spec 08?
- How to surface LiteLLM's cost tracking data in OpenResponses extensions?

## Deliverables

- [ ] `pkg/provider/openai/translator.go` - Shared OpenAI-compatible translator (refactored from vLLM)
- [ ] `pkg/provider/litellm/litellm.go` - LiteLLM adapter
- [ ] `pkg/provider/litellm/config.go` - Configuration
- [ ] `pkg/provider/litellm/extras.go` - LiteLLM-specific extensions
- [ ] `pkg/provider/router.go` - ProviderRouter interface + StaticRouter
- [ ] `pkg/provider/litellm/litellm_test.go` - Tests
- [ ] Refactor `pkg/provider/vllm/` to use shared translator
