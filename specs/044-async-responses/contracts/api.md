# API Contract: Async Responses (Background Mode)

**Feature**: 044-async-responses

## Request Changes

### POST /v1/responses

New field on `CreateResponseRequest`:

```json
{
  "model": "...",
  "input": [...],
  "background": true,
  "store": true
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `background` | boolean | `false` | When true, process request asynchronously |

**Validation rules**:
- `background: true` requires `store: true` (or `store` omitted, since default is true). Error: `invalid_request_error` on `background`, message: "background mode requires store to be enabled"
- `background: true` is mutually exclusive with `stream: true`. Error: `invalid_request_error` on `background`, message: "background mode cannot be used with streaming"

### Immediate Response (background: true)

When `background: true`, the server returns immediately with:

```json
{
  "id": "resp_abc123...",
  "object": "response",
  "created_at": 1709654400,
  "status": "queued",
  "background": true,
  "model": "...",
  "output": [],
  "usage": null,
  "error": null,
  ...
}
```

The response has `status: "queued"`, empty `output`, null `usage`, and null `error`.

### Completed Response (after processing)

After the worker processes the request, GET /v1/responses/{id} returns:

```json
{
  "id": "resp_abc123...",
  "object": "response",
  "created_at": 1709654400,
  "completed_at": 1709654460,
  "status": "completed",
  "background": true,
  "model": "...",
  "output": [
    {
      "type": "message",
      "role": "assistant",
      "content": [{"type": "output_text", "text": "..."}]
    }
  ],
  "usage": {
    "input_tokens": 100,
    "output_tokens": 200,
    "total_tokens": 300
  },
  "error": null,
  ...
}
```

### Failed Response

```json
{
  "id": "resp_abc123...",
  "status": "failed",
  "background": true,
  "output": [],
  "usage": null,
  "error": {
    "type": "server_error",
    "message": "background processing failed: ..."
  },
  ...
}
```

Failed responses contain only error information, no partial output.

## List Endpoint Changes

### GET /v1/responses

New query parameters:

| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by status: `queued`, `in_progress`, `completed`, `failed`, `cancelled` |
| `background` | boolean | Filter by background flag: `true` or `false` |

Example: `GET /v1/responses?status=queued&background=true`

## Delete Endpoint Changes

### DELETE /v1/responses/{id}

Existing endpoint, enhanced behavior for background responses:

| Current Status | Behavior |
|----------------|----------|
| `queued` | Cancel: status changes to `cancelled`, response preserved |
| `in_progress` | Cancel: worker detects cancellation, status changes to `cancelled` |
| `completed` | Normal soft-delete (unchanged from current behavior) |
| `failed` | Normal soft-delete (unchanged) |
| `cancelled` | Normal soft-delete (unchanged) |

## Status Transitions

```
Valid transitions (extending existing state machine):

"" -> queued             (new: background request creation)
"" -> in_progress        (existing: synchronous request)
queued -> in_progress    (existing: worker claims)
queued -> cancelled      (new: client cancels before processing)
in_progress -> completed (existing)
in_progress -> failed    (existing)
in_progress -> cancelled (existing)
```
