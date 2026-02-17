# Spec 03: Provider Abstraction & vLLM Adapter

**Branch**: `spec/03-provider-vllm`
**Dependencies**: Spec 01 (Core Protocol)
**Package**: `github.com/rhuss/antwort/pkg/provider`

## Purpose

Define the provider interface that abstracts LLM backend communication, and implement the first adapter for vLLM. The interface must be specific enough to enable clean implementations while remaining open to additional backends (LiteLLM in Spec 08, and future providers).

## Scope

### In Scope
- Provider interface definition with capability negotiation
- Request/response translation contract
- Streaming token delivery abstraction
- vLLM adapter (OpenAI-compatible `/v1/chat/completions`)
- Model discovery and listing
- Context window and truncation handling
- Error mapping (provider errors -> OpenResponses errors)

### Out of Scope
- Tool execution (see Spec 04, but tool *invocation* in requests is in scope)
- LiteLLM adapter (see Spec 08)
- Authentication to the provider (credentials passed via config)

## Provider Interface

The interface is designed around three concerns: capabilities, inference, and model management.

```go
// Provider abstracts an LLM inference backend.
type Provider interface {
    // Name returns the provider identifier (e.g., "vllm", "litellm").
    Name() string

    // Capabilities returns what this provider supports.
    // Used by the core engine to validate requests and skip unsupported features.
    Capabilities() ProviderCapabilities

    // Complete performs non-streaming inference.
    // Translates the OpenResponses request to the provider's native format,
    // calls the backend, and returns the result as OpenResponses items.
    Complete(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)

    // Stream performs streaming inference.
    // Returns a channel of ProviderEvents that the transport layer
    // converts to OpenResponses StreamEvents.
    Stream(ctx context.Context, req *ProviderRequest) (<-chan ProviderEvent, error)

    // ListModels returns available models from the backend.
    ListModels(ctx context.Context) ([]ModelInfo, error)

    // Close releases provider resources (HTTP clients, connections).
    Close() error
}

// ProviderCapabilities declares what the backend supports.
// The core engine uses this to:
//   - Reject unsupported requests early with clear errors
//   - Skip features the backend cannot handle
//   - Route requests to capable providers (future multi-provider)
type ProviderCapabilities struct {
    // Streaming indicates whether the provider supports streaming responses.
    Streaming bool

    // ToolCalling indicates whether the provider supports function/tool calls.
    ToolCalling bool

    // Vision indicates whether the provider supports image inputs.
    Vision bool

    // Audio indicates whether the provider supports audio inputs.
    Audio bool

    // Reasoning indicates whether the provider can produce reasoning items.
    Reasoning bool

    // MaxContextWindow is the maximum token count (0 = unknown/unlimited).
    MaxContextWindow int

    // SupportedModels lists models this provider can serve.
    // Empty means "ask ListModels()".
    SupportedModels []string

    // Extensions lists provider-specific extension types supported.
    Extensions []string
}
```

## Provider Request/Response Types

These types are the translation boundary. The provider adapter converts between these and the backend's native format.

```go
// ProviderRequest is the backend-facing request.
// It contains only the information the provider needs,
// stripped of transport and storage concerns.
type ProviderRequest struct {
    Model       string
    Messages    []ProviderMessage
    Tools       []ProviderTool
    ToolChoice  api.ToolChoice
    Temperature *float64
    TopP        *float64
    MaxTokens   *int
    Stop        []string

    // Provider-specific parameters that don't map to standard fields.
    Extra map[string]any
}

// ProviderMessage represents a message in the provider's conversation format.
type ProviderMessage struct {
    Role       string          // "system", "user", "assistant", "tool"
    Content    any             // string or []ContentPart
    ToolCalls  []ProviderToolCall `json:"tool_calls,omitempty"`
    ToolCallID string          `json:"tool_call_id,omitempty"`
    Name       string          `json:"name,omitempty"`
}

// ProviderResponse is the backend's complete response.
type ProviderResponse struct {
    Items []api.Item
    Usage api.Usage
    Model string // actual model used (may differ from requested)
}

// ProviderEvent is a single streaming event from the backend.
type ProviderEvent struct {
    // Type indicates what kind of event this is.
    Type ProviderEventType

    // Delta contains incremental text or argument data.
    Delta string

    // Item is populated for item-level events (added, done).
    Item *api.Item

    // Usage is populated on the final event.
    Usage *api.Usage

    // Err is populated if the stream encountered an error.
    Err error
}

type ProviderEventType int

const (
    ProviderEventTextDelta ProviderEventType = iota
    ProviderEventTextDone
    ProviderEventToolCallDelta
    ProviderEventToolCallDone
    ProviderEventReasoningDelta
    ProviderEventReasoningDone
    ProviderEventDone
    ProviderEventError
)
```

