# Brainstorm 34: Audit Logging

**Date**: 2026-03-04
**Participants**: Roland Huss
**Goal**: Design structured audit logging for compliance, debugging, and operational visibility in multi-user deployments.

## Motivation

With authentication (Spec 007), resource ownership (Spec 040), and scope-based authorization (Spec 041) in place, antwort now makes authorization decisions on every request. These decisions need to be observable for two purposes:

1. **Operational visibility** (Phase 1): "Why can't Bob see this vector store?" Debugging multi-user issues requires a clear trail of who accessed what, when, and whether access was granted or denied.
2. **Compliance** (Phase 2): SOC2, GDPR, FedRAMP require tamper-evident audit trails with retention policies. The Phase 1 infrastructure should be extensible to support compliance sinks later.

## Distinction from Existing Logging

| Concern | Existing | Audit (new) |
|---------|----------|-------------|
| Purpose | System health, debugging | Who did what, authorization trail |
| Examples | "server starting on port 8080", "provider request took 2.3s" | "alice created response resp_abc, authorized via scope responses:create" |
| Level | slog.Info/Debug/Warn/Error | Dedicated audit logger |
| Volume | All operational events | Security events + mutations only |
| Retention | Standard log rotation | Potentially longer for compliance |
| Output | slog.Default() | Separate slog.Logger with own handler |

## Design Decisions

### Event Scope: Security events + mutations

Audit only security-relevant events and state-changing operations. Reads are excluded to manage volume. This covers:

- **Authentication**: success, failure, rate limiting
- **Authorization**: scope denial (403), ownership denial (404), admin override usage
- **Mutations**: resource created, deleted, permissions changed
- **Tool execution**: tool dispatched, tool failed

### Output Channel: Dedicated audit logger

A separate `slog.Logger` instance with its own handler, configurable independently from operational logging. Shares the slog API but produces isolated output.

```go
// pkg/audit/audit.go
type Logger struct {
    logger *slog.Logger
}

func (l *Logger) Log(ctx context.Context, event string, attrs ...any) {
    if l == nil {
        return // nil-safe: no audit when not configured
    }
    // Extract identity from context automatically
    identity := auth.IdentityFromContext(ctx)
    baseAttrs := []any{
        "event", event,
        "timestamp", time.Now().UTC().Format(time.RFC3339Nano),
    }
    if identity != nil {
        baseAttrs = append(baseAttrs, "subject", identity.Subject)
        if tid := identity.TenantID(); tid != "" {
            baseAttrs = append(baseAttrs, "tenant_id", tid)
        }
    }
    l.logger.LogAttrs(ctx, slog.LevelInfo, event,
        toAttrs(append(baseAttrs, attrs...))...)
}
```

### Default State: Nil-safe, opt-in

Disabled by default. Enable via config. Follows constitution Principle III. No overhead in dev/quickstart deployments. When nil, all `audit.Log()` calls are no-ops.

### Configuration

```yaml
audit:
  enabled: true
  format: json       # json or text
  output: stdout     # stdout or file
  file: /var/log/antwort/audit.log  # only when output=file
```

## Audit Event Catalog

### Authentication Events

| Event | Trigger | Severity | Key Fields |
|-------|---------|----------|------------|
| `auth.success` | Successful authentication | info | subject, auth_method (jwt/apikey), remote_addr |
| `auth.failure` | Failed authentication | warn | auth_method, remote_addr, error |
| `auth.rate_limited` | Rate limit exceeded | warn | subject, tier, remote_addr |

### Authorization Events

| Event | Trigger | Severity | Key Fields |
|-------|---------|----------|------------|
| `authz.scope_denied` | Missing scope for endpoint (403) | warn | subject, endpoint, required_scope, effective_scopes |
| `authz.ownership_denied` | Non-owner access attempt (404) | info | subject, resource_type, resource_id, operation |
| `authz.admin_override` | Admin accessed another user's resource | info | subject, resource_type, resource_id, resource_owner, operation |

