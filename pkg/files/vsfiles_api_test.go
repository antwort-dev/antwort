package files

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rhuss/antwort/pkg/vectorstore"
)

// mockVectorIndexer records calls to UpsertPoints and DeletePointsByFile.
type mockVectorIndexer struct {
	deletedFiles []string // "collection|fileID"
}

func (m *mockVectorIndexer) CreateCollection(_ context.Context, _ string, _ int) error { return nil }
func (m *mockVectorIndexer) DeleteCollection(_ context.Context, _ string) error        { return nil }
func (m *mockVectorIndexer) Search(_ context.Context, _ string, _ []float32, _ int) ([]vectorstore.SearchMatch, error) {
	return nil, nil
}

func (m *mockVectorIndexer) UpsertPoints(_ context.Context, _ string, _ []VectorPoint) error {
	return nil
}

func (m *mockVectorIndexer) DeletePointsByFile(_ context.Context, collection, fileID string) error {
	m.deletedFiles = append(m.deletedFiles, collection+"|"+fileID)
	return nil
}

// mockIngestionPipeline records Ingest calls without doing real work.
type mockIngestionPipeline struct {
	ingested []string // "fileID|vectorStoreID"
}

func (m *mockIngestionPipeline) record(file *File, vectorStoreID string) {
	m.ingested = append(m.ingested, file.ID+"|"+vectorStoreID)
}

// newTestVSFilesAPI creates a VSFilesAPI backed by in-memory stores and a mock lookup.
// The validStores map controls which store IDs are "known" and maps them to collection names.
func newTestVSFilesAPI(validStores map[string]string) (*VSFilesAPI, *MemoryMetadataStore, *MemoryVectorStoreFileStore, *mockVectorIndexer, *mockIngestionPipeline) {
	metadata := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()
	indexer := &mockVectorIndexer{}
	mock := &mockIngestionPipeline{}
	fileStore := NewMemoryFileStore()

	lookup := func(vsID string) (string, error) {
		col, ok := validStores[vsID]
		if !ok {
			return "", fmt.Errorf("vector store %q not found", vsID)
		}
		return col, nil
	}

	pipeline := NewIngestionPipeline(PipelineConfig{
		FileStore:   fileStore,
		Metadata:    metadata,
		VSFileStore: vsFileStore,
		Passthrough: NewPassthroughExtractor(),
		Indexer:     indexer,
		VSLookup:    lookup,
		Workers:     1,
	})

	api := &VSFilesAPI{
		metadata:    metadata,
		vsFileStore: vsFileStore,
		vsLookup:    lookup,
		indexer:     indexer,
		pipeline:    pipeline,
		batches:     NewBatchStore(),
	}

	return api, metadata, vsFileStore, indexer, mock
}

// setupMux registers VSFilesAPI handlers on a new ServeMux and returns it.
func setupVSFilesMux(api *VSFilesAPI) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /vector_stores/{store_id}/files", api.handleAddFile)
	mux.HandleFunc("GET /vector_stores/{store_id}/files", api.handleListFiles)
	mux.HandleFunc("DELETE /vector_stores/{store_id}/files/{file_id}", api.handleRemoveFile)
	mux.HandleFunc("POST /vector_stores/{store_id}/file_batches", api.handleCreateBatch)
	mux.HandleFunc("GET /vector_stores/{store_id}/file_batches/{batch_id}", api.handleGetBatch)
	return mux
}

// seedFile creates a file record in the metadata store so it can be referenced.
func seedFile(t *testing.T, metadata *MemoryMetadataStore, fileID, filename string) *File {
	t.Helper()
	file := NewFile(fileID, filename, "text/plain", "assistants", "", 100)
	if err := metadata.Save(context.Background(), file); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	return file
}