## Request Translation

The core engine converts OpenResponses types to provider types:

```go
// Translator converts between OpenResponses and provider formats.
// Each provider adapter embeds a Translator or implements custom translation.
type Translator interface {
    // TranslateRequest converts an OpenResponses request to a provider request.
    TranslateRequest(ctx context.Context, req *api.CreateResponseRequest) (*ProviderRequest, error)

    // TranslateResponse converts a provider response to OpenResponses items.
    TranslateResponse(ctx context.Context, resp *ProviderResponse) ([]api.Item, *api.Usage, error)

    // TranslateStreamEvent converts a single provider event to OpenResponses stream events.
    // One provider event may produce zero or more OpenResponses events.
    TranslateStreamEvent(ctx context.Context, event ProviderEvent) ([]transport.StreamEvent, error)
}
```

## vLLM Adapter

vLLM exposes an OpenAI-compatible API, so translation is thin:

```go
// VLLMProvider connects to a vLLM instance via its OpenAI-compatible API.
type VLLMProvider struct {
    client   *http.Client
    baseURL  string
    apiKey   string // optional, for vLLM auth
    caps     ProviderCapabilities
}

// Key translation mappings:
//
// OpenResponses              -> vLLM (OpenAI format)
// ---------------------------------------------------------
// CreateResponseRequest      -> ChatCompletionRequest
// Item(message, user)        -> ChatCompletionMessage(role=user)
// Item(message, assistant)   -> ChatCompletionMessage(role=assistant)
// Item(function_call)        -> tool_calls in assistant message
// Item(reasoning)            -> (dropped if not supported, or mapped to reasoning_content)
// Tool                       -> ChatCompletionTool (function type)
// ToolChoice                 -> tool_choice (direct mapping)
// stream=true                -> stream=true, stream_options.include_usage=true
//
// vLLM (OpenAI format)       -> OpenResponses
// ---------------------------------------------------------
// ChatCompletionResponse     -> Response with Output items
// choice.message.content     -> Item(message, assistant) with ModelContent
// choice.message.tool_calls  -> Item(function_call) per tool call
// SSE chunk delta            -> ProviderEvent(TextDelta/ToolCallDelta)
// usage                      -> Usage
```

### Streaming Event Mapping

When the backend speaks Chat Completions, the adapter must translate SSE chunks (`chat.completion.chunk`) into Responses API streaming events. The following table shows the concrete mapping:

| Chat Completions SSE chunk | Responses API event |
|---|---|
| First chunk with `role` field | `response.output_item.added` + `response.content_part.added` |
| `delta.content` (text fragment) | `response.output_text.delta` |
| `delta.tool_calls` (function call fragment) | `response.function_call_arguments.delta` |
| Final chunk with `finish_reason` set | `response.output_text.done` + `response.output_item.done` |
| Stream end (`[DONE]` sentinel) | `response.completed` + `[DONE]` |

One Chat Completions chunk may produce zero, one, or multiple Responses API events. For example, the first chunk typically produces two events (item added and content part added), while a mid-stream text delta produces exactly one.

#### finish_reason Mapping

The Chat Completions `finish_reason` field maps to the Responses API response status as follows:

| Chat Completions `finish_reason` | Responses API status | Notes |
|---|---|---|
| `stop` | `completed` | Normal completion |
| `length` | `incomplete` | Output truncated due to `max_tokens` |
| `tool_calls` | `completed` (with function call items) | The caller should continue the agentic loop by executing the tool calls and feeding results back |

When `finish_reason` is `tool_calls`, the response itself is marked `completed`, but the presence of `function_call` items in the output signals the orchestration layer to continue the agentic loop.

