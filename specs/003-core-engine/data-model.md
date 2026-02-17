# Data Model: Core Engine & Provider Abstraction

**Feature**: 003-core-engine
**Date**: 2026-02-17

## Entity Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         Engine                               │
│  Implements transport.ResponseCreator                        │
│  provider: Provider (required)                               │
│  store: transport.ResponseStore (nil-safe, optional)         │
│  config: EngineConfig                                        │
└──────────┬───────────────────────────────┬──────────────────┘
           │ calls                         │ uses (optional)
           ▼                               ▼
┌──────────────────────┐    ┌────────────────────────────────┐
│      Provider        │    │    transport.ResponseStore      │
│  (interface)         │    │    GetResponse / DeleteResponse  │
│                      │    └────────────────────────────────┘
│  Name()              │
│  Capabilities()      │
│  Complete()          │    ┌────────────────────────────────┐
│  Stream()  ──────────┼──> │   <-chan ProviderEvent          │
│  ListModels()        │    └────────────────────────────────┘
│  Close()             │
└──────────┬───────────┘
           │ implemented by
           ▼
┌──────────────────────┐
│   VLLMProvider       │
│   (Chat Completions) │
│                      │
│  http.Client         │
│  baseURL, apiKey     │
│  config: VLLMConfig  │
└──────────────────────┘
```

## Amended Entity (Spec 001)

### Response (Amendment)

Added field to support conversation history reconstruction:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| input | []Item | No | Input Items from the originating request. Populated when `store: true`. |

All other Response fields remain unchanged from Spec 001.

## New Entities

### Provider (Interface)

Protocol-agnostic abstraction over an LLM inference backend.

**Operations**:
| Operation | Input | Output | Description |
|-----------|-------|--------|-------------|
| Name | (none) | string | Provider identifier (e.g., "vllm", "litellm") |
| Capabilities | (none) | ProviderCapabilities | Declare supported features |
| Complete | context, *ProviderRequest | *ProviderResponse, error | Non-streaming inference |
| Stream | context, *ProviderRequest | <-chan ProviderEvent, error | Streaming inference |
| ListModels | context | []ModelInfo, error | Available models from backend |
| Close | (none) | error | Release resources (HTTP clients, connections) |

**Constraints**:
- Thread-safe: multiple goroutines may call Complete/Stream concurrently
- Stream channel MUST be closed by the provider when stream completes or errors
- Context cancellation MUST stop processing and return promptly
- Errors returned MUST be `*api.APIError` or wrapped errors containing `*api.APIError`

### ProviderCapabilities

Declaration of what features a provider instance supports.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| Streaming | bool | false | Supports streaming responses |
| ToolCalling | bool | false | Supports function/tool calls |
| Vision | bool | false | Supports image inputs |
| Audio | bool | false | Supports audio inputs |
| Reasoning | bool | false | Can produce reasoning items |
| MaxContextWindow | int | 0 | Maximum token count (0 = unknown) |
| SupportedModels | []string | nil | Models this provider serves (empty = ask ListModels) |
| Extensions | []string | nil | Provider-specific extension types supported |

### ProviderRequest

Backend-facing request type, stripped of transport and storage concerns.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Model | string | Yes | Model identifier |
| Messages | []ProviderMessage | Yes | Conversation messages |
| Tools | []ProviderTool | No | Tool definitions |
| ToolChoice | api.ToolChoice | No | Tool selection strategy |
| Temperature | *float64 | No | Sampling temperature |
| TopP | *float64 | No | Nucleus sampling |
| MaxTokens | *int | No | Output token limit |
| Stop | []string | No | Stop sequences |
| Stream | bool | No | Whether to stream |
| Extra | map[string]any | No | Provider-specific parameters |

### ProviderMessage

A message in the provider's conversation format.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Role | string | Yes | "system", "user", "assistant", "tool" |
| Content | any | Conditional | string or []ContentPart (multimodal) |
| ToolCalls | []ProviderToolCall | No | Tool calls (assistant messages) |
| ToolCallID | string | No | Originating call ID (tool messages) |
| Name | string | No | Function name (tool messages) |

### ProviderToolCall

A tool call entry in an assistant message.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ID | string | Yes | Tool call identifier |
| Type | string | Yes | Always "function" |
| Function | ProviderFunctionCall | Yes | Function name and arguments |

### ProviderFunctionCall

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Name | string | Yes | Function name |
| Arguments | string | Yes | JSON-encoded arguments |

### ProviderTool

Tool definition in provider format.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Type | string | Yes | Tool type (e.g., "function") |
| Function | ProviderFunctionDef | Yes | Function definition |

### ProviderFunctionDef

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Name | string | Yes | Function name |
| Description | string | No | Human-readable description |
| Parameters | json.RawMessage | No | JSON Schema for arguments |

### ProviderResponse

Complete non-streaming response from the backend.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Items | []api.Item | Yes | Output items (already translated) |
| Usage | api.Usage | Yes | Token usage statistics |
| Model | string | Yes | Actual model used |
| Status | api.ResponseStatus | Yes | completed, incomplete, or failed |

### ProviderEvent

Single streaming event from the backend.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Type | ProviderEventType | Yes | Event classification |
| Delta | string | Conditional | Incremental text or argument data |
| ToolCallIndex | int | Conditional | Which tool call (for tool events) |
| ToolCallID | string | Conditional | Tool call identifier |
| FunctionName | string | Conditional | Function name (first tool call event) |
| Item | *api.Item | Conditional | Complete item (for done events) |
| Usage | *api.Usage | Conditional | Usage stats (final event) |
| Err | error | Conditional | Error (for error events) |

### ProviderEventType

| Value | Description |
|-------|-------------|
| ProviderEventTextDelta | Incremental text content |
| ProviderEventTextDone | Text content complete |
| ProviderEventToolCallDelta | Incremental tool call arguments |
| ProviderEventToolCallDone | Tool call complete with full arguments |
| ProviderEventReasoningDelta | Incremental reasoning content |
| ProviderEventReasoningDone | Reasoning content complete |
| ProviderEventDone | Stream finished successfully |
| ProviderEventError | Stream error |

### ModelInfo

Information about a model served by the provider.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| ID | string | Yes | Model identifier |
| Object | string | No | Object type (e.g., "model") |
| OwnedBy | string | No | Owner identifier |

### EngineConfig

Configuration for the core engine.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| DefaultModel | string | "" | Default model when request omits model field (empty = require explicit model) |

### VLLMConfig

Configuration for the vLLM adapter.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| BaseURL | string | required | Backend URL (e.g., "http://localhost:8000") |
| APIKey | string | "" | Optional API key for backend auth |
| Timeout | time.Duration | 120s | Per-request timeout |
| MaxRetries | int | 0 | Maximum retry count for failed requests |

### Translator (Interface)

Request translation logic, implemented per provider adapter.

**Operations**:
| Operation | Input | Output | Description |
|-----------|-------|--------|-------------|
| TranslateRequest | context, *api.CreateResponseRequest | *ProviderRequest, error | Convert OpenResponses request to provider format |
| TranslateResponse | context, *ProviderResponse | []api.Item, *api.Usage, error | Convert provider response to OpenResponses items |

**Constraints**:
- Translator does NOT handle stream event mapping (that's the engine's job)
- Each adapter may embed its own Translator or implement translation inline

## Relationships

```
Engine
  ├── implements transport.ResponseCreator
  ├── requires Provider (1:1, must not be nil)
  ├── optionally uses transport.ResponseStore (0..1, nil-safe)
  ├── uses Translator (embedded in provider or standalone)
  └── writes to transport.ResponseWriter (provided per request)

