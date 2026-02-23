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
| `background` | Background execution not implemented |
| `conversation` | Not implemented |
| `include` | Not implemented |
| `parallel_tool_calls` | Always enabled |
| `prompt_cache_key` | Not implemented |
| `prompt_cache_retention` | Not implemented |
| `reasoning` | Passed through to provider |
| `safety_identifier` | Not implemented |
| `stream_options` | Not implemented |
| `text` | Passed through to provider |
| `top_logprobs` | Passed through to provider |
| `user` | Not tracked |

## Response Properties Not Returned

These upstream response properties are not emitted by antwort:

| Property | Reason |
|---|---|
| `billing` | Not applicable |
| `context_edits` | Not implemented |
| `conversation` | Not implemented |
| `cost_token` | Not applicable |
| `next_response_ids` | Not implemented |
| `user` | Not tracked |

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
