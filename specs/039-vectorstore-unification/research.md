# Research: 039-vectorstore-unification

**Date**: 2026-03-03

## R1: Unified Interface Location

**Decision**: New `pkg/vectorstore/` package with the `Backend` interface. Both `filesearch` and `files` import it. No import cycles.

**Rationale**: A dedicated package makes the vector store a first-class abstraction. Neither `filesearch` nor `files` owns the interface, preventing future coupling.

## R2: pgvector Schema

**Decision**: One table per collection. Table name derived from collection name. Schema:
```sql
CREATE TABLE IF NOT EXISTS vs_{collection} (
    id TEXT PRIMARY KEY,
    vector vector({dimensions}),
    metadata JSONB NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_vs_{collection}_vector ON vs_{collection} USING hnsw (vector vector_cosine_ops);
```

**Rationale**: HNSW index provides fast approximate nearest-neighbor search. One table per collection keeps data isolated. The `vector` type from pgvector handles storage and distance computation.

## R3: Connection Pool Sharing

**Decision**: The pgvector backend accepts a `*pgxpool.Pool` parameter. The server passes the same pool used by the response store. No duplicate connections.

**Rationale**: pgx supports custom type registration per connection. The pgvector Go library (`pgvector-go`) registers the vector type on the pool. Sharing the pool avoids connection duplication.

## R4: In-Memory Search Algorithm

**Decision**: Brute-force cosine similarity. No index structure. Iterate all points in the collection, compute cosine distance, sort by score, return top-k.

**Rationale**: The in-memory backend is for testing with small datasets (hundreds of points). Brute-force is simple, correct, and fast enough. No external dependency needed.

## R5: Qdrant Migration Strategy

**Decision**: Move `QdrantBackend` from `pkg/tools/builtins/filesearch/qdrant.go` to `pkg/vectorstore/qdrant/qdrant.go`. Update imports in filesearch and files. Remove the old `VectorStoreBackend` interface from filesearch and `VectorIndexer` from files, replacing both with `vectorstore.Backend`.

**Rationale**: Clean cut migration. The old files become thin imports of the new package. Compile-time checks ensure nothing breaks.

## R6: External Dependency: pgvector-go

**Decision**: Use `github.com/pgvector/pgvector-go` for pgvector type registration. This is a small library (pure Go, no CGO) that registers the `vector` type with pgx.

**Rationale**: Constitution Principle II allows external dependencies in adapter packages. The pgvector adapter is in `pkg/vectorstore/pgvector/`, isolated from core. The library is necessary to send and receive vector data through pgx.
