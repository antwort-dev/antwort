# Implementation Plan: File Search Provider

**Branch**: `018-file-search` | **Date**: 2026-02-23 | **Spec**: [spec.md](spec.md)

## Summary

File search provider: query layer + metadata manager for vector-based document search. VectorStoreBackend interface with Qdrant adapter, EmbeddingClient for query-time embedding, Vector Store management API (4 endpoints), file_search tool.

## Technical Context

**Dependencies**: Qdrant Go client (HTTP, not gRPC). No other new dependencies.
**External services**: Qdrant (vector DB), embedding service (/v1/embeddings).

## Project Structure

```text
pkg/tools/builtins/filesearch/
├── provider.go         # FileSearchProvider implementing FunctionProvider
├── backend.go          # VectorStoreBackend interface
├── qdrant.go           # Qdrant adapter
├── embedding.go        # EmbeddingClient interface + HTTP client
├── api.go              # Vector Store management API handlers
├── store.go            # VectorStore metadata management
├── provider_test.go    # Provider + search tests
├── qdrant_test.go      # Qdrant adapter tests (mock HTTP)
└── api_test.go         # API endpoint tests
```
