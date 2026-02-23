# API Divergences from Upstream OpenResponses

This document tracks intentional differences between antwort's API and the
upstream [OpenResponses specification](https://github.com/openresponses/openresponses).

## Path Prefix

- **Upstream**: `/responses` (with server base URL containing `/v1`)
- **Antwort**: `/v1/responses` (full path in spec)
- **Reason**: Antwort embeds the version prefix in the path for clarity.

## Request Properties Not Supported

These upstream request properties are accepted but not actively used by antwort:

| Property | Reason |
|---|---|
| `background` | Background execution not implemented (future spec) |
| `conversation` | Not implemented (future spec) |
| `prompt_cache_key` | Passed through but not enforced |
| `prompt_cache_retention` | Passed through but not enforced |
| `safety_identifier` | Passed through but not enforced |

## Supported Request Properties (Spec 020)

These fields were added in Spec 020 and are fully supported:

| Property | Behavior |
|---|---|
| `metadata` | Accepted and echoed in response |
| `user` | Accepted and echoed in response |
| `frequency_penalty` | Forwarded to inference provider |
| `presence_penalty` | Forwarded to inference provider |
| `top_logprobs` | Forwarded to inference provider |
| `reasoning` | Forwarded to inference provider |
| `text` | Forwarded to inference provider |
| `parallel_tool_calls` | Controls concurrent vs sequential tool dispatch |
| `max_tool_calls` | Limits agentic loop iterations |
| `include` | Controls response verbosity (type added, filtering P2) |
| `stream_options` | Controls streaming behavior (type added, usage P2) |

## Response Properties Not Returned

These upstream response properties are not emitted by antwort:

| Property | Reason |
|---|---|
| `billing` | Not applicable |
| `context_edits` | Not implemented |
| `conversation` | Not implemented (future spec) |
| `cost_token` | Not applicable |
| `next_response_ids` | Not implemented |

## Nullable Handling

The upstream spec uses `anyOf` with `null` for nullable fields. Antwort's
OpenAPI spec uses the OpenAPI 3.1 `type: ["string", "null"]` syntax or
`oneOf` with `type: "null"` for the same purpose. The wire format is identical.

## Side-APIs (Not in Upstream)

These endpoints exist only in antwort and are not part of the OpenResponses spec:

- `POST/GET /builtin/file_search/vector_stores` (vector store management)
- `GET/DELETE /builtin/file_search/vector_stores/{store_id}`
- `GET /healthz` (health check)
- `GET /readyz` (readiness check)
- `GET /metrics` (Prometheus metrics)
