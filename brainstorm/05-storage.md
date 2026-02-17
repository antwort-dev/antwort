# Spec 05: State Management & Storage

**Branch**: `spec/05-storage`
**Dependencies**: Spec 01 (Core Protocol)
**Package**: `github.com/rhuss/antwort/pkg/storage`

## Purpose

Define the storage interface for persisting responses and conversation state, and implement the PostgreSQL adapter. Storage is required only for the stateful API tier and enables `previous_response_id` chaining, response retrieval, and deletion.

## Scope

### In Scope
- Storage interface definition
- PostgreSQL adapter implementation
- In-memory adapter (for testing and stateless deployments)
- Response persistence (create, get, delete)
- `previous_response_id` context reconstruction
- Schema migrations
- Connection pooling and lifecycle

### Out of Scope
- Tool state (MCP session state, file search indices) handled by tool implementations
- Caching layer (future optimization)
- Multi-tenancy enforcement middleware (depends on auth spec)

## Storage Interface

```go
// Store persists and retrieves OpenResponses responses.
type Store interface {
    // SaveResponse persists a completed response.
    // Called after inference completes (or after each agentic loop turn).
    SaveResponse(ctx context.Context, resp *api.Response) error

    // GetResponse retrieves a response by ID.
    // Returns ErrNotFound if the response does not exist.
    GetResponse(ctx context.Context, id string) (*api.Response, error)

    // DeleteResponse removes a response by ID.
    // Returns ErrNotFound if the response does not exist.
    DeleteResponse(ctx context.Context, id string) error

    // BuildContext reconstructs the full conversation context
    // by following the previous_response_id chain.
    // Returns items in chronological order:
    //   ancestor.input + ancestor.output + ... + current.input
    BuildContext(ctx context.Context, previousResponseID string) ([]api.Item, error)

    // HealthCheck verifies the store connection is alive.
    // Wired into the HTTP health endpoint (/healthz) and Kubernetes readiness probe.
    HealthCheck(ctx context.Context) error

    // Close releases database connections.
    Close() error
}

// Sentinel errors
var (
    ErrNotFound = errors.New("response not found")
    ErrConflict = errors.New("response already exists")
)
```

## PostgreSQL Adapter

### Schema

```sql
CREATE TABLE responses (
    id                   TEXT PRIMARY KEY,
    status               TEXT NOT NULL,
    model                TEXT NOT NULL,
    previous_response_id TEXT REFERENCES responses(id) ON DELETE SET NULL,
    input                JSONB NOT NULL,
    output               JSONB NOT NULL,
    usage_input_tokens   INTEGER,
    usage_output_tokens  INTEGER,
    usage_total_tokens   INTEGER,
    error                JSONB,
    extensions           JSONB,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ
);

CREATE INDEX idx_responses_previous ON responses(previous_response_id);
CREATE INDEX idx_responses_created ON responses(created_at);
```

Soft delete is used: `DeleteResponse` sets `deleted_at` rather than removing the row. This preserves chain integrity for `previous_response_id` references.

### Implementation

```go
type PostgresStore struct {
    pool *pgxpool.Pool
}

func NewPostgresStore(ctx context.Context, cfg PostgresConfig) (*PostgresStore, error)

type PostgresConfig struct {
    // DSN is the PostgreSQL connection string.
    DSN string `json:"dsn" env:"ANTWORT_DB_DSN"`

    // MaxConns is the maximum number of open connections (default: 25).
    MaxConns int `json:"max_conns" env:"ANTWORT_DB_MAX_CONNS"`

    // MaxIdleConns is the maximum number of idle connections (default: 5).
    MaxIdleConns int `json:"max_idle_conns" env:"ANTWORT_DB_MAX_IDLE_CONNS"`

    // ConnMaxLifetime is the maximum lifetime of a connection (default: 5m).
    ConnMaxLifetime time.Duration `json:"conn_max_lifetime" env:"ANTWORT_DB_CONN_MAX_LIFETIME"`

    // MigrateOnStart runs schema migrations at startup.
    MigrateOnStart bool `json:"migrate_on_start" env:"ANTWORT_DB_MIGRATE"`
}
```

Connection pool defaults when not explicitly configured:

| Parameter | Default | Rationale |
|-----------|---------|-----------|
| `MaxConns` | 25 | Sufficient for most single-instance deployments |
| `MaxIdleConns` | 5 | Keeps a small warm pool without wasting connections |
| `ConnMaxLifetime` | 5 min | Prevents stale connections, works with PgBouncer and cloud-managed databases |

### Context Reconstruction

`BuildContext` follows the `previous_response_id` chain using a recursive CTE:

```sql
WITH RECURSIVE chain AS (
    SELECT id, input, output, previous_response_id, created_at
    FROM responses
    WHERE id = $1 AND deleted_at IS NULL
    UNION ALL
    SELECT r.id, r.input, r.output, r.previous_response_id, r.created_at
    FROM responses r
    JOIN chain c ON r.id = c.previous_response_id
    WHERE r.deleted_at IS NULL
)
SELECT input, output FROM chain ORDER BY created_at ASC;
```

The result is flattened: `[ancestor.input, ancestor.output, ..., parent.input, parent.output]`.

A depth limit (configurable, default 100) prevents unbounded chain traversal.

### JSONB Query Capabilities

The `input`, `output`, `error`, and `extensions` columns use PostgreSQL's `JSONB` type, which enables structured queries that would be impossible with plain text JSON storage.

**Filtering by item type** (find responses containing a specific output item type):