### Conversation History Reconstruction

When `previous_response_id` is set and the backend only speaks Chat Completions, the provider adapter cannot forward a response ID to the backend. Instead, antwort must reconstruct the full conversation history from stored responses. The flow is:

1. Load the response chain by following `previous_response_id` links in storage
2. Extract the stored conversation messages from each response in the chain
3. Build an ordered messages array: system message (from `instructions`), then alternating user, assistant, tool call, and tool result messages
4. Append the current request's input items to the end of the messages array
5. Send the complete conversation to the Chat Completions backend as a flat messages list

This means the Chat Completions adapter receives a fully reconstructed `ProviderRequest` with all messages populated. The adapter itself does not need to know about response chaining or storage. The core engine handles the reconstruction before calling `Complete()` or `Stream()`.

### vLLM-Specific Considerations

- **Model routing**: vLLM serves a single model per instance (or uses `--served-model-name`). The adapter validates the requested model against `ListModels()`.
- **Reasoning**: vLLM with DeepSeek R1 or similar models may produce reasoning tokens. The adapter detects and maps these to `ReasoningItem`.
- **Context window**: Determined by the loaded model. The adapter queries model info on startup.
- **Guided decoding**: vLLM supports JSON schema guided decoding. Exposed via `Extra` parameters.

### Streaming Edge Cases

These edge cases apply to any Chat Completions backend (including vLLM) and must be handled by the adapter's stream translation logic:

- **Incremental tool call arguments**: Tool call arguments arrive as incremental JSON fragments across multiple SSE chunks. The adapter must buffer these fragments and only emit `ProviderEventToolCallDone` once the full JSON string has been assembled.
- **Tool call ID correlation**: When the model produces function call results, each result must be correlated back to the originating tool call via `tool_call_id`. The adapter tracks active tool call IDs during streaming so that downstream tool execution can match results correctly.
- **Multi-modal content encoding**: Image and other multi-modal inputs require different encoding depending on the backend. vLLM expects base64-encoded images inline, while other backends may expect URLs or different content part structures. The adapter normalizes these differences during request translation.

### Configuration

```go
type VLLMConfig struct {
    // BaseURL is the vLLM server URL (e.g., "http://localhost:8000").
    BaseURL string `json:"base_url" env:"ANTWORT_VLLM_URL"`

    // APIKey for vLLM authentication (optional).
    APIKey string `json:"api_key" env:"ANTWORT_VLLM_API_KEY"`

    // Timeout for individual requests.
    Timeout time.Duration `json:"timeout" env:"ANTWORT_VLLM_TIMEOUT"`

    // MaxRetries for transient failures.
    MaxRetries int `json:"max_retries" env:"ANTWORT_VLLM_MAX_RETRIES"`
}
```

## Extension Points

- **New providers**: Implement the `Provider` interface. No changes to core engine required.
- **Custom translation**: Override `Translator` methods for providers with non-standard API formats.
- **Capability-based routing**: The `ProviderCapabilities` struct enables future multi-provider routing where requests are directed to the most capable backend.
- **Provider-specific parameters**: The `Extra` map in `ProviderRequest` and `Extensions` in the API types allow passing provider-specific options without modifying the interface.

## Open Questions

- Should provider health checking be part of the interface (e.g., `HealthCheck() error`)?
- How to handle model aliases (user requests "gpt-4" but vLLM serves it as a different name)?
- Should the `Translator` be a separate interface or methods on `Provider`?
- Connection pooling strategy for high-throughput scenarios?

## Deliverables

- [ ] `pkg/provider/provider.go` - Provider and Translator interfaces
- [ ] `pkg/provider/types.go` - ProviderRequest, ProviderResponse, ProviderEvent
- [ ] `pkg/provider/capabilities.go` - Capability negotiation logic
- [ ] `pkg/provider/vllm/vllm.go` - vLLM adapter implementation
- [ ] `pkg/provider/vllm/translate.go` - Request/response translation
- [ ] `pkg/provider/vllm/config.go` - Configuration
- [ ] `pkg/provider/vllm/vllm_test.go` - Tests with mock vLLM server
