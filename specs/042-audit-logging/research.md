# Research: Audit Logging

**Feature**: 042-audit-logging
**Date**: 2026-03-04

## Decision 1: Audit Logger Architecture

**Decision**: Dedicated `pkg/audit/` package with nil-safe `Logger` wrapping `*slog.Logger`.

**Rationale**: The codebase already uses `log/slog` pervasively with key-value structured logging. A thin wrapper around `*slog.Logger` provides:
- Zero learning curve (same API patterns)
- Automatic identity extraction from context (DRY)
- Nil-safe design matching Constitution Principle III
- No external dependencies (Constitution Principle II)

**Alternatives considered**:
- **Separate audit interface with pluggable backends**: Over-engineered for Phase 1. Phase 2 can add sink interfaces without changing the audit API.
- **slog.Handler middleware**: Could intercept and redirect events based on attributes, but complicates configuration and blurs the boundary between operational and audit logging.
- **Context-based audit recorder**: Would require passing audit state through context. The explicit Logger parameter is simpler and matches existing patterns (e.g., `FilesAPI.logger`).

## Decision 2: Injection Strategy

**Decision**: Pass `*audit.Logger` as an optional parameter to components that emit audit events. Use nil to disable.

**Rationale**: The codebase uses two patterns for optional dependencies:
1. Constructor parameters (e.g., `engine.New(provider, store, cfg)`)
2. Setter methods (e.g., `adapter.SetConversationStore(convStore)`)

For audit logging, the audit logger needs to reach:
- `auth.Middleware()` (3 events: auth success, failure, rate limit)
- `scope.Middleware()` (1 event: scope denied)
- HTTP handlers in `transporthttp.Adapter` (resource.created, resource.deleted)
- `filesearch` API handlers (resource.created, resource.deleted, permissions_changed)
- `files` API handlers (resource.created, resource.deleted)
- `engine.Engine` (2 events: tool.executed, tool.failed)
- `cmd/server/main.go` (1 event: config.startup)

Using setter methods for adapter and constructor parameters for middleware/engine keeps the injection pattern consistent with existing code.

**Alternatives considered**:
- **Global audit logger**: Simpler wiring but violates dependency injection principle and makes testing harder.
- **Context-carried audit logger**: Too implicit. Components should declare their optional dependencies explicitly.

## Decision 3: Event Schema

**Decision**: Each audit event is a flat structured log record with a fixed set of base fields and event-specific fields.

**Rationale**: Flat records are:
- Easy to parse with jq, grep, log aggregation tools
- Compatible with slog's key-value attribute model
- Queryable without nested JSON path expressions

**Base fields** (present on every event):
- `time`: RFC3339Nano timestamp (slog default)
- `level`: INFO or WARN (slog level)
- `msg`: Event name (e.g., `authz.scope_denied`)
- `event`: Same as msg (explicit field for filtering)
- `subject`: Identity.Subject (empty if unauthenticated)
- `tenant_id`: Tenant ID (empty if no tenant)
- `remote_addr`: Client address (when available from request)

**Event-specific fields** vary per event type (documented in brainstorm 34).

**Alternatives considered**:
- **Nested JSON (event.data sub-object)**: More structured but harder to query with standard tools and not idiomatic slog.
- **Protobuf/binary format**: Not human-readable, overkill for Phase 1 stdout logging.

## Decision 4: Output Configuration

**Decision**: Two output modes: stdout (default when enabled) and file. Two formats: JSON (default) and text.

**Rationale**:
- **stdout + JSON**: K8s-native pattern. Container stdout is collected by fluentd/vector/filebeat without configuration. JSON enables structured queries.
- **file**: Useful for non-K8s development and for operators who need dedicated audit files separate from container logs.
- **text**: Human-readable format for development and debugging.

**Alternatives considered**:
- **Syslog**: External dependency, not stdlib. Can be added in Phase 2 as a sink.
- **Webhook/HTTP sink**: Network-dependent, retry complexity. Phase 2.
- **stderr**: Would mix with Go's default error output. stdout is cleaner.

## Decision 5: Tool Event Granularity

**Decision**: Audit tool dispatch and failure at the `executeTools` level, not at individual executor level.

**Rationale**: The engine's `executeToolsConcurrently()` and `executeToolsSequentially()` functions are the single dispatch points for all tool calls. Auditing here captures every tool type (MCP, builtin, function) uniformly without modifying each executor.

Tool type classification uses the existing `classifyToolType()` function which maps executors to types (mcp, file_search, code_interpreter, web_search, function).

**Alternatives considered**:
- **Audit in each ToolExecutor.Execute()**: Would require modifying every executor implementation. Violates DRY.
- **Audit only failures**: Misses the "who used which tool" visibility story (US-4).

## Decision 6: Storage-Layer vs Handler-Layer Ownership Audit

**Decision**: Ownership denial events are emitted at the storage layer (`ownerAllowed()` in memory store). Resource mutation events are emitted at the HTTP handler layer.

**Rationale**:
- **Ownership denial**: The storage layer is where the decision is made. The handler only sees a `storage.ErrNotFound` error. Auditing at the storage layer captures the actual denial decision with full context (caller subject, stored owner, resource ID, operation).
- **Resource mutations**: Handlers know whether the operation is a create or delete. The storage layer just does Save/Delete. Handlers have the request context and can emit richer events.

**Alternatives considered**:
- **All events at handler layer**: Ownership denial context (stored owner, admin bypass) would need to propagate up through errors. Complex.
- **All events at storage layer**: Storage doesn't know "create vs update" semantics in all cases.

## Decision 7: Documentation Requirements

**Decision**: Three documentation deliverables per Constitution v1.6.0:

1. **Reference**: Update `config-reference.adoc` and `environment-variables.adoc` with audit config settings
2. **Operations**: Add or update `security.adoc` with audit logging setup, event catalog, and interpretation guide
3. **Tutorial**: Not required (audit logging is an operational concern, not a user-facing capability)

**Rationale**: Audit logging introduces new configuration settings (mandatory per constitution) and is an observability/operations change (operations module). It's not a user-facing API.
