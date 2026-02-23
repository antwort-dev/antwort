# Brainstorm 14: File Search Provider

**Dependencies**: Brainstorm 12 (Function Registry), Spec 005 (Storage)
**Package**: `pkg/tools/builtins/filesearch/`

## Purpose

Built-in `file_search` tool matching OpenAI's capability. Provides the Vector Store API (`/v1/vector_stores`, `/v1/vector_stores/{id}/files`) for managing documents and a `file_search` tool that retrieves relevant chunks for RAG.

## Architecture: No Compute in Main Process

All compute-intensive operations happen outside antwort:

```
antwort (gateway)
├── Vector Store API (CRUD, metadata)
├── file_search tool (query routing)
│
├──→ Embedding Service (external, /v1/embeddings)
│    Converts text to vectors
│
└──→ Vector Database (external, Qdrant/Milvus/pgvector)
     Stores and searches vectors
```

Antwort is the orchestrator: it receives files, sends text to the embedding service, stores vectors in the vector DB, and queries the vector DB on search. No embeddings or vector math runs in-process.

## VectorStore Provider Interface

Pluggable backend for vector storage, similar to SearchAdapter in web search:

```go
type VectorStoreBackend interface {
    CreateCollection(ctx, name string, dimensions int) error
    DeleteCollection(ctx, name string) error
    Upsert(ctx, collection string, docs []VectorDocument) error
    Delete(ctx, collection string, docIDs []string) error
    Search(ctx, collection string, vector []float32, maxResults int) ([]SearchMatch, error)
}

type VectorDocument struct {
    ID       string
    Vector   []float32
    Metadata map[string]string  // source file, chunk index, etc.
    Content  string             // original text chunk
}

type SearchMatch struct {
    DocumentID string
    Score      float32
    Content    string
    Metadata   map[string]string
}
```

Implementations:
- **Qdrant adapter** (P1): HTTP API, lightweight, purpose-built
- **Milvus adapter** (P2): Distributed, scalable
- **pgvector adapter** (P2): Reuse existing PostgreSQL

## Embedding Client Interface

```go
type EmbeddingClient interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimensions() int
}
```

Implementation: HTTP client calling any OpenAI-compatible `/v1/embeddings` endpoint. Works with:
- Dedicated embedding models (sentence-transformers, BGE, nomic-embed)
- LiteLLM proxy (routes to any embedding model)
- TEI (Text Embeddings Inference from Hugging Face)

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
Upload: File → Parse (TXT/PDF) → Chunk → Embed (external) → Store (external VectorDB)
Search: Query → Embed (external) → Search (external VectorDB) → Return ranked chunks
```

## Configuration

```yaml
providers:
  file_search:
    enabled: true
    settings:
      embedding_url: http://embedding-service:8080/v1/embeddings
      embedding_model: ""              # use default model
      vector_backend: qdrant           # or "milvus", "pgvector"
      chunk_size: 512
      chunk_overlap: 50
      max_results: 10
      max_file_size: 50MB

      qdrant:
        url: http://qdrant:6333

      milvus:
        url: http://milvus:19530

      pgvector:
        dsn_file: /run/secrets/postgres/dsn
```

## File Parsing

Supported formats (P1):
- Plain text (.txt, .md)
- PDF (using a Go PDF library)

Supported formats (P2):
- DOCX, HTML, CSV

## Quickstart Addition

Add a `05b-file-search` or extend `06-rag` quickstart with:
- **Qdrant**: lightweight vector DB (single pod, `qdrant/qdrant:latest`)
- **Embedding service**: TEI (Text Embeddings Inference, `ghcr.io/huggingface/text-embeddings-inference:cpu-latest`) with a small model (all-MiniLM-L6-v2, 80MB, runs on CPU)
- Antwort with file_search provider enabled

This gives a complete RAG setup with no GPU needed for embeddings.

## Decisions

- Vector stores are tenant-scoped (like responses)
- File metadata stored in antwort's existing storage (PostgreSQL or in-memory)
- Vectors stored in external vector DB
- Embedding happens synchronously during upload for P1 (async with status tracking for P2)
- Maximum file size: configurable, default 50MB

## Deliverables

- [ ] FileSearchProvider implementing FunctionProvider
- [ ] VectorStoreBackend interface
- [ ] Qdrant adapter
- [ ] EmbeddingClient interface + OpenAI-compatible HTTP client
- [ ] Vector Store management API (OpenAI-compatible subset)
- [ ] Text/PDF file parsing and chunking
- [ ] file_search tool execution (query → embed → search → return)
- [ ] Configuration in config.yaml
- [ ] Quickstart with Qdrant + TEI
- [ ] Tests
