# Chat Completions Translation Mapping

**Feature**: 003-core-engine
**Date**: 2026-02-17

This document defines the exact translation rules between OpenResponses types and Chat Completions API types used by the vLLM adapter.

## Request Translation (OpenResponses -> Chat Completions)

### Item-to-Message Mapping

| OpenResponses Input Item | Chat Completions Message | Notes |
|--------------------------|--------------------------|-------|
| `instructions` field (CreateResponseRequest) | `{role: "system", content: "..."}` | Always first message. Only most recent instructions used in chain reconstruction. |
| Item type=`message`, role=`user`, text-only | `{role: "user", content: "text"}` | Content as plain string |
| Item type=`message`, role=`user`, multimodal | `{role: "user", content: [{type: "text", text: "..."}, {type: "image_url", image_url: {url: "..."}}]}` | Content as array of parts |
| Item type=`message`, role=`assistant` | `{role: "assistant", content: "text"}` | Text from OutputContentPart |
| Item type=`message`, role=`system` | `{role: "system", content: "text"}` | Text from ContentPart |
| Item type=`function_call` | `{role: "assistant", tool_calls: [{id: "call_id", type: "function", function: {name: "name", arguments: "json"}}]}` | Maps to assistant message with tool_calls array |
| Item type=`function_call_output` | `{role: "tool", tool_call_id: "call_id", content: "output"}` | Maps to tool-role message |
| Item type=`reasoning` | (skipped) | Reasoning is model-generated, not sent to backend |

### Multimodal Content Encoding

| ContentPart Type | Chat Completions Format |
|-----------------|------------------------|
| `input_text` | `{type: "text", text: "..."}` |
| `input_image` with URL | `{type: "image_url", image_url: {url: "https://..."}}` |
| `input_image` with base64 data | `{type: "image_url", image_url: {url: "data:<media_type>;base64,<data>"}}` |
| `input_audio` | Not supported by Chat Completions standard; rejected by capability check |
| `input_video` | Not supported by Chat Completions standard; rejected by capability check |

### Parameter Mapping

| OpenResponses Parameter | Chat Completions Parameter | Notes |
|------------------------|---------------------------|-------|
| `model` | `model` | Direct mapping |
| `temperature` | `temperature` | Direct mapping (omit if nil) |
| `top_p` | `top_p` | Direct mapping (omit if nil) |
| `max_output_tokens` | `max_tokens` | Name change (omit if nil) |
| `tools` | `tools` | Array of tool definitions (see below) |
| `tool_choice` | `tool_choice` | Direct mapping: "auto", "required", "none", or structured |
| `stream` = true | `stream` = true + `stream_options: {include_usage: true}` | Always request usage in stream |
| `stream` = false | `stream` = false | No stream_options needed |
| (always) | `n` = 1 | Always set; only first choice used |

### Tool Definition Mapping

| OpenResponses ToolDefinition | Chat Completions Tool |
|-----------------------------|-----------------------|
| `{type: "function", name: "X", description: "Y", parameters: {...}}` | `{type: "function", function: {name: "X", description: "Y", parameters: {...}}}` | Note the nested `function` object in Chat Completions |

### Consecutive Same-Role Messages

Consecutive input Items with the same role are preserved as separate messages. They are NOT merged. The backend may interpret multiple consecutive user messages differently than a single merged message.

## Response Translation (Chat Completions -> OpenResponses)

### Non-Streaming Response

| Chat Completions Field | OpenResponses Field | Notes |
|-----------------------|---------------------|-------|
| `choices[0].message.content` | Item type=`message`, role=`assistant`, output=[OutputContentPart type=`output_text`] | Only choices[0] used |
| `choices[0].message.tool_calls[i]` | Item type=`function_call`, name=tc.function.name, call_id=tc.id, arguments=tc.function.arguments | One Item per tool call |
| `choices[0].message.reasoning_content` | Item type=`reasoning`, content=value | Provider-specific (DeepSeek R1) |
| `choices[0].finish_reason` | Response status | See finish_reason mapping below |
| `usage.prompt_tokens` | Usage.input_tokens | Name change |
| `usage.completion_tokens` | Usage.output_tokens | Name change |
| `usage.total_tokens` | Usage.total_tokens | Direct mapping |
| `model` | Response.model | Actual model used (may differ from requested) |

### finish_reason Mapping

| Chat Completions finish_reason | OpenResponses Response Status | Notes |
|-------------------------------|-------------------------------|-------|
| `stop` | `completed` | Normal completion |
| `length` | `incomplete` | Output truncated due to max_tokens |
| `tool_calls` | `completed` | Function call items in output signal tool use |
| `content_filter` | `failed` | Content filtered by safety system |
| (unknown value) | `completed` | Log warning, treat as normal completion |
| (null/missing) | `in_progress` | Stream still in progress |

### Streaming Event Mapping

| Chat Completions SSE Chunk | ProviderEvent | Engine Action |
|---------------------------|---------------|---------------|
| First chunk with `role` field | ProviderEventTextDelta (empty delta, signals new message) | Emit `response.output_item.added` + `response.content_part.added` |
| `delta.content` (text fragment) | ProviderEventTextDelta | Emit `response.output_text.delta` |
| `delta.tool_calls[i].function.name` | ProviderEventToolCallDelta (first fragment for index i) | Emit `response.output_item.added` (function_call type) |
| `delta.tool_calls[i].function.arguments` | ProviderEventToolCallDelta | Emit `response.function_call_arguments.delta` |
| `delta.reasoning_content` | ProviderEventReasoningDelta | Emit reasoning-related events |
| Chunk with `finish_reason` set | ProviderEventTextDone or ProviderEventToolCallDone | Emit `response.output_text.done` + `response.content_part.done` + `response.output_item.done` |
| `data: [DONE]` sentinel | ProviderEventDone | Emit `response.completed` (or `response.failed` if error) |
| Usage in final chunk | (included in ProviderEventDone) | Populate Response.Usage |

### Streaming Lifecycle Events (Engine-Generated)

The engine generates these events; they are NOT produced by the Chat Completions backend:

| Event | When Generated | Payload |
|-------|----------------|---------|
| `response.created` | Before calling provider | Response snapshot (status: in_progress) |
| `response.in_progress` | Before calling provider | Response snapshot |
| `response.output_item.added` | First delta for a new output item | Item snapshot (status: in_progress) |
| `response.content_part.added` | First text delta for a new content part | Content part snapshot |
| `response.content_part.done` | After text/arguments done for a content part | Content part snapshot |
| `response.output_item.done` | After all content for an item is complete | Item snapshot (status: completed) |
| `response.completed` | After stream ends successfully | Response snapshot (status: completed/incomplete) |
| `response.failed` | On error during or after streaming | Response snapshot (status: failed, error populated) |
| `response.cancelled` | On context cancellation | Response snapshot (status: cancelled) |

## Error Mapping (HTTP Status -> APIError)

| Backend HTTP Status | APIError Type | Notes |
|--------------------|---------------|-------|
| 400 | `invalid_request` | Bad request to backend |
| 401, 403 | `server_error` | Backend auth is a server-side concern |
| 404 | `not_found` | Model not found |
| 422 | `invalid_request` | Unprocessable entity |
| 429 | `too_many_requests` | Rate limited by backend |
| 500, 502, 503, 504 | `server_error` | Backend server error |
| Connection refused | `server_error` | Backend unreachable |
| Timeout | `server_error` | Request timeout |
| DNS failure | `server_error` | Backend hostname not resolvable |