```sql
SELECT id, status, created_at
FROM responses
WHERE output @> '[{"type": "message"}]'
  AND deleted_at IS NULL;
```

**Filtering by token usage** (find responses exceeding a token threshold):

```sql
SELECT id, model, usage_total_tokens
FROM responses
WHERE usage_total_tokens > 1000
  AND deleted_at IS NULL
ORDER BY usage_total_tokens DESC;
```

**Querying nested JSONB fields** (find responses with a specific model in the extensions):

```sql
SELECT id, extensions->>'routing_model' AS routing_model
FROM responses
WHERE extensions ? 'routing_model'
  AND deleted_at IS NULL;
```

**GIN indexing** for fast containment queries on JSONB columns:

```sql
-- Index the output column for @> (containment) queries
CREATE INDEX idx_responses_output_gin ON responses USING GIN (output);

-- Index the extensions column for ? (key existence) and @> queries
CREATE INDEX idx_responses_extensions_gin ON responses USING GIN (extensions);
```

GIN indexes accelerate `@>`, `?`, `?|`, and `?&` operators. They add write overhead, so only create them on columns that are frequently queried. The `input` column is typically not queried directly and does not need a GIN index.

## In-Memory Adapter

```go
// MemoryStore is an in-memory Store for testing and stateless deployments.
type MemoryStore struct {
    mu        sync.RWMutex
    responses map[string]*api.Response
    maxSize   int // eviction threshold
}

func NewMemoryStore(maxSize int) *MemoryStore
```

The memory store implements the same interface but with no durability guarantees. It supports optional LRU eviction when `maxSize` is reached.

## Migrations

Schema migrations use a simple embedded approach:

```go
//go:embed migrations/*.sql
var migrations embed.FS

// Migrate applies pending schema migrations.
func (s *PostgresStore) Migrate(ctx context.Context) error
```

Migration files are numbered sequentially:
- `001_create_responses.sql`
- `002_add_indexes.sql`
- etc.

## Multi-Tenancy Support

Tenant isolation is handled at the storage layer without changing the core engine logic.

### Tenant Context

A tenant is identified by a string extracted from the authenticated request. The tenant ID flows through context to all storage operations.

```go
type TenantContext struct {
    TenantID string
}

func TenantFromContext(ctx context.Context) string {
    tc, ok := ctx.Value(tenantContextKey{}).(TenantContext)
    if !ok {
        return "" // single-tenant mode (backwards compatible)
    }
    return tc.TenantID
}
```

When no auth middleware is configured, `TenantFromContext` returns an empty string. Storage implementations treat the empty tenant as "no filtering," preserving current single-tenant behavior. Existing deployments work unchanged with no migration required.

### Entity Scoping

| Entity | Scope | Notes |
|--------|-------|-------|
| Responses | Per tenant | Tenant A cannot see Tenant B's responses |
| Conversations | Per tenant | Conversation chains are tenant-isolated |
| Files | Per tenant | Uploaded files are private to the tenant |
| Vector stores | Per tenant | Embeddings are tenant-isolated |
| Connectors | Shared | MCP servers are shared infrastructure |
| Prompts | Shared or per-tenant | System prompts may be shared, user prompts are per-tenant |

The `Store` interface does not change. Implementations extract the tenant from context and apply filtering internally:

```go
func (s *PostgresStore) GetResponse(ctx context.Context, id string) (*api.Response, error) {
    tenant := TenantFromContext(ctx)
    if tenant == "" {
        // Single-tenant: no filtering (current behavior)
        return s.getResponseUnscoped(id)
    }
    // Multi-tenant: filter by tenant
    return s.getResponseByTenant(tenant, id)
}
```

Database schema adds a `tenant_id` column to the responses table:

```sql
ALTER TABLE responses ADD COLUMN tenant_id TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_responses_tenant ON responses(tenant_id);
```

### Data Migration When Enabling Auth

When auth is enabled for the first time on an existing deployment, previously stored data will have an empty tenant ID (and empty user ID). The recommended approach is to leave existing data with the empty identifier. New data created after auth is enabled will carry the tenant or user ID from the auth context. Data with empty identifiers is only visible when no auth middleware is configured, which is the simplest and safest migration strategy.

## Extension Points

- **Alternative backends**: Implement the `Store` interface for Redis, SQLite, DynamoDB, etc.
- **Custom serialization**: Override how items are serialized to JSONB (e.g., for encryption at rest)
- **Chain depth policies**: Configure max chain depth per tenant or model
- **TTL/retention**: Add automatic cleanup of old responses (future)

## Open Questions

- Should `BuildContext` include truncation logic (respecting `truncation: "auto"`) or should that be handled by the core engine after retrieval?
- Should we support streaming writes (saving partial responses during inference)?
- Is soft delete the right approach, or should we use a separate `deleted_responses` table?
- Should the `tenant_id` column be added in the initial schema migration, or deferred to a later migration when auth is implemented?

## Deliverables

- [ ] `pkg/storage/store.go` - Store interface and errors
- [ ] `pkg/storage/postgres/postgres.go` - PostgreSQL adapter
- [ ] `pkg/storage/postgres/config.go` - Configuration
- [ ] `pkg/storage/postgres/migrations/` - SQL migration files
- [ ] `pkg/storage/memory/memory.go` - In-memory adapter
- [ ] `pkg/storage/postgres/postgres_test.go` - Integration tests (testcontainers)
- [ ] `pkg/storage/memory/memory_test.go` - Unit tests
