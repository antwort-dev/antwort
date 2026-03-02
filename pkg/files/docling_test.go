package files

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDoclingExtractor_Extract(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     string
		ocr        bool
		handler    http.HandlerFunc
		wantText   string
		wantMethod string
		wantErr    string
	}{
		{
			name: "successful markdown extraction",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
					t.Errorf("expected multipart/form-data content type")
				}
				json.NewEncoder(w).Encode(map[string]any{
					"md_content":      "# Extracted Title\n\nSome content.",
					"status":          "success",
					"processing_time": 1.23,
				})
			},
			wantText:   "# Extracted Title\n\nSome content.",
			wantMethod: "docling",
		},
		{
			name:   "ocr enabled sends do_ocr true",
			ocr:    true,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseMultipartForm(10 << 20); err != nil {
					t.Fatalf("parsing multipart form: %v", err)
				}
				ocrVal := r.FormValue("do_ocr")
				if ocrVal != "true" {
					t.Errorf("expected do_ocr=true, got %q", ocrVal)
				}
				json.NewEncoder(w).Encode(map[string]any{
					"md_content": "OCR content",
				})
			},
			wantText:   "OCR content",
			wantMethod: "docling",
		},
		{
			name: "ocr disabled sends do_ocr false",
			ocr:  false,
			handler: func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseMultipartForm(10 << 20); err != nil {
					t.Fatalf("parsing multipart form: %v", err)
				}
				ocrVal := r.FormValue("do_ocr")
				if ocrVal != "false" {
					t.Errorf("expected do_ocr=false, got %q", ocrVal)
				}
				json.NewEncoder(w).Encode(map[string]any{
					"md_content": "Non-OCR content",
				})
			},
			wantText:   "Non-OCR content",
			wantMethod: "docling",
		},
		{
			name:   "api key authentication header",
			apiKey: "test-secret-key",
			handler: func(w http.ResponseWriter, r *http.Request) {
				got := r.Header.Get("X-Api-Key")
				if got != "test-secret-key" {
					t.Errorf("expected X-Api-Key=test-secret-key, got %q", got)
				}
				json.NewEncoder(w).Encode(map[string]any{
					"md_content": "Authenticated content",
				})
			},
			wantText:   "Authenticated content",
			wantMethod: "docling",
		},
		{
			name: "no api key omits header",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("X-Api-Key"); got != "" {
					t.Errorf("expected no X-Api-Key header, got %q", got)
				}
				json.NewEncoder(w).Encode(map[string]any{
					"md_content": "Public content",
				})
			},
			wantText:   "Public content",
			wantMethod: "docling",
		},
		{
			name: "HTTP 500 error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("internal server error"))
			},
			wantErr: "extraction service returned status 500",
		},
		{
			name: "HTTP 504 gateway timeout",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusGatewayTimeout)
				w.Write([]byte("gateway timeout"))
			},
			wantErr: "extraction service returned status 504",
		},
		{
			name: "HTTP 401 unauthorized",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			wantErr: "extraction service authentication failed",
		},
		{
			name: "empty content response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{
					"md_content": "",
					"status":     "success",
				})
			},
			wantErr: "no extractable content found",
		},
		{
			name: "to_formats field set to md",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseMultipartForm(10 << 20); err != nil {
					t.Fatalf("parsing multipart form: %v", err)
				}
				toFormats := r.FormValue("to_formats")
				if toFormats != "md" {
					t.Errorf("expected to_formats=md, got %q", toFormats)
				}
				json.NewEncoder(w).Encode(map[string]any{
					"md_content": "Markdown output",
				})
			},
			wantText:   "Markdown output",
			wantMethod: "docling",
		},
		{
			name: "filename sent in multipart form",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseMultipartForm(10 << 20); err != nil {
					t.Fatalf("parsing multipart form: %v", err)
				}
				file, header, err := r.FormFile("files")
				if err != nil {
					t.Fatalf("getting form file: %v", err)
				}
				defer file.Close()
				if header.Filename != "report.pdf" {
					t.Errorf("expected filename report.pdf, got %q", header.Filename)
				}
				json.NewEncoder(w).Encode(map[string]any{
					"md_content": "File content",
				})
			},
			wantText:   "File content",
			wantMethod: "docling",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			ext := NewDoclingExtractor(srv.URL, tt.apiKey, tt.ocr, 5*time.Second)
			result, err := ext.Extract(
				context.Background(),
				"report.pdf",
				"application/pdf",
				strings.NewReader("fake pdf content"),
			)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Text != tt.wantText {
				t.Errorf("expected text %q, got %q", tt.wantText, result.Text)
			}
			if result.Method != tt.wantMethod {
				t.Errorf("expected method %q, got %q", tt.wantMethod, result.Method)
			}
		})
	}
}

func TestDoclingExtractor_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow server that exceeds the client timeout.
		select {
		case <-time.After(2 * time.Second):
			json.NewEncoder(w).Encode(map[string]any{"md_content": "late"})
		case <-r.Context().Done():
			return
		}
	}))
	defer srv.Close()

	ext := NewDoclingExtractor(srv.URL, "", false, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := ext.Extract(ctx, "doc.pdf", "application/pdf", strings.NewReader("data"))
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Errorf("expected error containing 'unreachable', got %q", err.Error())
	}
}

func TestDoclingExtractor_ConnectionRefused(t *testing.T) {
	// Use a URL where nothing is listening.
	ext := NewDoclingExtractor("http://127.0.0.1:1", "", false, 2*time.Second)

	_, err := ext.Extract(
		context.Background(),
		"doc.pdf",
		"application/pdf",
		strings.NewReader("data"),
	)
	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Errorf("expected error containing 'unreachable', got %q", err.Error())
	}
}

func TestDoclingExtractor_SupportedFormats(t *testing.T) {
	ext := NewDoclingExtractor("http://localhost", "", false, time.Second)
	formats := ext.SupportedFormats()

	expected := map[string]bool{
		"application/pdf": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
		"text/html":   true,
		"image/png":   true,
		"image/jpeg":  true,
		"image/tiff":  true,
		"image/gif":   true,
		"image/webp":  true,
	}

	for _, f := range formats {
		if !expected[f] {
			t.Errorf("unexpected format %q in SupportedFormats", f)
		}
		delete(expected, f)
	}
	for f := range expected {
		t.Errorf("missing format %q from SupportedFormats", f)
	}
}

func TestDoclingExtractor_FileContentSent(t *testing.T) {
	fileContent := "this is the raw file content for extraction"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parsing multipart form: %v", err)
		}
		file, _, err := r.FormFile("files")
		if err != nil {
			t.Fatalf("getting form file: %v", err)
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("reading form file: %v", err)
		}
		if string(data) != fileContent {
			t.Errorf("expected file content %q, got %q", fileContent, string(data))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"md_content": "extracted",
		})
	}))
	defer srv.Close()

	ext := NewDoclingExtractor(srv.URL, "", false, 5*time.Second)
	result, err := ext.Extract(
		context.Background(),
		"test.pdf",
		"application/pdf",
		strings.NewReader(fileContent),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text != "extracted" {
		t.Errorf("expected text 'extracted', got %q", result.Text)
	}
}
