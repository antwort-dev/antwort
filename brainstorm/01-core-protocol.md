# Spec 01: Core Protocol & Data Model

**Branch**: `spec/01-core-protocol`
**Dependencies**: None (foundation spec)
**Package**: `github.com/rhuss/antwort/pkg/api`

## Purpose

Define the core data types, state machines, and request/response schemas that implement the OpenResponses specification. This spec is the foundation that all other specs depend on.

## Scope

### In Scope
- All OpenResponses data types as Go structs with JSON serialization
- Item type hierarchy (message, function_call, reasoning, provider extensions)
- Content models (user input: text/image/audio/video, model output: text)
- State machine semantics for Response and Item objects
- Error types and structured error objects
- Request parameters (truncation, service_tier, store, previous_response_id)
- Stateless vs stateful API tier classification
- Extension point definitions (provider-prefixed custom types)

### Out of Scope
- Transport (HTTP, gRPC, SSE) - see Spec 02
- Provider-specific translation - see Specs 03, 08
- Tool execution logic - see Spec 04
- Persistence - see Spec 05

## Data Model

### Items

Items are the fundamental unit. All items share a common base:

```go
// ItemType discriminates the polymorphic Item.
type ItemType string

const (
    ItemTypeMessage      ItemType = "message"
    ItemTypeFunctionCall ItemType = "function_call"
    ItemTypeReasoning    ItemType = "reasoning"
    // Provider extensions use "provider:type" format.
)

// ItemStatus represents lifecycle state.
type ItemStatus string

const (
    ItemStatusInProgress ItemStatus = "in_progress"
    ItemStatusIncomplete ItemStatus = "incomplete"
    ItemStatusCompleted  ItemStatus = "completed"
)

// Item is the core polymorphic unit of context.
// The Type field determines which concrete fields are populated.
type Item struct {
    ID     string     `json:"id"`
    Type   ItemType   `json:"type"`
    Status ItemStatus `json:"status"`

    // Populated when Type == "message"
    Message *MessageItem `json:"message,omitempty"`

    // Populated when Type == "function_call"
    FunctionCall *FunctionCallItem `json:"function_call,omitempty"`

    // Populated when Type == "reasoning"
    Reasoning *ReasoningItem `json:"reasoning,omitempty"`

    // Extension data for provider-prefixed types.
    Extension json.RawMessage `json:"extension,omitempty"`
}
```

### Content Models

User and model content are asymmetric per the spec:

```go
// UserContent supports multimodal input.
type UserContent struct {
    Type  string `json:"type"` // "input_text", "input_image", "input_audio", "input_video"
    Text  string `json:"text,omitempty"`
    URL   string `json:"url,omitempty"`
    Data  string `json:"data,omitempty"`   // base64
    Media string `json:"media,omitempty"` // MIME type
}

// ModelContent is text-only in the base protocol.
type ModelContent struct {
    Type string `json:"type"` // "output_text"
    Text string `json:"text"`
    // Optional: token logprobs
    Logprobs []TokenLogprob `json:"logprobs,omitempty"`
}
```

### Message, FunctionCall, Reasoning Items

```go
type MessageRole string

const (
    RoleUser      MessageRole = "user"
    RoleAssistant MessageRole = "assistant"
    RoleSystem    MessageRole = "system"
)

type MessageItem struct {
    Role    MessageRole    `json:"role"`
    Content []UserContent  `json:"content,omitempty"`  // for user/system
    Output  []ModelContent `json:"output,omitempty"`   // for assistant
}

type FunctionCallItem struct {
    Name      string `json:"name"`
    CallID    string `json:"call_id"`
    Arguments string `json:"arguments"` // JSON string
}

type ReasoningItem struct {
    Content          string `json:"content,omitempty"`
    EncryptedContent string `json:"encrypted_content,omitempty"`
    Summary          string `json:"summary,omitempty"`
}
```

### Request / Response

