# Data Model: Scope-Based Authorization and Resource Permissions

## Entities

### Scope (runtime, not stored)

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| name | string | JWT `scope` claim or role expansion | Format: `resource:operation` (e.g., `responses:create`) |

Scopes are not stored entities. They exist at runtime, derived from JWT claims and role configuration.

### Role (configuration, not stored)

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| name | string | Config key | e.g., "viewer", "user", "manager", "admin" |
| scopes | []string | Config value | Mix of scope names and role references |

Roles are defined in configuration. Role references are resolved at startup into flat scope lists.

### Permissions (stored on resources)

| Field | Type | Format | Notes |
|-------|------|--------|-------|
| permissions | string | `owner\|group\|others` | Each level: combination of `r`, `w`, `d`, `-` |

Stored as a compact string on vector stores and files. Owner level is always `rwd` (immutable).

### VectorStore (extended)

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| id | string | yes | `vs_` prefix |
| name | string | no | User-provided name |
| owner | string | yes | `Identity.Subject` of creator (Spec 040) |
| tenant_id | string | yes | User group identifier (Spec 040) |
| permissions | string | yes | Default `rwd\|---\|---`. Settable by owner. |
| collection_name | string | yes | Backend vector collection |
| created_at | int64 | yes | Unix timestamp |

**New field**: `permissions`

### File (extended)

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| id | string | yes | File identifier |
| filename | string | yes | Original filename |
| bytes | int64 | yes | File size |
| purpose | string | yes | Upload purpose |
| status | string | yes | processing, completed, failed |
| user_id | string | yes | Owner (Identity.Subject) |
| permissions | string | yes | Default `rwd\|---\|---`. Settable by owner. |
| created_at | int64 | yes | Unix timestamp |

**New field**: `permissions`

### AgentProfile (extended)

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| name | string | yes | Profile identifier |
| description | string | no | Human-readable description |
| model | string | no | Default model |
| instructions | string | no | System instructions with template variables |
| tools | []ToolDefinition | no | Default tools |
| vector_store_ids | []string | no | Default vector stores for file_search (union merged with request) |
| temperature | *float64 | no | Default temperature |

**New field**: `vector_store_ids`

## Permission Enforcement Rules

### Scope Enforcement (endpoint level)

| Condition | Result |
|-----------|--------|
| No role_scopes configured | No enforcement (all endpoints open) |
| User has required scope | Request proceeds |
| User has `*` wildcard scope | Request proceeds (matches any) |
| User lacks required scope | 403 Forbidden |

### Permission Enforcement (resource level)

For vector stores and files with `owner|group|others` format:

| Caller relationship | Check | Result if denied |
|---------------------|-------|------------------|
| Caller is owner | Always allowed (owner = `rwd`) | N/A |
| Caller is admin (same tenant) | Read + delete allowed (Spec 040) | N/A |
| Caller in same tenant (group) | Check group permissions | 404 (not visible) |
| Caller in different tenant (others) | Check others permissions | 404 (not visible) |
| No identity (NoOp auth) | No filtering | N/A |

### Endpoint-to-Scope Map

| Endpoint | Scope |
|----------|-------|
| `POST /v1/responses` | `responses:create` |
| `GET /v1/responses` | `responses:read` |
| `GET /v1/responses/{id}` | `responses:read` |
| `GET /v1/responses/{id}/input_items` | `responses:read` |
| `DELETE /v1/responses/{id}` | `responses:delete` |
| `POST /v1/conversations` | `conversations:create` |
| `GET /v1/conversations` | `conversations:read` |
| `GET /v1/conversations/{id}` | `conversations:read` |
| `DELETE /v1/conversations/{id}` | `conversations:delete` |
| `GET /v1/conversations/{id}/items` | `conversations:read` |
| `POST /v1/conversations/{id}/items` | `conversations:write` |
| `POST /v1/vector_stores` | `vector_stores:create` |
| `GET /v1/vector_stores` | `vector_stores:read` |
| `GET /v1/vector_stores/{id}` | `vector_stores:read` |
| `DELETE /v1/vector_stores/{id}` | `vector_stores:delete` |
| `POST /v1/files` | `files:create` |
| `GET /v1/files` | `files:read` |
| `GET /v1/files/{id}` | `files:read` |
| `DELETE /v1/files/{id}` | `files:delete` |
| `GET /v1/agents` | `agents:read` |

### Role Hierarchy (default configuration)

```
admin ["*"]
  └── manager [user + vector_stores:*, files:delete, conversations:delete, responses:delete]
       └── user [viewer + responses:create, files:create, conversations:create, conversations:write]
            └── viewer [responses:read, files:read, vector_stores:read, conversations:read, agents:read]
```

## Relationships

```
Identity (auth context)
  |
  |-- Scopes (from JWT) ──┐
  |-- Roles (from JWT) ───┼── Effective Scopes (union) ── Scope Middleware
  |                        |
  |-- Subject ──────────── Owner (resource field)
  |-- TenantID ─────────── Group (permission level)
  |
Resource (vector store, file)
  |-- owner: string (immutable)
  |-- tenant_id: string
  |-- permissions: string ("rwd|r--|---")
  |
AgentProfile (config)
  |-- vector_store_ids: []string ──── Union Merge ──── Request vector_store_ids
```
