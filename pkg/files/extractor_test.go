package files

import (
	"context"
	"strings"
	"testing"
)

func TestPassthroughExtractor_Extract(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantText   string
		wantMethod string
		wantErr    string
	}{
		{
			name:       "plain text content",
			content:    "Hello, world!",
			wantText:   "Hello, world!",
			wantMethod: "passthrough",
		},
		{
			name:       "markdown content",
			content:    "# Title\n\nSome paragraph.",
			wantText:   "# Title\n\nSome paragraph.",
			wantMethod: "passthrough",
		},
		{
			name:    "empty content returns error",
			content: "",
			wantErr: "no extractable content found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := NewPassthroughExtractor()
			result, err := ext.Extract(
				context.Background(),
				"test.txt",
				"text/plain",
				strings.NewReader(tt.content),
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

func TestPassthroughExtractor_SupportedFormats(t *testing.T) {
	ext := NewPassthroughExtractor()
	formats := ext.SupportedFormats()

	expected := map[string]bool{
		"text/plain":       true,
		"text/markdown":    true,
		"text/csv":         true,
		"text/html":        true,
		"application/json": true,
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

func TestIsPassthroughFormat(t *testing.T) {
	tests := []struct {
		mimeType string
		want     bool
	}{
		{"text/plain", true},
		{"text/markdown", true},
		{"text/csv", true},
		{"text/html", true},
		{"application/json", true},
		{"application/pdf", false},
		{"image/png", false},
		{"application/octet-stream", false},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			got := IsPassthroughFormat(tt.mimeType)
			if got != tt.want {
				t.Errorf("IsPassthroughFormat(%q) = %v, want %v", tt.mimeType, got, tt.want)
			}
		})
	}
}

func TestIsComplexFormat(t *testing.T) {
	tests := []struct {
		mimeType string
		want     bool
	}{
		{"application/pdf", true},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", true},
		{"application/vnd.openxmlformats-officedocument.presentationml.presentation", true},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true},
		{"image/png", true},
		{"image/jpeg", true},
		{"image/tiff", true},
		{"image/gif", true},
		{"image/webp", true},
		{"text/plain", false},
		{"text/markdown", false},
		{"application/json", false},
		{"application/octet-stream", false},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			got := IsComplexFormat(tt.mimeType)
			if got != tt.want {
				t.Errorf("IsComplexFormat(%q) = %v, want %v", tt.mimeType, got, tt.want)
			}
		})
	}
}
