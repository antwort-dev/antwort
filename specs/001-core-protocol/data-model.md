# Data Model: Core Protocol & Data Model

**Feature**: 001-core-protocol
**Date**: 2026-02-16

## Entity Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    CreateResponseRequest                     │
│  model, input[]Item, instructions?, tools[], tool_choice,   │
│  store?, stream?, previous_response_id?, truncation?,       │
│  service_tier?, max_output_tokens?, temperature?, top_p?,   │
│  extensions?                                                 │
└──────────────────────────┬──────────────────────────────────┘
                           │ produces
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                         Response                             │
│  id(resp_*), object="response", status, output[]Item,       │
│  model, usage?, error?, previous_response_id?,              │
│  created_at, extensions?                                     │
│                                                              │
│  State Machine: queued -> in_progress -> completed|failed|   │
│                                         cancelled            │
└──────────────────────────┬──────────────────────────────────┘
                           │ contains
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                          Item                                │
│  id(item_*), type, status                                    │
│                                                              │
│  Discriminated union on `type`:                              │
│  ├── "message"              → MessageData                    │
│  ├── "function_call"        → FunctionCallData               │
│  ├── "function_call_output" → FunctionCallOutputData         │
│  ├── "reasoning"            → ReasoningData                  │
│  └── "provider:*"           → Extension (opaque JSON)        │
│                                                              │
│  State Machine: in_progress -> completed|incomplete|failed   │
└─────────────────────────────────────────────────────────────┘
```

## Entities

### Item

The atomic unit of context. Polymorphic via `type` discriminator.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Prefixed: `item_` + random alphanumeric |
| type | ItemType | Yes | Discriminator: `message`, `function_call`, `function_call_output`, `reasoning`, or `provider:*` |
| status | ItemStatus | Yes | `in_progress`, `incomplete`, `completed`, `failed` |
| message | *MessageData | Conditional | Present when type = `message` |
| function_call | *FunctionCallData | Conditional | Present when type = `function_call` |
| function_call_output | *FunctionCallOutputData | Conditional | Present when type = `function_call_output` |
| reasoning | *ReasoningData | Conditional | Present when type = `reasoning` |
| extension | raw JSON | Conditional | Present when type matches `provider:*` |

**Validation rules**:
- Exactly one type-specific field must be populated, matching the `type` value
- `id` must be non-empty and match `item_[a-zA-Z0-9]+` pattern
- `status` must be a valid ItemStatus value
- Terminal states (`completed`, `incomplete`, `failed`) are immutable

**State transitions**:
- `in_progress` -> `completed` (normal completion)
- `in_progress` -> `incomplete` (token budget exhausted)
- `in_progress` -> `failed` (error during generation)
- No transitions from terminal states

### MessageData

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| role | MessageRole | Yes | `user`, `assistant`, `system` |
| content | []ContentPart | Conditional | User/system input content (multimodal) |
| output | []OutputContentPart | Conditional | Assistant output content (text) |

**Validation rules**:
- `role` must be `user`, `assistant`, or `system`
- User/system messages: `content` required, `output` empty
- Assistant messages: `output` required, `content` empty

### FunctionCallData

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | Yes | Function name |
| call_id | string | Yes | Unique call identifier |
| arguments | string | Yes | JSON-encoded arguments string |

**Validation rules**:
- All fields required and non-empty
- `arguments` must be valid JSON (string-encoded)

### FunctionCallOutputData

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| call_id | string | Yes | Matches originating function_call's call_id |
| output | string | Yes | Tool execution result |

**Validation rules**:
- Both fields required and non-empty

### ReasoningData

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| content | string | No | Raw reasoning trace |
| encrypted_content | string | No | Provider-opaque protected reasoning |
| summary | string | No | Safe user-facing explanation |

**Validation rules**:
- All fields optional
- At least one field should be present (warning, not error)

### ContentPart (User Input)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| type | string | Yes | `input_text`, `input_image`, `input_audio`, `input_video` |
| text | string | Conditional | Present for `input_text` |
| url | string | Conditional | URL reference for media |
| data | string | Conditional | Base64-encoded inline data |
| media_type | string | Conditional | MIME type for media content |

### OutputContentPart (Model Output)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| type | string | Yes | `output_text` or `summary_text` |
| text | string | Yes | Generated text content |
| annotations | []Annotation | No | Inline metadata (citations, links) |
| logprobs | []TokenLogprob | No | Token-level log probabilities |

### TokenLogprob

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| token | string | Yes | The token text |
| logprob | float64 | Yes | Log probability |
| top_logprobs | []TopLogprob | No | Alternative tokens with probabilities |

### Annotation

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| type | string | Yes | Annotation type identifier |
| text | string | No | Annotated text span |
| start_index | int | No | Start position in parent text |
| end_index | int | No | End position in parent text |

### CreateResponseRequest

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| model | string | Yes | - | Model identifier |
| input | []Item | Yes | - | Input items (min 1) |
| instructions | string | No | - | System instructions |
| tools | []ToolDefinition | No | - | Available tools |
| tool_choice | ToolChoice | No | `auto` | Tool selection policy |
| allowed_tools | []string | No | - | Restricts callable tools |
| store | *bool | No | `true` | Stateful/stateless tier |
| stream | bool | No | `false` | Enable streaming |
| previous_response_id | string | No | - | Chain to previous response |
| truncation | string | No | - | `auto` or `disabled` |
| service_tier | string | No | - | Priority hint |
| max_output_tokens | *int | No | - | Output token limit |
| temperature | *float64 | No | - | Sampling temperature |
| top_p | *float64 | No | - | Nucleus sampling |
| extensions | map[string]raw JSON | No | - | Provider-specific params |

**Validation rules**:
- `model` required, non-empty
- `input` required, at least one item
- `store: false` + `previous_response_id` = error
- `max_output_tokens` must be positive if provided
- `temperature` must be 0.0-2.0 if provided
- `top_p` must be 0.0-1.0 if provided
- `truncation` must be `auto` or `disabled` if provided
- Configurable size limits on input count and content size

### ToolChoice

Union type: either a string or a structured object.

| Variant | Value | Description |
|---------|-------|-------------|
| String | `auto` | Model decides (default) |
| String | `required` | Must call at least one tool |
| String | `none` | Must not call tools |
| Object | `{type: "function", name: "..."}` | Force specific tool |

### ToolDefinition

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| type | string | Yes | Tool type (e.g., `function`) |
| name | string | Yes | Tool name |
| description | string | No | Human-readable description |
| parameters | raw JSON | No | JSON Schema for arguments |

### Response

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Prefixed: `resp_` + random alphanumeric |
| object | string | Yes | Always `"response"` |
| status | ResponseStatus | Yes | `queued`, `in_progress`, `completed`, `failed`, `cancelled` |
| output | []Item | Yes | Generated output items |
| model | string | Yes | Model that produced the response |
| usage | *Usage | No | Token usage statistics |
| error | *APIError | No | Error details (when failed) |
| previous_response_id | string | No | Reference to prior response |
| created_at | int64 | Yes | Unix timestamp |
| extensions | map[string]raw JSON | No | Provider-specific data |

**State transitions**:
- `queued` -> `in_progress` (processing starts)
- `in_progress` -> `completed` (normal completion)
- `in_progress` -> `failed` (error)
- `in_progress` -> `cancelled` (transport-driven)
- May skip `queued`, starting directly at `in_progress`

### Usage

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| input_tokens | int | Yes | Tokens in input |
| output_tokens | int | Yes | Tokens in output |
| total_tokens | int | Yes | Sum of input + output |

### APIError

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| type | ErrorType | Yes | `server_error`, `invalid_request`, `not_found`, `model_error`, `too_many_requests` |
| code | string | No | Specific error code |
| param | string | No | Related input parameter |
| message | string | Yes | Human-readable explanation |

### StreamEvent

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| type | StreamEventType | Yes | Event type identifier |
| sequence_number | int | Yes | Monotonically increasing |
| response | *Response | Conditional | For state machine events |
| item | *Item | Conditional | For item-level events |
| part | *OutputContentPart | Conditional | For content part events |
| delta | string | Conditional | For text/argument deltas |
| item_id | string | Conditional | Delta context: which item |
| output_index | int | Conditional | Delta context: which output |
| content_index | int | Conditional | Delta context: which content part |

**Delta event types**: `response.output_item.added`, `response.content_part.added`, `response.output_text.delta`, `response.output_text.done`, `response.function_call_arguments.delta`, `response.function_call_arguments.done`, `response.content_part.done`, `response.output_item.done`

**State machine event types**: `response.created`, `response.queued`, `response.in_progress`, `response.completed`, `response.failed`, `response.cancelled`

**Extension events**: `provider:event_type` pattern

### ValidationConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| max_input_items | int | 1000 | Maximum input items per request |
| max_content_size | int | 10MB | Maximum size per content part |
| max_tools | int | 128 | Maximum tools per request |

## Relationships

```
CreateResponseRequest 1──*  Item (input)
Response              1──*  Item (output)
Response              0..1──1 Response (previous_response_id chain)
Item                  1──*  ContentPart (user content, via MessageData)
Item                  1──*  OutputContentPart (model output, via MessageData)
OutputContentPart     0──*  Annotation
OutputContentPart     0──*  TokenLogprob
CreateResponseRequest 0──*  ToolDefinition
```