### Resource Mutation Events

| Event | Trigger | Severity | Key Fields |
|-------|---------|----------|------------|
| `resource.created` | Resource created | info | subject, resource_type, resource_id |
| `resource.deleted` | Resource deleted | info | subject, resource_type, resource_id |
| `resource.permissions_changed` | Permissions updated | info | subject, resource_type, resource_id, old_permissions, new_permissions |

### Tool Events

| Event | Trigger | Severity | Key Fields |
|-------|---------|----------|------------|
| `tool.executed` | Tool call dispatched | info | subject, tool_type, tool_name, response_id |
| `tool.failed` | Tool execution failed | warn | subject, tool_type, tool_name, response_id, error |

### System Events

| Event | Trigger | Severity | Key Fields |
|-------|---------|----------|------------|
| `config.startup` | Server started | info | auth_enabled, audit_enabled, role_count, scope_enforcement |

## Audit Event Format (JSON)

```json
{
  "time": "2026-03-04T15:30:00.123Z",
  "event": "authz.scope_denied",
  "level": "WARN",
  "subject": "bob",
  "tenant_id": "team-a",
  "endpoint": "POST /v1/responses",
  "required_scope": "responses:create",
  "effective_scopes": ["responses:read", "files:read"],
  "remote_addr": "10.0.0.5:43210"
}
```

```json
{
  "time": "2026-03-04T15:31:00.456Z",
  "event": "resource.created",
  "level": "INFO",
  "subject": "alice",
  "tenant_id": "team-a",
  "resource_type": "vector_store",
  "resource_id": "vs_abc123"
}
```

## Integration Points

Where audit events are emitted in the existing code:

| Event | Location | Current Code |
|-------|----------|-------------|
| `auth.success` | `pkg/auth/middleware.go:55` | `slog.Debug("authentication succeeded", ...)` -> upgrade to audit |
| `auth.failure` | `pkg/auth/middleware.go:34` | `slog.Warn("authentication failed", ...)` -> add audit |
| `auth.rate_limited` | `pkg/auth/middleware.go:64` | `slog.Warn("rate limit exceeded", ...)` -> add audit |
| `authz.scope_denied` | `pkg/auth/scope/middleware.go:88` | 403 response -> add audit |
| `authz.ownership_denied` | `pkg/storage/memory/memory.go:39` | `slog.Debug("ownership denied", ...)` -> add audit |
| `authz.admin_override` | `pkg/storage/memory/memory.go:30` | admin bypass path -> add audit |
| `resource.created` | handlers (create endpoints) | after successful save -> add audit |
| `resource.deleted` | handlers (delete endpoints) | after successful delete -> add audit |
| `resource.permissions_changed` | `pkg/tools/builtins/filesearch/api.go` | after permissions update -> add audit |
| `tool.executed` | `pkg/engine/loop.go` | after tool dispatch -> add audit |
| `tool.failed` | `pkg/engine/loop.go` | on tool error -> add audit |

## Implementation Approach

### Phase 1: slog-based audit logger

- New `pkg/audit/` package with nil-safe `Logger`
- Audit config in `pkg/config/config.go`
- Wire `audit.Logger` through middleware and handlers
- Emit events at the 12 integration points listed above
- JSON output to stdout (K8s-native, collected by fluentd/vector)

### Phase 2: Compliance extensions (future)

- Separate audit sink interface (database, webhook, S3)
- Event signing for tamper evidence
- Retention policies
- Query API for audit events

## Scope

### In Scope (spec candidate)
- `pkg/audit/` package with nil-safe Logger
- Audit config (enabled, format, output)
- 12 audit events at identified integration points
- JSON structured format
- Automatic subject/tenant extraction from context

### Out of Scope
- Tamper-evident storage (Phase 2)
- Audit query API (Phase 2)
- Event signing (Phase 2)
- Log shipping/aggregation (infrastructure concern)
- Read event auditing (high volume, not needed for Phase 1)

## Open Questions

None. All design decisions resolved.
