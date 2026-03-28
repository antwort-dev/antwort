---
name: Antwort (RAG)
id: antwort-rag
description: OpenResponses gateway with file search, vector store, and citation generation
requires: [model]
optional: []
params:
  - key: namespace
    description: Kubernetes namespace for Antwort deployment
    default: antwort
  - key: backend_url
    description: vLLM backend URL (auto-detected if empty)
    default: ""
    source: default
  - key: vector_store
    description: Vector store type (memory or qdrant)
    default: memory
---

# Antwort (RAG)

Deploys Antwort with the full RAG pipeline from quickstart 08-rag.
Includes file upload, content extraction, vector store indexing, file_search tool, and citation generation.

## What Gets Installed

- Antwort Deployment with RAG configuration
- Service and OpenShift Route
- Optional: Qdrant StatefulSet (if vector_store=qdrant)
- ConfigMap with file search and vector store settings

## Prerequisites

- A model must be deployed via the `model` instill
- For Qdrant: persistent volume support on the cluster

## Notes

- In-memory vector store is suitable for testing (data lost on restart)
- Qdrant provides persistent vector storage across restarts
- File upload, chunking, and embedding happen within the Antwort process
