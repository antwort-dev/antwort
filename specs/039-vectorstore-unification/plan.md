# Implementation Plan: Vector Store Unification & New Backends

**Branch**: `039-vectorstore-unification` | **Date**: 2026-03-03 | **Spec**: [spec.md](spec.md)

## Summary

Unify the split vector store interfaces into `pkg/vectorstore/Backend` (5 methods). Migrate Qdrant, add pgvector (reuses PostgreSQL) and in-memory (enables CI) backends. This is a refactoring + new backends feature.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: stdlib + `pgvector-go` (pgvector adapter only)
**Storage**: pgvector uses existing PostgreSQL, in-memory is ephemeral
**Testing**: `go test` with in-memory backend (no external services needed)

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | PASS | Backend (5 methods, at limit) |
| II. Zero External Deps | PASS | pgvector-go in adapter package only |
| III. Nil-Safe | PASS | No backend = file_search disabled |
| V. Validate Early | PASS | pgvector extension check at startup |

## Design Decisions

### D1: Package Layout

```
pkg/vectorstore/
├── backend.go          # Backend interface + shared types
├── qdrant/qdrant.go    # Migrated from filesearch
├── pgvector/pgvector.go # NEW
└── memory/memory.go     # NEW
```

### D2: Migration Strategy

1. Create `pkg/vectorstore/` with unified interface
2. Move `QdrantBackend` to `pkg/vectorstore/qdrant/`
3. Update `filesearch` to import from `pkg/vectorstore/`
4. Update `pkg/files/` to import from `pkg/vectorstore/`
5. Remove old interfaces (`VectorStoreBackend`, `VectorIndexer`, `Embedder`)
6. Update server wiring

### D3: pgvector Connection Sharing

The pgvector backend accepts `*pgxpool.Pool` from the server. Same pool as response storage. No duplicate connections.

### D4: Backend Selection

The `file_search` provider settings gain explicit backend selection. Default remains `qdrant` for backward compatibility.