func TestHandleAddFile(t *testing.T) {
	tests := []struct {
		name       string
		storeID    string
		body       string
		wantStatus int
		wantErr    string
	}{
		{
			name:       "success",
			storeID:    "vs_001",
			body:       `{"file_id": "file_001"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "vector store not found",
			storeID:    "vs_unknown",
			body:       `{"file_id": "file_001"}`,
			wantStatus: http.StatusNotFound,
			wantErr:    "vector store not found",
		},
		{
			name:       "missing file_id",
			storeID:    "vs_001",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "file_id is required",
		},
		{
			name:       "file not found",
			storeID:    "vs_001",
			body:       `{"file_id": "file_nonexistent"}`,
			wantStatus: http.StatusNotFound,
			wantErr:    "file not found",
		},
		{
			name:       "invalid JSON body",
			storeID:    "vs_001",
			body:       `{invalid`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, metadata, _, _, _ := newTestVSFilesAPI(map[string]string{
				"vs_001": "collection_001",
			})
			mux := setupVSFilesMux(api)

			// Seed file_001 so "success" and "duplicate" tests can find it.
			seedFile(t, metadata, "file_001", "test.txt")

			req := httptest.NewRequest("POST", "/vector_stores/"+tt.storeID+"/files", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if tt.wantErr != "" {
				var resp map[string]interface{}
				json.Unmarshal(rr.Body.Bytes(), &resp)
				errObj, ok := resp["error"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected error in response, got %s", rr.Body.String())
				}
				if msg, _ := errObj["message"].(string); !strings.Contains(msg, tt.wantErr) {
					t.Errorf("error message %q does not contain %q", msg, tt.wantErr)
				}
			}
		})
	}
}

func TestHandleAddFile_Duplicate(t *testing.T) {
	api, metadata, _, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)
	seedFile(t, metadata, "file_001", "test.txt")

	// First add should succeed.
	req := httptest.NewRequest("POST", "/vector_stores/vs_001/files", strings.NewReader(`{"file_id":"file_001"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("first add: status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	// Second add should return 400 (already exists).
	req = httptest.NewRequest("POST", "/vector_stores/vs_001/files", strings.NewReader(`{"file_id":"file_001"}`))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("duplicate add: status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	errObj, _ := resp["error"].(map[string]interface{})
	if msg, _ := errObj["message"].(string); !strings.Contains(msg, "already exists") {
		t.Errorf("error message %q does not contain 'already exists'", msg)
	}
}

func TestHandleAddFile_TriggersIngestion(t *testing.T) {
	api, metadata, vsFileStore, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)
	seedFile(t, metadata, "file_001", "test.txt")

	req := httptest.NewRequest("POST", "/vector_stores/vs_001/files", strings.NewReader(`{"file_id":"file_001"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	// Verify the record was created with processing status.
	rec, err := vsFileStore.Get(context.Background(), "vs_001", "file_001")
	if err != nil {
		t.Fatalf("get record: %v", err)
	}
	if rec.Status != FileStatusProcessing {
		t.Errorf("status = %q, want %q", rec.Status, FileStatusProcessing)
	}
	if rec.Object != "vector_store.file" {
		t.Errorf("object = %q, want %q", rec.Object, "vector_store.file")
	}

	// Verify JSON response has correct fields.
	var resp VectorStoreFileRecord
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.FileID != "file_001" {
		t.Errorf("response file_id = %q, want %q", resp.FileID, "file_001")
	}
	if resp.VectorStoreID != "vs_001" {
		t.Errorf("response vector_store_id = %q, want %q", resp.VectorStoreID, "vs_001")
	}
}

func TestHandleListFiles(t *testing.T) {
	api, metadata, vsFileStore, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	// Seed files with different statuses.
	seedFile(t, metadata, "file_001", "a.txt")
	seedFile(t, metadata, "file_002", "b.txt")
	seedFile(t, metadata, "file_003", "c.txt")

	ctx := context.Background()
	rec1 := NewVectorStoreFileRecord("vs_001", "file_001")
	rec1.Status = FileStatusCompleted
	vsFileStore.Save(ctx, rec1)

	rec2 := NewVectorStoreFileRecord("vs_001", "file_002")
	rec2.Status = FileStatusProcessing
	vsFileStore.Save(ctx, rec2)

	rec3 := NewVectorStoreFileRecord("vs_001", "file_003")
	rec3.Status = FileStatusCompleted
	vsFileStore.Save(ctx, rec3)

	tests := []struct {
		name       string
		query      string
		wantCount  int
		wantStatus int
	}{
		{
			name:       "list all files",
			query:      "",
			wantCount:  3,
			wantStatus: http.StatusOK,
		},
		{
			name:       "filter by completed status",
			query:      "?filter=completed",
			wantCount:  2,
			wantStatus: http.StatusOK,
		},
		{
			name:       "filter by processing status",
			query:      "?filter=processing",
			wantCount:  1,
			wantStatus: http.StatusOK,
		},
		{
			name:       "filter returns empty list",
			query:      "?filter=failed",
			wantCount:  0,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/vector_stores/vs_001/files"+tt.query, nil)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}

			var resp map[string]interface{}
			json.Unmarshal(rr.Body.Bytes(), &resp)

			data, _ := resp["data"].([]interface{})
			if len(data) != tt.wantCount {
				t.Errorf("data length = %d, want %d", len(data), tt.wantCount)
			}
			if resp["object"] != "list" {
				t.Errorf("object = %q, want %q", resp["object"], "list")
			}
		})
	}
}

func TestHandleListFiles_VectorStoreNotFound(t *testing.T) {
	api, _, _, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	req := httptest.NewRequest("GET", "/vector_stores/vs_unknown/files", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleRemoveFile(t *testing.T) {
	api, metadata, vsFileStore, indexer, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	seedFile(t, metadata, "file_001", "test.txt")

	// Add file to vector store.
	ctx := context.Background()
	rec := NewVectorStoreFileRecord("vs_001", "file_001")
	vsFileStore.Save(ctx, rec)

	// Remove file.
	req := httptest.NewRequest("DELETE", "/vector_stores/vs_001/files/file_001", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	// Verify response.
	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["deleted"] != true {
		t.Errorf("deleted = %v, want true", resp["deleted"])
	}
	if resp["id"] != "file_001" {
		t.Errorf("id = %v, want file_001", resp["id"])
	}
	if resp["object"] != "vector_store.file.deleted" {
		t.Errorf("object = %v, want vector_store.file.deleted", resp["object"])
	}

	// Verify chunks were deleted.
	if len(indexer.deletedFiles) != 1 || indexer.deletedFiles[0] != "collection_001|file_001" {
		t.Errorf("indexer.deletedFiles = %v, want [collection_001|file_001]", indexer.deletedFiles)
	}

	// Verify VS file record was deleted.
	_, err := vsFileStore.Get(ctx, "vs_001", "file_001")
	if err == nil {
		t.Error("expected record to be deleted")
	}

	// Verify the file metadata still exists (only chunks removed, not the file itself).
	file, err := metadata.Get(ctx, "file_001")
	if err != nil {
		t.Errorf("file metadata should still exist: %v", err)
	}
	if file.ID != "file_001" {
		t.Errorf("file ID = %q, want %q", file.ID, "file_001")
	}
}

func TestHandleRemoveFile_NotFound(t *testing.T) {
	api, _, _, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	req := httptest.NewRequest("DELETE", "/vector_stores/vs_001/files/file_nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestMultiStoreIsolation(t *testing.T) {
	api, metadata, vsFileStore, indexer, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
		"vs_002": "collection_002",
	})
	mux := setupVSFilesMux(api)

	seedFile(t, metadata, "file_001", "shared.txt")

	ctx := context.Background()

	// Add same file to two different vector stores.
	rec1 := NewVectorStoreFileRecord("vs_001", "file_001")
	vsFileStore.Save(ctx, rec1)
	rec2 := NewVectorStoreFileRecord("vs_002", "file_001")
	vsFileStore.Save(ctx, rec2)

	// Remove from vs_001 only.
	req := httptest.NewRequest("DELETE", "/vector_stores/vs_001/files/file_001", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("remove from vs_001: status = %d, want %d", rr.Code, http.StatusOK)
	}

	// vs_001 record should be gone.
	_, err := vsFileStore.Get(ctx, "vs_001", "file_001")
	if err == nil {
		t.Error("expected file record to be deleted from vs_001")
	}

	// vs_002 record should still exist.
	rec, err := vsFileStore.Get(ctx, "vs_002", "file_001")
	if err != nil {
		t.Errorf("file should still exist in vs_002: %v", err)
	}
	if rec.FileID != "file_001" {
		t.Errorf("file_id = %q, want %q", rec.FileID, "file_001")
	}

	// Only chunks from collection_001 should be deleted.
	if len(indexer.deletedFiles) != 1 || indexer.deletedFiles[0] != "collection_001|file_001" {
		t.Errorf("indexer.deletedFiles = %v, want [collection_001|file_001]", indexer.deletedFiles)
	}

	// File metadata should still exist.
	file, err := metadata.Get(ctx, "file_001")
	if err != nil || file.ID != "file_001" {
		t.Errorf("file metadata should still exist: %v", err)
	}
}

func TestHandleListFiles_Pagination(t *testing.T) {
	api, metadata, vsFileStore, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		fid := fmt.Sprintf("file_%03d", i)
		seedFile(t, metadata, fid, fmt.Sprintf("%d.txt", i))
		rec := NewVectorStoreFileRecord("vs_001", fid)
		vsFileStore.Save(ctx, rec)
	}

	// Request with limit=2.
	req := httptest.NewRequest("GET", "/vector_stores/vs_001/files?limit=2", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)

	data := resp["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("data length = %d, want 2", len(data))
	}
	if resp["has_more"] != true {
		t.Errorf("has_more = %v, want true", resp["has_more"])
	}
}

// --- Batch file operation tests (T045) ---

func TestHandleCreateBatch(t *testing.T) {
	api, metadata, vsFileStore, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	seedFile(t, metadata, "file_001", "a.txt")
	seedFile(t, metadata, "file_002", "b.txt")
	seedFile(t, metadata, "file_003", "c.txt")

	body := `{"file_ids": ["file_001", "file_002", "file_003"]}`
	req := httptest.NewRequest("POST", "/vector_stores/vs_001/file_batches", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var batch FileBatch
	json.Unmarshal(rr.Body.Bytes(), &batch)

	if batch.Object != "vector_store.files_batch" {
		t.Errorf("object = %q, want %q", batch.Object, "vector_store.files_batch")
	}
	if batch.VectorStoreID != "vs_001" {
		t.Errorf("vector_store_id = %q, want %q", batch.VectorStoreID, "vs_001")
	}
	if batch.Status != "in_progress" {
		t.Errorf("status = %q, want %q", batch.Status, "in_progress")
	}
	if batch.FileCounts.Total != 3 {
		t.Errorf("total = %d, want 3", batch.FileCounts.Total)
	}
	if batch.FileCounts.InProgress != 3 {
		t.Errorf("in_progress = %d, want 3", batch.FileCounts.InProgress)
	}
	if batch.ID == "" {
		t.Error("batch ID should not be empty")
	}

	// Verify all three VS file records were created with the batch ID.
	ctx := context.Background()
	for _, fid := range []string{"file_001", "file_002", "file_003"} {
		rec, err := vsFileStore.Get(ctx, "vs_001", fid)
		if err != nil {
			t.Errorf("file %s should be in VS store: %v", fid, err)
			continue
		}
		if rec.BatchID != batch.ID {
			t.Errorf("file %s batch_id = %q, want %q", fid, rec.BatchID, batch.ID)
		}
	}
}

func TestHandleCreateBatch_PartialFailures(t *testing.T) {
	api, metadata, _, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	// Only seed file_001 and file_002; file_003 does not exist.
	seedFile(t, metadata, "file_001", "a.txt")
	seedFile(t, metadata, "file_002", "b.txt")

	body := `{"file_ids": ["file_001", "file_002", "file_003"]}`
	req := httptest.NewRequest("POST", "/vector_stores/vs_001/file_batches", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var batch FileBatch
	json.Unmarshal(rr.Body.Bytes(), &batch)

	if batch.FileCounts.Total != 3 {
		t.Errorf("total = %d, want 3", batch.FileCounts.Total)
	}
	if batch.FileCounts.InProgress != 2 {
		t.Errorf("in_progress = %d, want 2", batch.FileCounts.InProgress)
	}
	if batch.FileCounts.Failed != 1 {
		t.Errorf("failed = %d, want 1", batch.FileCounts.Failed)
	}
	// Batch should still be in_progress because some files are processing.
	if batch.Status != "in_progress" {
		t.Errorf("status = %q, want %q", batch.Status, "in_progress")
	}
}

func TestHandleCreateBatch_AllFailed(t *testing.T) {
	api, _, _, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	// No files seeded, so all will fail.
	body := `{"file_ids": ["file_x", "file_y"]}`
	req := httptest.NewRequest("POST", "/vector_stores/vs_001/file_batches", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var batch FileBatch
	json.Unmarshal(rr.Body.Bytes(), &batch)

	if batch.FileCounts.Total != 2 {
		t.Errorf("total = %d, want 2", batch.FileCounts.Total)
	}
	if batch.FileCounts.Failed != 2 {
		t.Errorf("failed = %d, want 2", batch.FileCounts.Failed)
	}
	if batch.FileCounts.InProgress != 0 {
		t.Errorf("in_progress = %d, want 0", batch.FileCounts.InProgress)
	}
	// All failed, no in-progress, so batch should be completed.
	if batch.Status != "completed" {
		t.Errorf("status = %q, want %q", batch.Status, "completed")
	}
}

func TestHandleCreateBatch_EmptyFileIDs(t *testing.T) {
	api, _, _, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	body := `{"file_ids": []}`
	req := httptest.NewRequest("POST", "/vector_stores/vs_001/file_batches", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateBatch_VectorStoreNotFound(t *testing.T) {
	api, _, _, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	body := `{"file_ids": ["file_001"]}`
	req := httptest.NewRequest("POST", "/vector_stores/vs_unknown/file_batches", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleGetBatch(t *testing.T) {
	api, metadata, vsFileStore, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	seedFile(t, metadata, "file_001", "a.txt")
	seedFile(t, metadata, "file_002", "b.txt")

	// Create a batch via the handler.
	body := `{"file_ids": ["file_001", "file_002"]}`
	req := httptest.NewRequest("POST", "/vector_stores/vs_001/file_batches", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var created FileBatch
	json.Unmarshal(rr.Body.Bytes(), &created)

	// Wait for background ingestion goroutines to settle.
	time.Sleep(100 * time.Millisecond)

	// Simulate that file_001 completed ingestion.
	ctx := context.Background()
	rec, _ := vsFileStore.Get(ctx, "vs_001", "file_001")
	rec.Status = FileStatusCompleted
	rec.ChunkCount = 5
	vsFileStore.Save(ctx, rec)

	// Reset file_002 back to processing (pipeline may have set it to failed).
	rec2, _ := vsFileStore.Get(ctx, "vs_001", "file_002")
	rec2.Status = FileStatusProcessing
	vsFileStore.Save(ctx, rec2)

	// Get batch status.
	req = httptest.NewRequest("GET", "/vector_stores/vs_001/file_batches/"+created.ID, nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var batch FileBatch
	json.Unmarshal(rr.Body.Bytes(), &batch)

	if batch.FileCounts.Total != 2 {
		t.Errorf("total = %d, want 2", batch.FileCounts.Total)
	}
	if batch.FileCounts.Completed != 1 {
		t.Errorf("completed = %d, want 1", batch.FileCounts.Completed)
	}
	if batch.FileCounts.InProgress != 1 {
		t.Errorf("in_progress = %d, want 1", batch.FileCounts.InProgress)
	}
	// Still in progress because file_002 is not done.
	if batch.Status != "in_progress" {
		t.Errorf("status = %q, want %q", batch.Status, "in_progress")
	}
}

func TestHandleGetBatch_AllCompleted(t *testing.T) {
	api, metadata, vsFileStore, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	seedFile(t, metadata, "file_001", "a.txt")
	seedFile(t, metadata, "file_002", "b.txt")

	// Create batch.
	body := `{"file_ids": ["file_001", "file_002"]}`
	req := httptest.NewRequest("POST", "/vector_stores/vs_001/file_batches", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var created FileBatch
	json.Unmarshal(rr.Body.Bytes(), &created)

	// Wait for background ingestion goroutines to finish (they will fail since
	// there is no file content in the test file store, setting status to "failed").
	time.Sleep(100 * time.Millisecond)

	// Simulate both files completing (overwrite the failed status set by the pipeline).
	ctx := context.Background()
	for _, fid := range []string{"file_001", "file_002"} {
		rec, _ := vsFileStore.Get(ctx, "vs_001", fid)
		rec.Status = FileStatusCompleted
		vsFileStore.Save(ctx, rec)
	}

	// Get batch status.
	req = httptest.NewRequest("GET", "/vector_stores/vs_001/file_batches/"+created.ID, nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var batch FileBatch
	json.Unmarshal(rr.Body.Bytes(), &batch)

	if batch.FileCounts.Completed != 2 {
		t.Errorf("completed = %d, want 2", batch.FileCounts.Completed)
	}
	if batch.Status != "completed" {
		t.Errorf("status = %q, want %q", batch.Status, "completed")
	}
}

func TestHandleGetBatch_NotFound(t *testing.T) {
	api, _, _, _, _ := newTestVSFilesAPI(map[string]string{
		"vs_001": "collection_001",
	})
	mux := setupVSFilesMux(api)

	req := httptest.NewRequest("GET", "/vector_stores/vs_001/file_batches/batch_nonexistent", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
