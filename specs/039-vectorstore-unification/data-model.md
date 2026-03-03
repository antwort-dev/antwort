# Data Model: 039-vectorstore-unification

**Date**: 2026-03-03

## Interfaces

### Backend (5 methods, unified)

Replaces `VectorStoreBackend` (filesearch) + `VectorIndexer` (files).

| Method             | Input                                       | Output              |
|--------------------|---------------------------------------------|---------------------|
| CreateCollection   | ctx, name string, dimensions int            | error               |
| DeleteCollection   | ctx, name string                            | error               |
| Search             | ctx, collection string, vector []float32, maxResults int | []SearchMatch, error |
| UpsertPoints       | ctx, collection string, points []VectorPoint | error              |
| DeletePointsByFile | ctx, collection string, fileID string       | error               |

### Shared Types (moved to pkg/vectorstore/)

| Type        | Fields                                          | From        |
|-------------|------------------------------------------------|-------------|
| SearchMatch | DocumentID, Score, Content, Metadata            | filesearch  |
| VectorPoint | ID, Vector, Metadata                            | files       |

## Package Layout

```
pkg/vectorstore/
├── backend.go          # Backend interface, SearchMatch, VectorPoint
├── qdrant/
│   └── qdrant.go       # Moved from filesearch, implements Backend
├── pgvector/
│   └── pgvector.go     # NEW: PostgreSQL + pgvector, implements Backend
└── memory/
    └── memory.go       # NEW: In-memory brute-force, implements Backend
```

## pgvector Schema

One table per collection:

| Column   | Type         | Description                |
|----------|--------------|----------------------------|
| id       | TEXT PK      | Point identifier           |
| vector   | vector(N)    | Embedding vector           |
| metadata | JSONB        | Payload (file_id, content) |

HNSW index on vector column with cosine distance operator.

## Configuration Extension

```yaml
providers:
  file_search:
    enabled: true
    settings:
      backend: pgvector       # qdrant | pgvector | memory
      backend_url: ""          # qdrant only
      # pgvector uses the existing storage.postgres connection
```
