# Quickstart 08: RAG (Document Upload, Ingestion & Search)

Deploy antwort with a full RAG pipeline: upload documents, extract content via Docling, chunk, embed, index into a vector store, and search via the `file_search` tool. Supports PDF, DOCX, PPTX, images (with OCR), and plain text.

**Time to deploy**: 10 minutes (after LLM backend is running)

## Architecture

```
User                 antwort              docling-serve    embedding-svc    qdrant
 |                     |                      |                |              |
 |-- upload file ----> |                      |                |              |
 |<-- file_id -------- |                      |                |              |
 |                     |                      |                |              |
 |-- add to store ---> |                      |                |              |
 |<-- in_progress ---- |                      |                |              |
 |                     |-- extract ---------> |                |              |
 |                     |<-- markdown -------- |                |              |
 |                     |-- embed chunks ----> |  ------------> |              |
 |                     |<-- vectors --------- |  <------------ |              |
 |                     |-- index -----------> |                | -----------> |
 |                     |<-- done ------------ |                | <----------- |
 |                     |                      |                |              |
 |-- file_search ----> |                      |                |              |
 |                     |-- embed query -----> |  ------------> |              |
 |                     |-- search ----------> |                | -----------> |
 |<-- results -------- |                      |                |              |
```

**Components**:
- **antwort**: Gateway with Files API and file_search tool
- **Qdrant**: Vector database for chunk storage and similarity search
- **docling-serve**: Document extraction service (PDF, DOCX, images with OCR)
- **embedding-service**: vLLM with `all-MiniLM-L6-v2` for text embeddings (CPU-only)

## Prerequisites

- [Shared LLM Backend](../shared/llm-backend/) deployed and running
- `kubectl` or `oc` CLI configured
- At least 8 GB of allocatable memory on the cluster (for all four components)

## Deploy

```bash
# Create namespace
kubectl create namespace antwort

# Deploy all components
kubectl apply -k quickstarts/08-rag/base/ -n antwort

# Wait for backing services (embedding model download takes ~60s)
kubectl rollout status deployment/qdrant -n antwort --timeout=120s
kubectl rollout status deployment/docling-serve -n antwort --timeout=300s
kubectl rollout status deployment/embedding-service -n antwort --timeout=300s

# Wait for antwort
kubectl rollout status deployment/antwort -n antwort --timeout=60s
```

### OpenShift / ROSA

For external access via Route:

```bash
# Apply with OpenShift overlay
kubectl apply -k quickstarts/08-rag/openshift/ -n antwort

# Get the route URL
ROUTE=$(kubectl get route antwort -n antwort -o jsonpath='{.spec.host}')
echo "Antwort URL: https://$ROUTE"
```

## Test

### Setup Port-Forward

```bash
# Using port-forward (vanilla Kubernetes)
kubectl port-forward -n antwort svc/antwort 8080:8080 &

# Or using the Route URL (OpenShift)
# export URL=https://$ROUTE

export URL=http://localhost:8080
```

### Step 1: Upload a Text File

```bash
echo "Kubernetes is an open source container orchestration platform. \
It automates deployment, scaling, and management of containerized applications. \
Pods are the smallest deployable units in Kubernetes." > /tmp/k8s-intro.txt

FILE_ID=$(curl -s -X POST "$URL/builtin/files" \
  -F "file=@/tmp/k8s-intro.txt" \
  -F "purpose=assistants" | jq -r .id)

echo "Uploaded file: $FILE_ID"
```

### Step 2: Create a Vector Store

```bash
VS_ID=$(curl -s -X POST "$URL/builtin/vector_stores" \
  -H "Content-Type: application/json" \
  -d '{"name":"docs"}' | jq -r .id)

echo "Vector store: $VS_ID"
```

### Step 3: Add File to Vector Store (Triggers Ingestion)

```bash
curl -s -X POST "$URL/builtin/vector_stores/$VS_ID/files" \
  -H "Content-Type: application/json" \
  -d "{\"file_id\":\"$FILE_ID\"}" | jq .
```

### Step 4: Wait for Ingestion to Complete

```bash
for i in $(seq 1 30); do
  STATUS=$(curl -s "$URL/builtin/vector_stores/$VS_ID/files" | \
    jq -r '.data[0].status')
  echo "Ingestion status: $STATUS"
  [ "$STATUS" = "completed" ] && break
  [ "$STATUS" = "failed" ] && { echo "FAILED"; break; }
  sleep 2
done
```

