# Research: State Management & Storage

**Feature**: 005-storage
**Date**: 2026-02-18

## R1: PostgreSQL Driver Choice

**Decision**: Use `github.com/jackc/pgx/v5` as the PostgreSQL driver.

**Rationale**: pgx is the most widely used pure-Go PostgreSQL driver. It provides native support for connection pooling (`pgxpool`), JSONB encoding/decoding, TLS, and context-based cancellation. It does not depend on `database/sql`, avoiding the overhead of the generic SQL interface.

**Alternatives considered**:
- `database/sql` + `lib/pq`: Older driver, less performant, no native JSONB support. Would need manual JSON marshaling.
- `database/sql` + `pgx/v5/stdlib`: Uses pgx under the hood but through the database/sql interface. Adds unnecessary abstraction.
- `go-pg`: ORM-oriented, heavier than needed for a simple key-value-like store.

## R2: Schema Migration Strategy

**Decision**: Embed SQL migration files using `//go:embed` and apply them sequentially on startup.

**Rationale**: Embedded migrations avoid external tooling (no `goose`, `migrate`, or similar). The migration runner is simple: track applied migrations in a `schema_migrations` table, apply pending ones in order. This keeps the project self-contained.

**Alternatives considered**:
- `golang-migrate/migrate`: Well-known but adds an external dependency.
- `pressly/goose`: Similar concern.
- Manual schema creation: No versioning, error-prone for upgrades.

## R3: Soft Delete Implementation

**Decision**: Add a `deleted_at TIMESTAMPTZ` column. `GetResponse` filters `WHERE deleted_at IS NULL`. `GetResponseForChain` does NOT filter by `deleted_at`.

**Rationale**: Soft delete preserves chain integrity. When a response is deleted, its `previous_response_id` links remain valid for chain traversal. A separate `GetResponseForChain` method provides chain-aware retrieval that includes soft-deleted responses.

**Alternatives considered**:
- Separate `deleted_responses` table: Requires JOINs, complicates chain traversal.
- Hard delete with cascade: Breaks chains irreversibly.
- Logical `is_deleted` boolean: Equivalent to `deleted_at`, but `deleted_at` also records when deletion happened.

## R4: Multi-Tenancy Scoping

**Decision**: Use a `tenant_id TEXT NOT NULL DEFAULT ''` column. Storage implementations extract tenant from context and add `WHERE tenant_id = $tenant` to queries. Empty tenant means no scoping.

**Rationale**: This approach requires no interface changes. The tenant flows through context (injected by auth middleware in Spec 006). Single-tenant deployments work unchanged. The `DEFAULT ''` means existing data migrates without changes.

**Alternatives considered**:
- Row-Level Security (RLS): PostgreSQL-native but requires SET ROLE per connection, complicating connection pooling.
- Separate schemas per tenant: Operational overhead, connection pool per schema.
- Tenant as a method parameter: Breaks the clean interface, forces every caller to pass tenant.

## R5: Connection Pool Configuration

**Decision**: Use pgxpool with configurable MaxConns (default 25), MinConns (default 5), MaxConnLifetime (default 5min).

**Rationale**: pgxpool provides built-in connection pooling. Defaults are suitable for single-instance deployments. Production deployments behind PgBouncer or cloud-managed PostgreSQL can tune these.

## R6: In-Memory Store LRU Eviction

**Decision**: Use a doubly-linked list + map for O(1) LRU eviction, protected by sync.RWMutex.

**Rationale**: Standard LRU implementation. The map provides O(1) lookup by ID. The linked list tracks access order for eviction. RWMutex allows concurrent reads with exclusive writes.

**Alternatives considered**:
- `container/list` from stdlib: Works for the list component.
- No eviction (grow unbounded): Memory risk for long-running processes.
- Time-based eviction (TTL): More complex, not needed for the in-memory use case.
