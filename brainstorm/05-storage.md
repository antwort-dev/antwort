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
- Multi-tenancy (future, depends on auth)

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

    // MaxConns is the connection pool size.
    MaxConns int `json:"max_conns" env:"ANTWORT_DB_MAX_CONNS"`

    // MigrateOnStart runs schema migrations at startup.
    MigrateOnStart bool `json:"migrate_on_start" env:"ANTWORT_DB_MIGRATE"`
}
```

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

## Extension Points

- **Alternative backends**: Implement the `Store` interface for Redis, SQLite, DynamoDB, etc.
- **Custom serialization**: Override how items are serialized to JSONB (e.g., for encryption at rest)
- **Chain depth policies**: Configure max chain depth per tenant or model
- **TTL/retention**: Add automatic cleanup of old responses (future)

## Open Questions

- Should `BuildContext` include truncation logic (respecting `truncation: "auto"`) or should that be handled by the core engine after retrieval?
- Should we support streaming writes (saving partial responses during inference)?
- Is soft delete the right approach, or should we use a separate `deleted_responses` table?
- Should the store be aware of multi-tenancy (tenant column) from the start?

## Deliverables

- [ ] `pkg/storage/store.go` - Store interface and errors
- [ ] `pkg/storage/postgres/postgres.go` - PostgreSQL adapter
- [ ] `pkg/storage/postgres/config.go` - Configuration
- [ ] `pkg/storage/postgres/migrations/` - SQL migration files
- [ ] `pkg/storage/memory/memory.go` - In-memory adapter
- [ ] `pkg/storage/postgres/postgres_test.go` - Integration tests (testcontainers)
- [ ] `pkg/storage/memory/memory_test.go` - Unit tests
