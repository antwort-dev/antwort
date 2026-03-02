package files

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/auth"
)

// newTestServer sets up an HTTP test server with the FilesAPI handlers registered.
func newTestServer(t *testing.T) (*httptest.Server, *MemoryFileStore, *MemoryMetadataStore, *MemoryVectorStoreFileStore) {
	t.Helper()

	fileStore := NewMemoryFileStore()
	metadata := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()
	indexer := newStubIndexer()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	api := &FilesAPI{
		fileStore:     fileStore,
		metadata:      metadata,
		vsFileStore:   vsFileStore,
		indexer:       indexer,
		maxUploadSize: 1024 * 1024, // 1 MB for tests
		logger:        logger,
		vsCollectionLookup: func(vsID string) (string, error) {
			return "coll-" + vsID, nil
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/files", api.handleUpload)
	mux.HandleFunc("GET /v1/files", api.handleListFiles)
	mux.HandleFunc("GET /v1/files/{file_id}", api.handleGetFile)
	mux.HandleFunc("GET /v1/files/{file_id}/content", api.handleGetContent)
	mux.HandleFunc("DELETE /v1/files/{file_id}", api.handleDeleteFile)

	// Wrap with auth middleware that injects a test identity.
	handler := authMiddleware("test-user", mux)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server, fileStore, metadata, vsFileStore
}

// authMiddleware wraps a handler to inject a test user identity.
func authMiddleware(userID string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.SetIdentity(r.Context(), &auth.Identity{Subject: userID})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// uploadFile performs a multipart file upload to the test server.
func uploadFile(t *testing.T, serverURL, filename, purpose, content string) *http.Response {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("creating form file: %v", err)
	}
	part.Write([]byte(content))

	if purpose != "" {
		writer.WriteField("purpose", purpose)
	}
	writer.Close()

	resp, err := http.Post(serverURL+"/v1/files", writer.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("POST /v1/files: %v", err)
	}
	return resp
}

func TestFilesAPI_Upload(t *testing.T) {
	server, _, _, _ := newTestServer(t)

	resp := uploadFile(t, server.URL, "test.txt", "assistants", "hello world")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(body))
	}

	var file File
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if file.ID == "" {
		t.Error("expected non-empty file ID")
	}
	if file.Filename != "test.txt" {
		t.Errorf("filename: got %q, want %q", file.Filename, "test.txt")
	}
	if file.Purpose != "assistants" {
		t.Errorf("purpose: got %q, want %q", file.Purpose, "assistants")
	}
	if file.Bytes != 11 {
		t.Errorf("bytes: got %d, want %d", file.Bytes, 11)
	}
	if file.Object != "file" {
		t.Errorf("object: got %q, want %q", file.Object, "file")
	}
	if file.Status != FileStatusUploaded {
		t.Errorf("status: got %q, want %q", file.Status, FileStatusUploaded)
	}
}