Provider (interface)
  ├── implemented by VLLMProvider
  ├── returns ProviderResponse (non-streaming)
  └── returns <-chan ProviderEvent (streaming)

VLLMProvider
  ├── owns http.Client
  ├── owns VLLMConfig
  ├── translates ProviderRequest -> Chat Completions HTTP request
  └── translates Chat Completions HTTP response -> ProviderResponse / ProviderEvent

ProviderRequest
  ├── contains []ProviderMessage (conversation)
  ├── contains []ProviderTool (tool definitions)
  └── references api.ToolChoice (tool selection)

ProviderMessage
  ├── content is string (text) or []api.ContentPart (multimodal)
  └── optionally contains []ProviderToolCall (assistant tool calls)
```

## Reused Types from Spec 001 (pkg/api)

- `api.Item` - Input and output items (polymorphic)
- `api.CreateResponseRequest` - Incoming request from transport
- `api.Response` - Outgoing response (AMENDED: now includes `Input []Item`)
- `api.StreamEvent` - Streaming event (engine writes these)
- `api.StreamEventType` - Event type constants
- `api.APIError` - Structured error type
- `api.Usage` - Token usage statistics
- `api.ToolChoice` - Tool selection strategy
- `api.ToolDefinition` - Tool definition from request
- `api.ContentPart` - Multimodal content part
- `api.OutputContentPart` - Model output content
- `api.ResponseStatus` - Response lifecycle status
- `api.ItemStatus` - Item lifecycle status

## Reused Types from Spec 002 (pkg/transport)

- `transport.ResponseCreator` - Handler interface (Engine implements this)
- `transport.ResponseStore` - Storage interface (Engine optionally uses this)
- `transport.ResponseWriter` - Output abstraction (Engine writes to this)
