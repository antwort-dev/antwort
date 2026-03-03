# API Contract: Files API

**Version**: 1.0 | **Base Path**: `/builtin` (via FunctionProvider route mounting)

## POST /files

Upload a file.

**Content-Type**: multipart/form-data

**Form Fields**:
| Field   | Type   | Required | Description                                      |
|---------|--------|----------|--------------------------------------------------|
| file    | file   | Yes      | The file to upload (max 50 MB default)           |
| purpose | string | Yes      | One of: `assistants`, `batch`, `fine-tune`, `vision` |

**Response** (201 Created):
```json
{
  "id": "file_abc123def456ghi789jkl012",
  "object": "file",
  "bytes": 120456,
  "created_at": 1709366400,
  "filename": "report.pdf",
  "purpose": "assistants",
  "status": "uploaded",
  "status_details": null
}
```

**Errors**:
- 400: Missing file or purpose, invalid purpose value
- 413: File exceeds maximum upload size

---

## GET /files

List files for the authenticated user.

**Query Parameters**:
| Param  | Type   | Default | Description                |
|--------|--------|---------|----------------------------|
| after  | string | (none)  | Cursor for pagination      |
| limit  | int    | 20      | Results per page (max 100) |
| order  | string | desc    | Sort order: `asc` or `desc`|
| purpose| string | (none)  | Filter by purpose          |

**Response** (200 OK):
```json
{
  "object": "list",
  "data": [
    {
      "id": "file_abc123def456ghi789jkl012",
      "object": "file",
      "bytes": 120456,
      "created_at": 1709366400,
      "filename": "report.pdf",
      "purpose": "assistants",
      "status": "completed",
      "status_details": null
    }
  ],
  "has_more": false,
  "first_id": "file_abc123def456ghi789jkl012",
  "last_id": "file_abc123def456ghi789jkl012"
}
```

---

## GET /files/{file_id}

Retrieve file metadata.

**Response** (200 OK): Same schema as single file object above.

**Errors**:
- 404: File not found (or belongs to different user)

---

## GET /files/{file_id}/content

Download file content.

**Response** (200 OK): Raw file bytes with appropriate Content-Type and Content-Disposition headers.

**Errors**:
- 404: File not found (or belongs to different user)

---

## DELETE /files/{file_id}

Delete a file. Removes file content from storage and cleans up all chunks from associated vector stores.

**Response** (200 OK):
```json
{
  "id": "file_abc123def456ghi789jkl012",
  "object": "file",
  "deleted": true
}
```

**Errors**:
- 404: File not found (or belongs to different user)

---

## POST /vector_stores/{store_id}/files

Add a file to a vector store, triggering asynchronous ingestion.

**Request Body**:
```json
{
  "file_id": "file_abc123def456ghi789jkl012"
}
```

**Response** (200 OK):
```json
{
  "id": "file_abc123def456ghi789jkl012",
  "object": "vector_store.file",
  "vector_store_id": "vs_xyz789abc123def456ghi012",
  "status": "in_progress",
  "created_at": 1709366400,
  "last_error": null
}
```

**Errors**:
- 404: Vector store or file not found
- 400: File already exists in this vector store

---

## GET /vector_stores/{store_id}/files

List files in a vector store with their ingestion status.

**Query Parameters**:
| Param  | Type   | Default | Description                          |
|--------|--------|---------|--------------------------------------|
| after  | string | (none)  | Cursor for pagination                |
| limit  | int    | 20      | Results per page (max 100)           |
| order  | string | desc    | Sort order: `asc` or `desc`          |
| filter | string | (none)  | Filter by status: `in_progress`, `completed`, `failed`, `cancelled` |

**Response** (200 OK):
```json
{
  "object": "list",
  "data": [
    {
      "id": "file_abc123def456ghi789jkl012",
      "object": "vector_store.file",
      "vector_store_id": "vs_xyz789abc123def456ghi012",
      "status": "completed",
      "created_at": 1709366400,
      "last_error": null
    }
  ],
  "has_more": false,
  "first_id": "file_abc123def456ghi789jkl012",
  "last_id": "file_abc123def456ghi789jkl012"
}
```

---

## DELETE /vector_stores/{store_id}/files/{file_id}

Remove a file from a vector store. Deletes all chunks belonging to this file from the store. The file itself remains in file storage.

**Response** (200 OK):
```json
{
  "id": "file_abc123def456ghi789jkl012",
  "object": "vector_store.file.deleted",
  "deleted": true
}
```

**Errors**:
- 404: Vector store or file not found in this store

---

## POST /vector_stores/{store_id}/file_batches (P3)

Add multiple files to a vector store in one operation.

**Request Body**:
```json
{
  "file_ids": [
    "file_abc123def456ghi789jkl012",
    "file_def456ghi789jkl012mno345"
  ]
}
```

**Response** (200 OK):
```json
{
  "id": "batch_abc123def456ghi789jkl012",
  "object": "vector_store.file_batch",
  "vector_store_id": "vs_xyz789abc123def456ghi012",
  "status": "in_progress",
  "created_at": 1709366400,
  "file_counts": {
    "in_progress": 2,
    "completed": 0,
    "failed": 0,
    "cancelled": 0,
    "total": 2
  }
}
```

---

## GET /vector_stores/{store_id}/file_batches/{batch_id} (P3)

Check batch status.

**Response** (200 OK): Same schema as batch object above with updated file_counts.

**Errors**:
- 404: Vector store or batch not found
