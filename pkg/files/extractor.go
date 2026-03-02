package files

import (
	"context"
	"fmt"
	"io"
)

// ContentExtractor converts file content into structured text.
type ContentExtractor interface {
	// Extract processes file content and returns structured text.
	Extract(ctx context.Context, filename, mimeType string, content io.Reader) (*ExtractionResult, error)

	// SupportedFormats returns MIME types this extractor handles.
	SupportedFormats() []string
}

// PassthroughExtractor handles plain text, Markdown, and CSV files
// by reading the content directly without an external service.
type PassthroughExtractor struct{}

// NewPassthroughExtractor creates a passthrough extractor for simple text formats.
func NewPassthroughExtractor() *PassthroughExtractor {
	return &PassthroughExtractor{}
}

func (p *PassthroughExtractor) Extract(_ context.Context, _, _ string, content io.Reader) (*ExtractionResult, error) {
	data, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("reading content: %w", err)
	}
	text := string(data)
	if text == "" {
		return nil, fmt.Errorf("no extractable content found")
	}
	return &ExtractionResult{
		Text:   text,
		Method: "passthrough",
	}, nil
}

func (p *PassthroughExtractor) SupportedFormats() []string {
	return []string{
		"text/plain",
		"text/markdown",
		"text/csv",
		"text/html",
		"application/json",
	}
}

// IsPassthroughFormat checks whether a MIME type can be handled by the passthrough extractor.
func IsPassthroughFormat(mimeType string) bool {
	for _, f := range (&PassthroughExtractor{}).SupportedFormats() {
		if f == mimeType {
			return true
		}
	}
	return false
}

// IsComplexFormat checks whether a MIME type requires an external extraction service.
func IsComplexFormat(mimeType string) bool {
	switch mimeType {
	case "application/pdf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"image/png", "image/jpeg", "image/tiff", "image/gif", "image/webp":
		return true
	}
	return false
}
