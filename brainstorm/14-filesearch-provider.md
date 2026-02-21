# Brainstorm 14: File Search Provider

**Dependencies**: Brainstorm 12 (Function Registry), Spec 005 (Storage)
**Package**: `pkg/tools/builtins/filesearch/`

## Purpose

Built-in `file_search` tool matching OpenAI's capability. Provides the Vector Store API (`/v1/vector_stores`, `/v1/vector_stores/{id}/files`) for managing documents and a `file_search` tool that retrieves relevant chunks for RAG.

## Tool Definition

```json
{
  "type": "file_search",
  "name": "file_search",
  "description": "Search uploaded files for relevant information",
  "vector_store_ids": ["vs_abc123"]
}
```

## Management API (OpenAI-Compatible)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/vector_stores` | Create a vector store |
| GET | `/v1/vector_stores` | List vector stores |
| GET | `/v1/vector_stores/{id}` | Get vector store |
| DELETE | `/v1/vector_stores/{id}` | Delete vector store |
| POST | `/v1/vector_stores/{id}/files` | Upload and index a file |
| GET | `/v1/vector_stores/{id}/files` | List files in store |
| DELETE | `/v1/vector_stores/{id}/files/{file_id}` | Remove file |

## Pipeline

```
Upload: File → Parse (PDF/DOCX/TXT) → Chunk → Embed → Store vectors
Search: Query → Embed → Vector search → Return ranked chunks
```

## Vector Storage Options

| Backend | In-tree? | Pros | Cons |
|---------|---------|------|------|
| **pgvector** | Yes (reuse PostgreSQL) | No new infra | Slower at scale |
| **Qdrant** | External service | Fast, purpose-built | Extra deployment |
| **In-memory HNSW** | Yes | No deps | Not persistent |

**Recommendation**: Start with pgvector (extends our existing PostgreSQL from Spec 005). Add Qdrant adapter later.

## Embedding Options

| Option | Description |
|--------|-------------|
| LLM backend | If vLLM/LiteLLM supports `/v1/embeddings` |
| Dedicated model | Separate embedding model (e.g., BGE-M3, nomic-embed) |
| External API | OpenAI embeddings API, Cohere, etc. |

**Recommendation**: Use the LLM backend's `/v1/embeddings` endpoint if available. Fall back to a configurable embedding URL.

## Configuration

```yaml
builtins:
  file_search:
    enabled: true
    vector_backend: pgvector       # or "qdrant"
    embedding_url: http://llm-predictor:8080/v1/embeddings
    embedding_model: ""            # use default
    chunk_size: 512
    chunk_overlap: 50
    max_results: 10

    pgvector:
      dsn_file: /run/secrets/postgres/dsn  # reuse from storage

    qdrant:
      url: http://qdrant:6333
      collection: antwort_files
```

## File Parsing

Supported formats (P1):
- Plain text (.txt, .md)
- PDF (using a Go PDF library)

Supported formats (P2):
- DOCX
- HTML
- CSV

## Open Questions

- Should vector stores be tenant-scoped (like responses)?
  -> Yes. Each tenant has their own vector stores.
- Should embedding happen synchronously during upload or asynchronously?
  -> Asynchronously. Upload returns immediately, indexing happens in background. Status field tracks progress.
- Maximum file size?
  -> Configurable, default 50MB per file.

## Deliverables

- [ ] FileSearchProvider implementing FunctionProvider
- [ ] Vector Store management API (OpenAI-compatible subset)
- [ ] pgvector adapter for vector storage
- [ ] Text/PDF file parsing and chunking
- [ ] Embedding integration (via /v1/embeddings endpoint)
- [ ] file_search tool execution (query → embed → search → return)
- [ ] Configuration in config.yaml
- [ ] Tests
