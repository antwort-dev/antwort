package files

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// DoclingExtractor calls docling-serve's /v1/convert/file endpoint
// to extract structured text from complex document formats.
type DoclingExtractor struct {
	url    string
	apiKey string
	ocr    bool
	client *http.Client
}

// NewDoclingExtractor creates a Docling content extractor.
func NewDoclingExtractor(url, apiKey string, ocr bool, timeout time.Duration) *DoclingExtractor {
	return &DoclingExtractor{
		url:    url,
		apiKey: apiKey,
		ocr:    ocr,
		client: &http.Client{Timeout: timeout},
	}
}

func (d *DoclingExtractor) Extract(ctx context.Context, filename, mimeType string, content io.Reader) (*ExtractionResult, error) {
	data, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("reading file content: %w", err)
	}

	// Build multipart request.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("files", filename)
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("writing form file: %w", err)
	}

	// Request Markdown output.
	_ = writer.WriteField("to_formats", "md")

	// OCR toggle.
	if d.ocr {
		_ = writer.WriteField("do_ocr", "true")
	} else {
		_ = writer.WriteField("do_ocr", "false")
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.url+"/v1/convert/file", &buf)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if d.apiKey != "" {
		req.Header.Set("X-Api-Key", d.apiKey)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("extraction service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("extraction service authentication failed")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("extraction service returned status %d: %s", resp.StatusCode, string(body))
	}

	var doclingResp struct {
		MDContent  string  `json:"md_content"`
		Status     string  `json:"status"`
		ProcTime   float64 `json:"processing_time"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doclingResp); err != nil {
		return nil, fmt.Errorf("parsing extraction response: %w", err)
	}

	if doclingResp.MDContent == "" {
		return nil, fmt.Errorf("no extractable content found")
	}

	return &ExtractionResult{
		Text:   doclingResp.MDContent,
		Method: "docling",
	}, nil
}

func (d *DoclingExtractor) SupportedFormats() []string {
	return []string{
		"application/pdf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"text/html",
		"image/png",
		"image/jpeg",
		"image/tiff",
		"image/gif",
		"image/webp",
	}
}
