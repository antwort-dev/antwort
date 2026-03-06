# Data Model: Async Responses (Background Mode)

**Feature**: 044-async-responses
**Date**: 2026-03-05

## Entity Changes

### Response (extended)

Existing entity with new fields for background processing.

| Field | Type | Description | New? |
|-------|------|-------------|------|
| `id` | string | Response ID (`resp_` prefix) | No |
| `status` | enum | `queued`, `in_progress`, `completed`, `failed`, `cancelled`, `incomplete`, `requires_action` | No (values existed) |
| `background` | bool | Whether this is a background request | No (field existed on Response, adding to CreateResponseRequest) |
| `background_request` | JSON | Serialized `CreateResponseRequest` for worker reconstruction | Yes |
| `worker_id` | string | ID of the worker that claimed this response | Yes |
| `worker_heartbeat` | timestamp | Last heartbeat from the claiming worker | Yes |
| `error` | JSON | Error information (populated on failure) | No (existed) |

### State Transitions

```
                    ┌──────────┐
    submit ────────>│  queued   │
                    └────┬─────┘
                         │ worker claims
                    ┌────v─────┐
                    │in_progress│
                    └──┬──┬──┬─┘
          completed    │  │  │   failed
         ┌─────────────┘  │  └──────────────┐
    ┌────v─────┐          │           ┌─────v────┐
    │completed │          │           │  failed   │
    └──────────┘          │           └──────────┘
                          │ cancel
                    ┌─────v────┐
                    │cancelled │
                    └──────────┘
```

Terminal states: `completed`, `failed`, `cancelled`

Transitions:
- `""` -> `queued`: Gateway creates background response
- `queued` -> `in_progress`: Worker claims the response
- `queued` -> `cancelled`: Client cancels before processing starts
- `in_progress` -> `completed`: Worker finishes successfully
- `in_progress` -> `failed`: Worker encounters error, crash (stale detection), or shutdown timeout
- `in_progress` -> `cancelled`: Client cancels during processing

Note: `queued` -> `cancelled` is a new transition not in the current `ValidateResponseTransition`. Must be added.

### Worker

Not a persisted entity. Ephemeral process identity.

| Field | Type | Description |
|-------|------|-------------|
| `worker_id` | string | Unique identifier for this worker process (generated at startup) |
| `mode` | enum | `gateway`, `worker`, `integrated` |

## Storage Interface Extensions

### New Methods on `ResponseStore`

| Method | Signature | Purpose |
|--------|-----------|---------|
| `UpdateResponse` | `(ctx, id string, updates ResponseUpdate) error` | Update status, output, error, heartbeat on existing response |
| `ClaimQueuedResponse` | `(ctx, workerID string) (*Response, *CreateResponseRequest, error)` | Atomically claim one queued response for processing |
| `ListByStatus` | `(ctx, status ResponseStatus, background bool) ([]*Response, error)` | List responses by status (for stale detection and TTL cleanup) |
| `CleanupExpired` | `(ctx, olderThan time.Time, statuses []ResponseStatus) (int, error)` | Delete terminal background responses older than TTL |

### ResponseUpdate Type

| Field | Type | Description |
|-------|------|-------------|
| `Status` | *ResponseStatus | New status (validated against state machine) |
| `Output` | []Item | Response output items (set on completion) |
| `Error` | *APIError | Error information (set on failure) |
| `Usage` | *Usage | Token usage (set on completion) |
| `CompletedAt` | *int64 | Completion timestamp |
| `WorkerHeartbeat` | *time.Time | Heartbeat timestamp |

## Configuration Extensions

### New Config Fields

| Path | Type | Default | Description |
|------|------|---------|-------------|
| `engine.mode` | string | `integrated` | Server mode: `gateway`, `worker`, `integrated` |
| `engine.background.poll_interval` | duration | `5s` | Worker poll interval for queued requests |
| `engine.background.drain_timeout` | duration | `30s` | Graceful shutdown drain timeout |
| `engine.background.staleness_timeout` | duration | `10m` | Mark in_progress as failed after this duration without heartbeat |
| `engine.background.heartbeat_interval` | duration | `30s` | Worker heartbeat update frequency |
| `engine.background.ttl` | duration | `24h` | Auto-cleanup for terminal background responses |
| `engine.background.cleanup_batch_size` | int | `100` | Max responses to clean up per poll cycle |

### ListOptions Extensions

| Field | Type | Description |
|-------|------|-------------|
| `Status` | string | Filter by response status |
| `Background` | *bool | Filter by background flag |

## Database Migration (PostgreSQL)

New migration `003_add_background.sql`:
- Add `background_request JSONB` column to `responses` table
- Add `worker_id TEXT` column to `responses` table
- Add `worker_heartbeat TIMESTAMPTZ` column to `responses` table
- Add index on `(status, background)` for worker polling queries (`idx_responses_status_background`)
- Add index on `(worker_heartbeat)` for stale detection queries (`idx_responses_worker_heartbeat`)