func TestFilesAPI_UploadMissingFile(t *testing.T) {
	server, _, _, _ := newTestServer(t)

	// Upload with no file field, only purpose.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("purpose", "assistants")
	writer.Close()

	resp, err := http.Post(server.URL+"/v1/files", writer.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestFilesAPI_UploadMissingPurpose(t *testing.T) {
	server, _, _, _ := newTestServer(t)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("hello"))
	writer.Close()

	resp, err := http.Post(server.URL+"/v1/files", writer.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestFilesAPI_UploadInvalidPurpose(t *testing.T) {
	server, _, _, _ := newTestServer(t)

	resp := uploadFile(t, server.URL, "test.txt", "invalid-purpose", "hello")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid purpose, got %d", resp.StatusCode)
	}
}

func TestFilesAPI_ListFiles(t *testing.T) {
	server, _, _, _ := newTestServer(t)

	// Upload multiple files.
	for i := 0; i < 3; i++ {
		resp := uploadFile(t, server.URL, fmt.Sprintf("file-%d.txt", i), "assistants", "content")
		resp.Body.Close()
	}

	resp, err := http.Get(server.URL + "/v1/files")
	if err != nil {
		t.Fatalf("GET /v1/files: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var list FileList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	if list.Object != "list" {
		t.Errorf("object: got %q, want %q", list.Object, "list")
	}
	if len(list.Data) != 3 {
		t.Errorf("data count: got %d, want %d", len(list.Data), 3)
	}
}

func TestFilesAPI_GetFile(t *testing.T) {
	server, _, _, _ := newTestServer(t)

	// Upload a file first.
	uploadResp := uploadFile(t, server.URL, "test.txt", "assistants", "hello world")
	var uploaded File
	json.NewDecoder(uploadResp.Body).Decode(&uploaded)
	uploadResp.Body.Close()

	// Retrieve it.
	resp, err := http.Get(server.URL + "/v1/files/" + uploaded.ID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var file File
	json.NewDecoder(resp.Body).Decode(&file)
	if file.ID != uploaded.ID {
		t.Errorf("id: got %q, want %q", file.ID, uploaded.ID)
	}
	if file.Filename != "test.txt" {
		t.Errorf("filename: got %q, want %q", file.Filename, "test.txt")
	}
}

func TestFilesAPI_GetFileMissing(t *testing.T) {
	server, _, _, _ := newTestServer(t)

	resp, err := http.Get(server.URL + "/v1/files/nonexistent")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestFilesAPI_GetContent(t *testing.T) {
	server, _, _, _ := newTestServer(t)

	content := "hello world file content"
	uploadResp := uploadFile(t, server.URL, "test.txt", "assistants", content)
	var uploaded File
	json.NewDecoder(uploadResp.Body).Decode(&uploaded)
	uploadResp.Body.Close()

	resp, err := http.Get(server.URL + "/v1/files/" + uploaded.ID + "/content")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != content {
		t.Errorf("content: got %q, want %q", string(body), content)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/plain" {
		t.Errorf("content-type: got %q, want %q", ct, "text/plain")
	}

	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "test.txt") {
		t.Errorf("content-disposition should contain filename, got: %q", cd)
	}
}

func TestFilesAPI_GetContentMissing(t *testing.T) {
	server, _, _, _ := newTestServer(t)

	resp, err := http.Get(server.URL + "/v1/files/nonexistent/content")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestFilesAPI_DeleteFile(t *testing.T) {
	server, _, _, _ := newTestServer(t)

	// Upload and then delete.
	uploadResp := uploadFile(t, server.URL, "test.txt", "assistants", "hello")
	var uploaded File
	json.NewDecoder(uploadResp.Body).Decode(&uploaded)
	uploadResp.Body.Close()

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/v1/files/"+uploaded.ID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["deleted"] != true {
		t.Error("expected deleted=true")
	}
	if result["id"] != uploaded.ID {
		t.Errorf("id: got %v, want %v", result["id"], uploaded.ID)
	}

	// Verify file is gone.
	getResp, err := http.Get(server.URL + "/v1/files/" + uploaded.ID)
	if err != nil {
		t.Fatalf("GET after delete: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getResp.StatusCode)
	}
}

func TestFilesAPI_DeleteWithCascade(t *testing.T) {
	server, _, _, vsFileStore := newTestServer(t)

	// Upload a file.
	uploadResp := uploadFile(t, server.URL, "test.txt", "assistants", "hello")
	var uploaded File
	json.NewDecoder(uploadResp.Body).Decode(&uploaded)
	uploadResp.Body.Close()

	// Simulate adding the file to a vector store.
	rec := NewVectorStoreFileRecord("vs-1", uploaded.ID)
	vsFileStore.Save(context.Background(), rec)

	// Delete the file.
	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/v1/files/"+uploaded.ID, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()

	// Verify the VS file record was also deleted.
	_, err = vsFileStore.Get(context.Background(), "vs-1", uploaded.ID)
	if err == nil {
		t.Error("expected VS file record to be deleted after file cascade delete")
	}
}

func TestFilesAPI_DeleteMissing(t *testing.T) {
	server, _, _, _ := newTestServer(t)

	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/v1/files/nonexistent", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestFilesAPI_CrossUserAccessReturns404(t *testing.T) {
	fileStore := NewMemoryFileStore()
	metadata := NewMemoryMetadataStore()
	vsFileStore := NewMemoryVectorStoreFileStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	api := &FilesAPI{
		fileStore:     fileStore,
		metadata:      metadata,
		vsFileStore:   vsFileStore,
		indexer:       newStubIndexer(),
		maxUploadSize: 1024 * 1024,
		logger:        logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/files/{file_id}", api.handleGetFile)
	mux.HandleFunc("GET /v1/files/{file_id}/content", api.handleGetContent)

	// Upload a file as user-A.
	file := NewFile("file-secret", "secret.txt", "text/plain", "assistants", "user-A", 5)
	metadata.Save(context.Background(), file)
	fileStore.Store(context.Background(), file.ID, strings.NewReader("hello"))

	// Try to access as user-B.
	handlerB := authMiddleware("user-B", mux)
	serverB := httptest.NewServer(handlerB)
	defer serverB.Close()

	// GET metadata.
	resp, err := http.Get(serverB.URL + "/v1/files/" + file.ID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("cross-user metadata access: expected 404, got %d", resp.StatusCode)
	}

	// GET content.
	resp2, err := http.Get(serverB.URL + "/v1/files/" + file.ID + "/content")
	if err != nil {
		t.Fatalf("GET content: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("cross-user content access: expected 404, got %d", resp2.StatusCode)
	}
}

func TestFilesAPI_DetectMIME(t *testing.T) {
	tests := []struct {
		filename   string
		headerType string
		want       string
	}{
		{"test.pdf", "", "application/pdf"},
		{"test.txt", "", "text/plain"},
		{"test.md", "", "text/markdown"},
		{"test.json", "", "application/json"},
		{"test.csv", "", "text/csv"},
		{"test.html", "", "text/html"},
		{"test.png", "", "image/png"},
		{"test.jpg", "", "image/jpeg"},
		{"test.gif", "", "image/gif"},
		{"test.webp", "", "image/webp"},
		{"test.docx", "", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"test.pptx", "", "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
		{"test.xlsx", "", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"test.unknown", "", "application/octet-stream"},
		{"test.txt", "image/png", "image/png"}, // header takes priority
		{"test.txt", "application/octet-stream", "text/plain"}, // octet-stream falls through to extension
	}

	for _, tt := range tests {
		t.Run(tt.filename+"/"+tt.headerType, func(t *testing.T) {
			got := detectMIME(tt.filename, tt.headerType)
			if got != tt.want {
				t.Errorf("detectMIME(%q, %q) = %q, want %q", tt.filename, tt.headerType, got, tt.want)
			}
		})
	}
}