### Step 5: Search via file_search in a Conversation

Now use the LLM with file_search to query your document:

```bash
curl -s -X POST "$URL/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "/mnt/models",
    "tools": [{"type": "file_search", "vector_store_ids": ["'"$VS_ID"'"]}],
    "input": "What are Pods in Kubernetes?"
  }' | jq '.output[] | select(.type == "message") | .content[].text'
```

The LLM will search the uploaded document, find the relevant chunk about Pods, and include it in its answer.

### Step 6: Upload a PDF (Docling Extraction)

```bash
# Upload any PDF document
PDF_ID=$(curl -s -X POST "$URL/builtin/files" \
  -F "file=@/path/to/document.pdf" \
  -F "purpose=assistants" | jq -r .id)

# Add to vector store
curl -s -X POST "$URL/builtin/vector_stores/$VS_ID/files" \
  -H "Content-Type: application/json" \
  -d "{\"file_id\":\"$PDF_ID\"}" | jq .

# Wait for ingestion (PDF extraction takes longer)
for i in $(seq 1 60); do
  STATUS=$(curl -s "$URL/builtin/vector_stores/$VS_ID/files" | \
    jq -r "[.data[] | select(.id==\"$PDF_ID\")][0].status")
  echo "PDF ingestion: $STATUS"
  [ "$STATUS" = "completed" ] && break
  [ "$STATUS" = "failed" ] && break
  sleep 2
done
```

### Step 7: Batch Upload Multiple Files

```bash
# Upload several files first
F1=$(curl -s -X POST "$URL/builtin/files" -F "file=@doc1.txt" -F "purpose=assistants" | jq -r .id)
F2=$(curl -s -X POST "$URL/builtin/files" -F "file=@doc2.txt" -F "purpose=assistants" | jq -r .id)
F3=$(curl -s -X POST "$URL/builtin/files" -F "file=@doc3.txt" -F "purpose=assistants" | jq -r .id)

# Add all at once via batch
curl -s -X POST "$URL/builtin/vector_stores/$VS_ID/file_batches" \
  -H "Content-Type: application/json" \
  -d "{\"file_ids\":[\"$F1\",\"$F2\",\"$F3\"]}" | jq .
```

### File Lifecycle Management

```bash
# List all uploaded files
curl -s "$URL/builtin/files" | jq .

# Get file metadata
curl -s "$URL/builtin/files/$FILE_ID" | jq .

# Download original file content
curl -s "$URL/builtin/files/$FILE_ID/content" -o downloaded.txt

# List files in vector store
curl -s "$URL/builtin/vector_stores/$VS_ID/files" | jq .

# Remove file from store (chunks deleted, file remains)
curl -s -X DELETE "$URL/builtin/vector_stores/$VS_ID/files/$FILE_ID" | jq .

# Delete file entirely (cascades to all vector stores)
curl -s -X DELETE "$URL/builtin/files/$FILE_ID" | jq .
```

## Configuration Reference

All settings are in `base/antwort-configmap.yaml` under `providers.files.settings`:

| Setting | Default | Description |
|---------|---------|-------------|
| `store_type` | `filesystem` | File storage backend (`filesystem` or `memory`) |
| `store_path` | `/data/files` | Filesystem base directory for uploads |
| `max_upload_size` | `52428800` | Maximum file size in bytes (50 MB) |
| `docling_url` | (required) | docling-serve endpoint URL |
| `docling_ocr` | `true` | Enable OCR for scanned documents and images |
| `docling_timeout` | `300s` | Per-file extraction timeout |
| `chunk_size` | `800` | Maximum tokens per chunk |
| `chunk_overlap` | `200` | Token overlap between consecutive chunks |
| `pipeline_workers` | `4` | Concurrent ingestion goroutines |

## Without Docling (Text-Only Mode)

If you don't need PDF/DOCX support, remove the docling-serve deployment and clear `docling_url` in the ConfigMap. Plain text, Markdown, and CSV files will still work via the built-in passthrough extractor. Attempting to ingest a PDF will return a clear error explaining that docling-serve is required.

## Teardown

```bash
kubectl delete namespace antwort
```
