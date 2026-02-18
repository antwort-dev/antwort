# Data Model: State Management & Storage

**Feature**: 005-storage
**Date**: 2026-02-18

## Extended Interface: ResponseStore

The existing `ResponseStore` interface (Spec 002, `pkg/transport/handler.go`) is extended with new operations.

| Method | Signature | Description | New? |
|--------|-----------|-------------|------|
| GetResponse | `(ctx, id) -> (*Response, error)` | Retrieve by ID, excludes soft-deleted | No (existing) |
| DeleteResponse | `(ctx, id) -> error` | Soft-delete a response | No (existing, now soft-delete) |
| SaveResponse | `(ctx, *Response) -> error` | Persist a completed response | **Yes** |
| GetResponseForChain | `(ctx, id) -> (*Response, error)` | Retrieve by ID, includes soft-deleted | **Yes** |
| HealthCheck | `(ctx) -> error` | Verify store connection | **Yes** |
| Close | `() -> error` | Release resources | **Yes** |

## Stored Response Fields

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Primary key, `resp_` prefix + 24 alphanumeric chars |
| Object | string | Always "response" |
| Status | ResponseStatus | completed, incomplete, failed, cancelled, requires_action |
| Input | []Item | Input items from the request |
| Output | []Item | Output items from inference |
| Model | string | Model used for inference |
| Usage | *Usage | Token usage (input, output, total) |
| Error | *APIError | Error details (if status is failed) |
| PreviousResponseID | string | Link to previous response in chain |
| CreatedAt | int64 | Unix timestamp of creation |
| Extensions | map[string]json.RawMessage | Provider extension data |
| TenantID | string | Tenant identifier (empty for single-tenant) |
| DeletedAt | *time.Time | Soft-delete timestamp (nil if not deleted) |

## PostgreSQL Schema

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS responses (
    id                   TEXT PRIMARY KEY,
    tenant_id            TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL,
    model                TEXT NOT NULL,
    previous_response_id TEXT,
    input                JSONB NOT NULL DEFAULT '[]',
    output               JSONB NOT NULL DEFAULT '[]',
    usage_input_tokens   INTEGER NOT NULL DEFAULT 0,
    usage_output_tokens  INTEGER NOT NULL DEFAULT 0,
    usage_total_tokens   INTEGER NOT NULL DEFAULT 0,
    error                JSONB,
    extensions           JSONB,
    created_at           BIGINT NOT NULL,
    deleted_at           TIMESTAMPTZ,
    CONSTRAINT fk_previous FOREIGN KEY (previous_response_id) REFERENCES responses(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_responses_tenant ON responses(tenant_id);
CREATE INDEX IF NOT EXISTS idx_responses_previous ON responses(previous_response_id);
CREATE INDEX IF NOT EXISTS idx_responses_created ON responses(created_at);
```

## Sentinel Errors

| Error | Description |
|-------|-------------|
| ErrNotFound | Response does not exist or has been soft-deleted |
| ErrConflict | Response with this ID already exists |

## TenantContext

| Operation | Description |
|-----------|-------------|
| SetTenant(ctx, tenantID) -> ctx | Inject tenant ID into context |
| GetTenant(ctx) -> string | Extract tenant ID from context (empty = no tenant) |

The tenant context key is a private type to prevent collisions (per Constitution VIII).
