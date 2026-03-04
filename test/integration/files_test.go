package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rhuss/antwort/pkg/files"
)

// setupFilesServer creates an httptest.Server with the Files API routes wired in,
// using in-memory stores and no embedder/indexer.
func setupFilesServer(t *testing.T) *httptest.Server {
	t.Helper()

	provider, err := files.New(map[string]interface{}{
		"store_type": "memory",
	}, files.ProviderDeps{})
	if err != nil {
		t.Fatalf("creating files provider: %v", err)
	}

	mux := http.NewServeMux()
	for _, route := range provider.Routes() {
		pattern := fmt.Sprintf("%s /v1%s", route.Method, route.Pattern)
		mux.HandleFunc(pattern, route.Handler)
	}

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// uploadFile creates a multipart upload request and sends it to the server.
func uploadFile(t *testing.T, baseURL, filename, purpose, content string) map[string]any {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add the file part.
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("creating form file: %v", err)
	}
	part.Write([]byte(content))

	// Add the purpose field.
	writer.WriteField("purpose", purpose)
	writer.Close()

	resp, err := http.Post(baseURL+"/v1/files", writer.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("POST /v1/files: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	decodeJSON(t, resp, &result)
	return result
}

// TestFileUpload verifies that uploading a file returns correct metadata.
func TestFileUpload(t *testing.T) {
	srv := setupFilesServer(t)

	result := uploadFile(t, srv.URL, "test.txt", "assistants", "hello world")

	if result["object"] != "file" {
		t.Errorf("object = %v, want 'file'", result["object"])
	}
	if result["filename"] != "test.txt" {
		t.Errorf("filename = %v, want 'test.txt'", result["filename"])
	}
	if result["purpose"] != "assistants" {
		t.Errorf("purpose = %v, want 'assistants'", result["purpose"])
	}
	if result["id"] == nil || result["id"] == "" {
		t.Error("file ID is empty")
	}
	// bytes should match content length.
	if b, ok := result["bytes"].(float64); ok {
		if int(b) != len("hello world") {
			t.Errorf("bytes = %v, want %d", b, len("hello world"))
		}
	}
}

// TestFileList verifies listing files returns uploaded files.
func TestFileList(t *testing.T) {
	srv := setupFilesServer(t)

	// Upload two files.
	uploadFile(t, srv.URL, "a.txt", "assistants", "content a")
	uploadFile(t, srv.URL, "b.txt", "assistants", "content b")

	resp := getURL(t, srv.URL+"/v1/files")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var listResult map[string]any
	decodeJSON(t, resp, &listResult)

	data, ok := listResult["data"].([]any)
	if !ok {
		t.Fatal("expected 'data' array in list response")
	}
	if len(data) < 2 {
		t.Errorf("expected at least 2 files, got %d", len(data))
	}
}

// TestFileGet verifies retrieving a specific file by ID.
func TestFileGet(t *testing.T) {
	srv := setupFilesServer(t)

	uploaded := uploadFile(t, srv.URL, "get-test.txt", "assistants", "get me")
	fileID := uploaded["id"].(string)

	resp := getURL(t, srv.URL+"/v1/files/"+fileID)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var file map[string]any
	decodeJSON(t, resp, &file)

	if file["id"] != fileID {
		t.Errorf("id = %v, want %v", file["id"], fileID)
	}
	if file["filename"] != "get-test.txt" {
		t.Errorf("filename = %v, want 'get-test.txt'", file["filename"])
	}
}

// TestFileGetNotFound verifies that requesting a non-existent file returns 404.
func TestFileGetNotFound(t *testing.T) {
	srv := setupFilesServer(t)

	resp := getURL(t, srv.URL+"/v1/files/file_nonexistent")
	if resp.StatusCode != http.StatusNotFound {
		body := readBody(t, resp)
		t.Fatalf("expected 404, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

// TestFileDelete verifies deleting a file.
func TestFileDelete(t *testing.T) {
	srv := setupFilesServer(t)

	uploaded := uploadFile(t, srv.URL, "delete-me.txt", "assistants", "deletable")
	fileID := uploaded["id"].(string)

	resp := deleteURL(t, srv.URL+"/v1/files/"+fileID)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result map[string]any
	decodeJSON(t, resp, &result)

	if result["deleted"] != true {
		t.Errorf("deleted = %v, want true", result["deleted"])
	}

	// Verify the file is gone.
	resp2 := getURL(t, srv.URL+"/v1/files/"+fileID)
	if resp2.StatusCode != http.StatusNotFound {
		body := readBody(t, resp2)
		t.Fatalf("expected 404 after delete, got %d: %s", resp2.StatusCode, body)
	}
	resp2.Body.Close()
}

// TestFileUploadInvalidPurpose verifies that an invalid purpose returns 400.
func TestFileUploadInvalidPurpose(t *testing.T) {
	srv := setupFilesServer(t)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("hello"))
	writer.WriteField("purpose", "invalid_purpose")
	writer.Close()

	resp, err := http.Post(srv.URL+"/v1/files", writer.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("POST /v1/files: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got %d: %s", resp.StatusCode, string(body))
	}
}

// TestFileGetContent verifies that file content can be retrieved.
func TestFileGetContent(t *testing.T) {
	srv := setupFilesServer(t)

	content := "the actual file content"
	uploaded := uploadFile(t, srv.URL, "content-test.txt", "assistants", content)
	fileID := uploaded["id"].(string)

	resp := getURL(t, srv.URL+"/v1/files/"+fileID+"/content")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading content: %v", err)
	}
	if string(body) != content {
		t.Errorf("content = %q, want %q", string(body), content)
	}

	// Verify Content-Disposition header.
	cd := resp.Header.Get("Content-Disposition")
	if cd == "" {
		t.Error("expected Content-Disposition header")
	}
}

// TestFileUploadMIMEDetection verifies MIME type detection from filename.
func TestFileUploadMIMEDetection(t *testing.T) {
	srv := setupFilesServer(t)

	tests := []struct {
		filename string
		wantMIME string
	}{
		{"doc.pdf", "application/pdf"},
		{"data.json", "application/json"},
		{"notes.md", "text/markdown"},
		{"image.png", "image/png"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := uploadFile(t, srv.URL, tt.filename, "assistants", "content")
			gotMIME, _ := result["mime_type"].(string)
			if gotMIME != tt.wantMIME {
				// The JSON field might be named differently. Check raw JSON.
				raw, _ := json.Marshal(result)
				t.Logf("raw response: %s", string(raw))
				t.Errorf("mime_type for %s = %q, want %q", tt.filename, gotMIME, tt.wantMIME)
			}
		})
	}
}
