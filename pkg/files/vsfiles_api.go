package files

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/rhuss/antwort/pkg/api"
)

// VSFilesAPI provides HTTP handlers for vector store file operations.
type VSFilesAPI struct {
	metadata    FileMetadataStore
	vsFileStore VectorStoreFileStore
	vsLookup    VectorStoreLookup
	indexer     VectorIndexer
	pipeline    *IngestionPipeline
}

func (v *VSFilesAPI) handleAddFile(w http.ResponseWriter, r *http.Request) {
	storeID := r.PathValue("store_id")

	// Verify vector store exists by looking up its collection.
	if _, err := v.vsLookup(storeID); err != nil {
		writeAPIError(w, api.NewNotFoundError("vector store not found"))
		return
	}

	var req struct {
		FileID string `json:"file_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, api.NewInvalidRequestError("body", "invalid request body"))
		return
	}
	if req.FileID == "" {
		writeAPIError(w, api.NewInvalidRequestError("file_id", "file_id is required"))
		return
	}

	// Verify file exists and belongs to user.
	file, err := v.metadata.Get(r.Context(), req.FileID)
	if err != nil {
		writeAPIError(w, api.NewNotFoundError("file not found"))
		return
	}

	// Check if already added.
	if existing, _ := v.vsFileStore.Get(r.Context(), storeID, req.FileID); existing != nil {
		writeAPIError(w, api.NewInvalidRequestError("file_id", "file already exists in this vector store"))
		return
	}

	// Create record and trigger ingestion.
	rec := NewVectorStoreFileRecord(storeID, req.FileID)
	if err := v.vsFileStore.Save(r.Context(), rec); err != nil {
		writeAPIError(w, api.NewServerError("failed to add file to vector store"))
		return
	}

	v.pipeline.Ingest(file, storeID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rec)
}

func (v *VSFilesAPI) handleListFiles(w http.ResponseWriter, r *http.Request) {
	storeID := r.PathValue("store_id")

	// Verify vector store exists.
	if _, err := v.vsLookup(storeID); err != nil {
		writeAPIError(w, api.NewNotFoundError("vector store not found"))
		return
	}

	records, err := v.vsFileStore.List(r.Context(), storeID)
	if err != nil {
		writeAPIError(w, api.NewServerError("failed to list files"))
		return
	}

	// Apply filter if provided.
	filter := r.URL.Query().Get("filter")
	if filter != "" {
		var filtered []*VectorStoreFileRecord
		for _, rec := range records {
			if string(rec.Status) == filter {
				filtered = append(filtered, rec)
			}
		}
		records = filtered
	}

	// Apply pagination.
	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	if limit > 100 {
		limit = 100
	}

	after := r.URL.Query().Get("after")
	start := 0
	if after != "" {
		for i, rec := range records {
			if rec.FileID == after {
				start = i + 1
				break
			}
		}
	}

	end := start + limit
	hasMore := end < len(records)
	if end > len(records) {
		end = len(records)
	}
	page := records[start:end]

	resp := map[string]interface{}{
		"object":   "list",
		"data":     page,
		"has_more": hasMore,
	}
	if len(page) > 0 {
		resp["first_id"] = page[0].FileID
		resp["last_id"] = page[len(page)-1].FileID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (v *VSFilesAPI) handleRemoveFile(w http.ResponseWriter, r *http.Request) {
	storeID := r.PathValue("store_id")
	fileID := r.PathValue("file_id")

	// Verify record exists.
	if _, err := v.vsFileStore.Get(r.Context(), storeID, fileID); err != nil {
		writeAPIError(w, api.NewNotFoundError("file not found in vector store"))
		return
	}

	// Delete chunks from vector DB.
	if v.indexer != nil {
		collectionName, err := v.vsLookup(storeID)
		if err == nil {
			_ = v.indexer.DeletePointsByFile(r.Context(), collectionName, fileID)
		}
	}

	// Delete record.
	_ = v.vsFileStore.Delete(r.Context(), storeID, fileID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      fileID,
		"object":  "vector_store.file.deleted",
		"deleted": true,
	})
}
