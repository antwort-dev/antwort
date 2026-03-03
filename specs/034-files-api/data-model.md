# Data Model: 034-files-api

**Date**: 2026-03-02

## Entities

### File

Represents an uploaded file with metadata and status tracking.

| Field       | Type       | Description                                          |
|-------------|------------|------------------------------------------------------|
| ID          | string     | Unique identifier, format: `file_` + 24 alphanum     |
| Object      | string     | Always `"file"`                                      |
| Filename    | string     | Original filename from upload                        |
| Bytes       | int64      | File size in bytes                                   |
| MIMEType    | string     | Detected MIME type (e.g., `application/pdf`)         |
| Purpose     | string     | One of: `assistants`, `batch`, `fine-tune`, `vision` |
| Status      | FileStatus | Current processing state                             |
| StatusError | string     | Error detail when Status is `failed`                 |
| UserID      | string     | Owner identity (from auth context)                   |
| CreatedAt   | int64      | Unix timestamp of upload                             |
| UpdatedAt   | int64      | Unix timestamp of last status change                 |

**State Transitions**:

```
uploaded --> processing --> completed
                       \-> failed
```

- `uploaded`: File stored, no ingestion started
- `processing`: Ingestion pipeline is running (extract/chunk/embed/index)
- `completed`: All chunks embedded and indexed successfully
- `failed`: Ingestion failed at any stage; StatusError contains detail

### Chunk

A segment of extracted text with positional metadata.

| Field      | Type   | Description                                    |
|------------|--------|------------------------------------------------|
| Index      | int    | Sequential position within the source document |
| Text       | string | Chunk text content                             |
| StartChar  | int    | Start character offset in extracted text        |
| EndChar    | int    | End character offset in extracted text          |

### ExtractionResult

Output of content extraction.

| Field       | Type   | Description                                        |
|-------------|--------|----------------------------------------------------|
| Text        | string | Extracted content as structured Markdown            |
| PageCount   | int    | Number of pages (0 for non-paged formats)          |
| Method      | string | Extraction method used (e.g., `docling`, `passthrough`) |

### VectorPoint

A chunk prepared for vector store insertion.

| Field    | Type              | Description                         |
|----------|-------------------|-------------------------------------|
| ID       | string            | Unique point identifier             |
| Vector   | []float32         | Embedding vector                    |
| Metadata | map[string]string | Payload: file_id, filename, content |

### VectorStoreFileRecord

Tracks the relationship between a file and a vector store.

| Field         | Type       | Description                                    |
|---------------|------------|------------------------------------------------|
| VectorStoreID | string     | ID of the vector store                         |
| FileID        | string     | ID of the file                                 |
| Status        | FileStatus | Ingestion status within this store             |
| ChunkCount    | int        | Number of chunks indexed (0 until completed)   |
| LastError     | string     | Error detail if ingestion failed               |
| CreatedAt     | int64      | Unix timestamp when file was added to store    |

**State Transitions**: Same as File (uploaded -> processing -> completed | failed)

### FileBatch (P3)

Tracks a batch of files being added to a vector store.

| Field         | Type            | Description                       |
|---------------|-----------------|-----------------------------------|
| ID            | string          | Unique identifier, `batch_` + 24  |
| VectorStoreID | string          | Target vector store               |
| Status        | string          | Overall batch status              |
| FileCounts    | FileBatchCounts | Counts by status                  |
| CreatedAt     | int64           | Unix timestamp                    |

### FileBatchCounts (P3)

| Field      | Type | Description                     |
|------------|------|---------------------------------|
| InProgress | int  | Files currently being ingested  |
| Completed  | int  | Files successfully ingested     |
| Failed     | int  | Files that failed ingestion     |
| Cancelled  | int  | Files whose ingestion cancelled |
| Total      | int  | Total files in batch            |

## Interfaces

### FileStore (3 methods)

Stores and retrieves file binary content.

| Method   | Input                              | Output                   |
|----------|------------------------------------|--------------------------|
| Store    | ctx, fileID string, content Reader | error                    |
| Retrieve | ctx, fileID string                 | ReadCloser, error        |
| Delete   | ctx, fileID string                 | error                    |

**Implementations**: Filesystem (default), Memory (testing), S3 (adapter)

### FileMetadataStore (5 methods)

CRUD for File metadata records. All operations are user-scoped.

| Method   | Input                                             | Output          |
|----------|---------------------------------------------------|-----------------|
| Save     | ctx, file *File                                   | error           |
| Get      | ctx, id string                                    | *File, error    |
| List     | ctx, opts ListOptions                             | *FileList, error|
| Delete   | ctx, id string                                    | error           |
| Update   | ctx, id string, status FileStatus, errMsg string  | error           |

**Implementations**: In-memory (default), PostgreSQL (adapter, future)

### ContentExtractor (2 methods)

Converts file content into structured text.

| Method           | Input                                          | Output                    |
|------------------|------------------------------------------------|---------------------------|
| Extract          | ctx, filename string, mimeType string, Reader  | *ExtractionResult, error  |
| SupportedFormats | (none)                                         | []string                  |

**Implementations**: Docling (HTTP adapter), Passthrough (built-in)

### Chunker (1 method)

Splits text into embedding-sized segments.

| Method | Input       | Output   |
|--------|-------------|----------|
| Chunk  | text string | []Chunk  |

**Implementations**: FixedSize (built-in)

### VectorIndexer (2 methods)

Writes and deletes vector points in a collection.

| Method             | Input                                       | Output |
|--------------------|---------------------------------------------|--------|
| UpsertPoints       | ctx, collection string, points []VectorPoint| error  |
| DeletePointsByFile | ctx, collection string, fileID string       | error  |

**Implementations**: Qdrant (extends existing QdrantBackend)

### VectorStoreFileStore (5 methods)

Tracks file-to-vector-store relationships and ingestion status.

| Method     | Input                            | Output                              |
|------------|----------------------------------|-------------------------------------|
| Save       | ctx, rec *VectorStoreFileRecord  | error                               |
| Get        | ctx, vsID string, fileID string  | *VectorStoreFileRecord, error       |
| List       | ctx, vsID string                 | []*VectorStoreFileRecord, error     |
| Delete     | ctx, vsID string, fileID string  | error                               |
| ListByFile | ctx, fileID string               | []*VectorStoreFileRecord, error     |

**Implementations**: In-memory (default)

## Relationships

```
File 1──* VectorStoreFileRecord *──1 VectorStore (from filesearch)
File 1──* Chunk (transient, during ingestion)
File 1──1 FileStore entry (binary content)
VectorStore 1──1 Collection (in vector DB)
```

- One File can be added to multiple VectorStores (each gets its own VectorStoreFileRecord)
- Deleting a File cascades: remove from all VectorStores (via ListByFile), delete chunks from vector DB, delete content from FileStore
- Removing a File from a VectorStore: delete chunks from that collection only, delete VectorStoreFileRecord; the File and its content remain
