# Data Model: Audit Logging

**Feature**: 042-audit-logging
**Date**: 2026-03-04

## Entities

### AuditLogger

The central audit logging component. Wraps a `*slog.Logger` with nil-safe composition.

**Fields**:
- `logger`: The underlying structured logger instance (nil when audit disabled)

**Behavior**:
- When nil, all method calls are no-ops (zero overhead)
- When configured, extracts identity and tenant from request context automatically
- Emits events as structured log records with base fields + event-specific fields

### AuditEvent (Logical)

Not a persisted entity. A structured log record emitted to the audit output channel.

**Base Fields** (present on every event):

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `time` | timestamp | yes | RFC3339Nano, UTC |
| `level` | string | yes | INFO or WARN |
| `msg` | string | yes | Event name (e.g., `authz.scope_denied`) |
| `event` | string | yes | Same as msg, explicit for filtering |
| `subject` | string | no | Authenticated user identity (empty if anonymous) |
| `tenant_id` | string | no | Tenant membership (empty if no tenant) |
| `remote_addr` | string | no | Client network address (when available) |

### AuditConfig

Configuration for audit logging behavior.

**Fields**:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | boolean | false | Whether audit logging is active |
| `format` | string | "json" | Output format: "json" or "text" |
| `output` | string | "stdout" | Output destination: "stdout" or "file" |
| `file` | string | "" | File path (required when output is "file") |

**Validation Rules**:
- If `enabled` is true and `output` is "file", then `file` must be non-empty and the path must be writable
- If `enabled` is false, all other fields are ignored
- Invalid `format` or `output` values cause startup failure

## Event Catalog

### Authentication Events

| Event Name | Severity | Trigger | Event-Specific Fields |
|------------|----------|---------|----------------------|
| `auth.success` | INFO | Successful authentication | `auth_method`, `remote_addr` |
| `auth.failure` | WARN | Failed authentication | `auth_method`, `remote_addr`, `error` |
| `auth.rate_limited` | WARN | Rate limit exceeded | `tier`, `remote_addr` |

### Authorization Events

| Event Name | Severity | Trigger | Event-Specific Fields |
|------------|----------|---------|----------------------|
| `authz.scope_denied` | WARN | Missing scope (403) | `endpoint`, `required_scope`, `effective_scopes` |
| `authz.ownership_denied` | INFO | Non-owner access | `resource_type`, `resource_id`, `operation` |
| `authz.admin_override` | INFO | Admin cross-user access | `resource_type`, `resource_id`, `resource_owner`, `operation` |

### Resource Mutation Events

| Event Name | Severity | Trigger | Event-Specific Fields |
|------------|----------|---------|----------------------|
| `resource.created` | INFO | Resource created | `resource_type`, `resource_id` |
| `resource.deleted` | INFO | Resource deleted | `resource_type`, `resource_id` |
| `resource.permissions_changed` | INFO | Permissions updated | `resource_type`, `resource_id`, `old_permissions`, `new_permissions` |

### Tool Events

| Event Name | Severity | Trigger | Event-Specific Fields |
|------------|----------|---------|----------------------|
| `tool.executed` | INFO | Tool dispatched | `tool_type`, `tool_name`, `response_id` |
| `tool.failed` | WARN | Tool execution failed | `tool_type`, `tool_name`, `response_id`, `error` |

### System Events

| Event Name | Severity | Trigger | Event-Specific Fields |
|------------|----------|---------|----------------------|
| `config.startup` | INFO | Server started | `auth_enabled`, `audit_enabled`, `role_count`, `scope_enforcement` |

## Relationships

```text
AuditConfig ──configures──> AuditLogger
AuditLogger ──emits──> AuditEvent (structured log records)
AuditLogger ──reads──> context.Context (Identity, TenantID)
```

## State Transitions

AuditLogger has two states:
- **Disabled** (nil): No events emitted, zero overhead
- **Enabled** (non-nil): Events emitted to configured output

Transition: Disabled -> Enabled happens at server startup based on AuditConfig. There is no runtime toggle (restart required to change state).
