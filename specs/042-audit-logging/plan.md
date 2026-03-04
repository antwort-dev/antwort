# Implementation Plan: Audit Logging

**Branch**: `042-audit-logging` | **Date**: 2026-03-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/042-audit-logging/spec.md`

## Summary

Add a dedicated audit logging subsystem to antwort that records security-relevant events (authentication, authorization, resource mutations, tool execution) separately from operational logging. The audit logger follows nil-safe composition (disabled by default, zero overhead when off) and outputs structured records to stdout or file. Twelve event types across five categories are emitted at existing integration points in the auth middleware, scope middleware, storage layer, HTTP handlers, and engine tool dispatch.

## Technical Context

**Language/Version**: Go 1.22+ (consistent with all existing specs)
**Primary Dependencies**: Go standard library only (`log/slog`, `context`, `os`, `io`, `encoding/json`, `fmt`, `time`, `strings`)
**Storage**: N/A (audit events are emitted as log records, not persisted by antwort)
**Testing**: Go standard `testing` package, table-driven tests, nil-safe tests
**Target Platform**: Linux containers on Kubernetes (stdout collected by log aggregation)
**Project Type**: Web service (gateway)
**Performance Goals**: Zero measurable overhead when audit disabled. Negligible overhead when enabled (slog is designed for high-throughput structured logging).
**Constraints**: No external dependencies. Must not introduce import cycles.
**Scale/Scope**: 12 audit event types, ~12 integration points across 8 files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First Design | PASS | `audit.Logger` is a concrete type (not interface) since there's only one implementation. If Phase 2 needs pluggable sinks, an interface can be extracted then. Single concrete type is simpler and sufficient. |
| II. Zero External Dependencies | PASS | stdlib only (`log/slog`, `context`, `os`, `io`) |
| III. Nil-Safe Composition | PASS | Core design: nil Logger = no-ops. Matches existing patterns (optional store, optional tools). |
| IV. Typed Error Domain | PASS | Config validation errors at startup use existing error patterns. Audit logging itself does not surface errors to clients. |
| V. Validate Early, Fail Fast | PASS | Invalid audit config (e.g., non-writable file path) causes startup failure. |
| VI. Protocol-Agnostic Provider | N/A | Audit does not touch provider layer. |
| VII. Streaming | N/A | Audit is not streaming-related. |
| VIII. Context Carries Cross-Cutting Data | PASS | Audit logger reads Identity and TenantID from context. Does not add to context. |
| IX. Kubernetes-Native | PASS | stdout JSON output is K8s-native (collected by DaemonSet log agents). |
| Documentation | PASS | Will update config-reference.adoc, environment-variables.adoc, and security.adoc (operations module). |
| Testing | PASS | Table-driven tests for event emission, nil-safe tests, config validation error paths. |

No violations. No complexity tracking needed.

## Project Structure

### Documentation (this feature)

```text
specs/042-audit-logging/
├── spec.md
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
pkg/audit/
├── audit.go             # Logger type, Log method, nil-safe design
└── audit_test.go        # Table-driven tests, nil-safe tests

pkg/config/
└── config.go            # Add AuditConfig struct to Config

pkg/auth/
└── middleware.go         # Add audit calls: auth.success, auth.failure, auth.rate_limited

pkg/auth/scope/
└── middleware.go         # Add audit call: authz.scope_denied

pkg/storage/memory/
└── memory.go            # Add audit calls: authz.ownership_denied, authz.admin_override

pkg/transport/http/
├── adapter.go           # Add audit calls: resource.created (response), resource.deleted (response)
└── conversations.go     # Add audit calls: resource.created (conversation), resource.deleted (conversation)

pkg/files/
└── api.go               # Add audit calls: resource.created (file), resource.deleted (file)

pkg/tools/builtins/filesearch/
└── api.go               # Add audit calls: resource.created (vector_store), resource.deleted (vector_store), resource.permissions_changed

pkg/engine/
└── loop.go              # Add audit calls: tool.executed, tool.failed

cmd/server/
└── main.go              # Wire audit logger, emit config.startup event

docs/modules/reference/pages/
├── config-reference.adoc    # Add audit config section
└── environment-variables.adoc # Add ANTWORT_AUDIT_* vars

docs/modules/operations/pages/
└── security.adoc            # Add audit logging section with event catalog
```

**Structure Decision**: No new packages beyond `pkg/audit/`. All other changes are additions to existing files. This keeps the feature self-contained with minimal structural impact.

## Design Decisions

### D1: Logger Type (not Interface)

`audit.Logger` is a concrete struct, not an interface. There is only one implementation in Phase 1. Extracting an interface is trivial if Phase 2 needs pluggable sinks, but premature abstraction adds complexity without benefit.

### D2: Injection via Setters and Parameters

The audit logger reaches components through:
- **Constructor parameter**: `auth.Middleware(chain, limiter, adminRole, auditLogger)` and `scope.Middleware(roles, scopes, auditLogger)`
- **Setter method**: `adapter.SetAuditLogger(auditLogger)` for HTTP handlers
- **Config field**: `engine.Config.AuditLogger` for engine tool events
- **Direct call**: `main.go` emits `config.startup` directly

This matches existing patterns: middleware uses constructor params, adapter uses setters, engine uses Config struct.

### D3: Storage Layer Audit via Setter

The memory store needs the audit logger for ownership denial and admin override events. Adding `SetAuditLogger(*audit.Logger)` to the memory store follows the existing setter pattern used by the HTTP adapter (e.g., `SetConversationStore`).

### D4: Files and Vector Store API Audit

`pkg/files/api.go` (FilesAPI) and `pkg/tools/builtins/filesearch/api.go` (filesearch Provider) both handle their own resource creation/deletion. The audit logger is injected via setter methods on these types, consistent with existing optional dependency patterns.

### D5: Event Emission in Tool Dispatch

Audit events for tools are emitted in `executeToolsConcurrently()` and `executeToolsSequentially()` after each tool call completes. This captures both success and failure at the single dispatch point without modifying individual executors.

### D6: Configuration Structure

```yaml
audit:
  enabled: true
  format: json     # json | text
  output: stdout   # stdout | file
  file: /var/log/antwort/audit.log
```

Environment variable overrides:
- `ANTWORT_AUDIT_ENABLED` (bool)
- `ANTWORT_AUDIT_FORMAT` (string)
- `ANTWORT_AUDIT_OUTPUT` (string)
- `ANTWORT_AUDIT_FILE` (string)

### D7: Remote Address Extraction

Authentication events include `remote_addr` from the HTTP request. The auth middleware already has access to `r.RemoteAddr`. For events emitted deeper in the stack (storage layer, engine), the remote address is not available from context and is omitted. This is acceptable because those events already carry the `subject` which identifies the user.