```go
// CreateResponseRequest is the primary API input.
type CreateResponseRequest struct {
    Model              string      `json:"model"`
    Input              []Item      `json:"input"`
    Instructions       string      `json:"instructions,omitempty"`
    Tools              []Tool      `json:"tools,omitempty"`
    ToolChoice         ToolChoice  `json:"tool_choice,omitempty"`
    AllowedTools       []string    `json:"allowed_tools,omitempty"`
    Store              *bool       `json:"store,omitempty"`           // nil = true (default)
    Stream             bool        `json:"stream,omitempty"`
    PreviousResponseID string      `json:"previous_response_id,omitempty"`
    Truncation         string      `json:"truncation,omitempty"`     // "auto" | "disabled"
    ServiceTier        string      `json:"service_tier,omitempty"`
    MaxOutputTokens    *int        `json:"max_output_tokens,omitempty"`
    Temperature        *float64    `json:"temperature,omitempty"`
    TopP               *float64    `json:"top_p,omitempty"`

    // Extension fields for provider-specific parameters.
    Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

// Response is the primary API output.
type Response struct {
    ID                 string       `json:"id"`
    Object             string       `json:"object"` // "response"
    Status             ResponseStatus `json:"status"`
    Output             []Item       `json:"output"`
    Model              string       `json:"model"`
    Usage              *Usage       `json:"usage,omitempty"`
    Error              *APIError    `json:"error,omitempty"`
    PreviousResponseID string       `json:"previous_response_id,omitempty"`
    CreatedAt          int64        `json:"created_at"`

    Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}

type ResponseStatus string

const (
    ResponseStatusInProgress ResponseStatus = "in_progress"
    ResponseStatusCompleted  ResponseStatus = "completed"
    ResponseStatusFailed     ResponseStatus = "failed"
    ResponseStatusCancelled  ResponseStatus = "cancelled"
)

type Usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
    TotalTokens  int `json:"total_tokens"`
}
```

### Errors

```go
type ErrorType string

const (
    ErrorTypeServer        ErrorType = "server_error"
    ErrorTypeInvalidRequest ErrorType = "invalid_request"
    ErrorTypeNotFound      ErrorType = "not_found"
    ErrorTypeModelError    ErrorType = "model_error"
    ErrorTypeTooManyRequests ErrorType = "too_many_requests"
)

type APIError struct {
    Type    ErrorType `json:"type"`
    Code    string    `json:"code,omitempty"`
    Param   string    `json:"param,omitempty"`
    Message string    `json:"message"`
}
```

### State Machine

```
Response:  in_progress ──> completed
                       ──> failed
                       ──> cancelled

Item:      in_progress ──> completed
                       ──> incomplete (token budget exhausted)
```

No mutations are allowed after a terminal state.

## Stateless vs Stateful Classification

The `store` field in the request determines the tier:

| Feature | Stateless (`store: false`) | Stateful (`store: true`, default) |
|---|---|---|
| `POST /v1/responses` | Yes | Yes |
| `GET /v1/responses/{id}` | No | Yes |
| `DELETE /v1/responses/{id}` | No | Yes |
| `previous_response_id` | No | Yes |
| Persistence required | No | Yes (PostgreSQL) |
| Suitable for lightweight deploy | Yes | No |

## Extension Points

The spec defines two extension mechanisms:

1. **Custom item types**: Prefixed with provider slug (e.g., `"acme:telemetry_chunk"`)
2. **Custom fields**: Added to existing schemas via `Extensions` map

Both the `Item.Extension` field and `Request/Response.Extensions` maps use `json.RawMessage` to allow provider-specific data without modifying core types.

## Validation Rules

- `model` is required on all requests
- `input` must contain at least one item
- `ItemType` must be a known type or match `provider:type` pattern
- `previous_response_id` is only valid when `store` is not explicitly `false`
- `tool_choice` values: `"auto"`, `"required"`, `"none"`, or `{"type":"function","name":"..."}`

## Open Questions

- Should we use a sum type pattern (interface with sealed methods) instead of the flat struct with optional fields for Item polymorphism?
- How to handle OpenResponses spec versioning in the data model?
- Should `Extensions` be a first-class interface rather than raw JSON?

## Deliverables

- [ ] `pkg/api/types.go` - All data types
- [ ] `pkg/api/validation.go` - Request validation
- [ ] `pkg/api/state.go` - State machine transition logic
- [ ] `pkg/api/errors.go` - Error constructors and helpers
- [ ] `pkg/api/types_test.go` - JSON round-trip tests
- [ ] `pkg/api/validation_test.go` - Validation tests
