# Implementation Plan: State Management & Storage

**Branch**: `005-storage` | **Date**: 2026-02-18 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/005-storage/spec.md`

## Summary

Implement the persistence layer for antwort. Extend the existing `ResponseStore` interface with `SaveResponse`, `GetResponseForChain`, `HealthCheck`, and `Close`. Provide two implementations: an in-memory store for testing/development, and a PostgreSQL adapter for production. Integrate SaveResponse into the engine's `CreateResponse` flow. Support multi-tenancy via tenant context scoping and soft delete for chain integrity.

## Technical Context

**Language/Version**: Go 1.22+ (consistent with Specs 001-004)
**Primary Dependencies**: `pgx/v5` (PostgreSQL driver, adapter package only). Go standard library for core interface and in-memory store.
**Storage**: PostgreSQL 14+ for production. In-memory for testing/development.
**Testing**: `go test` with table-driven tests. PostgreSQL integration tests using testcontainers-go.
**Target Platform**: Library packages, cross-platform (in-memory), Linux (PostgreSQL adapter)
**Project Type**: Single Go module (`github.com/rhuss/antwort`)
**Constraints**: External dependency (pgx) only in adapter package. Core interface uses stdlib only. Nil-safe composition for optional store.
**Scale/Scope**: ~5 entities, 24 FRs, 3 packages (interface + 2 adapters), ~15 source files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | PASS | Extends ResponseStore (6 methods total). Within guideline (5+Close). |
| II. Zero Dependencies | PASS | pgx only in `pkg/storage/postgres/` (adapter). Core interface stdlib-only. |
| III. Nil-Safe Composition | PASS | Engine skips save when store is nil. FR-007. |
| IV. Typed Error Domain | PASS | ErrNotFound, ErrConflict as typed errors. |
| V. Validate Early | PASS | ID validation before save. Duplicate detection. |
| VIII. Context Propagation | PASS | Tenant ID via context.Context. |
| IX. Kubernetes-Native | PASS | Health checks for readiness probes, TLS support. |
| Layer Dependencies | PASS | `pkg/storage` imports `pkg/api` + `pkg/transport` (for interface). Adapters import `pkg/storage`. |

No violations. All gates pass.

## Project Structure

### Documentation (this feature)

```text
specs/005-storage/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0: design decisions
├── data-model.md        # Phase 1: entity definitions
├── quickstart.md        # Phase 1: usage guide
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```text
pkg/
├── transport/
│   └── handler.go               # AMENDED: Extend ResponseStore with SaveResponse, GetResponseForChain, HealthCheck, Close
│
├── storage/
│   ├── doc.go                    # Package documentation
│   ├── errors.go                 # ErrNotFound, ErrConflict sentinel errors
│   ├── tenant.go                 # TenantContext helpers (get/set tenant from context)
│   ├── tenant_test.go            # Tenant context tests
│   │
│   ├── memory/
│   │   ├── memory.go             # In-memory store with LRU eviction
│   │   └── memory_test.go        # Unit tests
│   │
│   └── postgres/
│       ├── postgres.go           # PostgreSQL adapter (pgx pool)
│       ├── config.go             # PostgresConfig struct
│       ├── migrations.go         # Embedded SQL migrations
│       ├── migrations/
│       │   └── 001_create_responses.sql
│       └── postgres_test.go      # Integration tests (testcontainers)
│
└── engine/
    ├── engine.go                 # MODIFIED: Call SaveResponse after WriteResponse/WriteEvent
    └── history.go                # MODIFIED: Use GetResponseForChain for chain traversal
```

**Structure Decision**: The storage interface extends `ResponseStore` in `pkg/transport/handler.go` (where it's already defined). Adapter implementations live in sub-packages under `pkg/storage/`. The pgx dependency is contained within the postgres adapter package.

## Complexity Tracking

No constitutional violations. No complexity justifications needed.
