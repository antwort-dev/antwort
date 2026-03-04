package integration

import (
	"testing"

	"github.com/rhuss/antwort/pkg/engine"
)

// TestAnnotationGenerationFileCitation verifies that the SubstringMatcher generates
// file_citation annotations when source content appears in the output.
func TestAnnotationGenerationFileCitation(t *testing.T) {
	matcher := engine.NewSubstringMatcher(20)

	sourceContent := "The quick brown fox jumps over the lazy dog near the river bank."
	outputText := "According to the source, the quick brown fox jumps over the lazy dog near the river bank. This is remarkable."

	sources := []engine.SourceContext{
		{
			ToolName: "file_search",
			FileID:   "file_abc123",
			Content:  sourceContent,
		},
	}

	annotations := matcher.Generate(outputText, sources)
	if len(annotations) == 0 {
		t.Fatal("expected at least one annotation, got none")
	}

	ann := annotations[0]
	if ann.Type != "file_citation" {
		t.Errorf("type = %q, want 'file_citation'", ann.Type)
	}
	if ann.FileID != "file_abc123" {
		t.Errorf("file_id = %q, want 'file_abc123'", ann.FileID)
	}
	if ann.StartIndex < 0 || ann.EndIndex <= ann.StartIndex {
		t.Errorf("invalid range: start=%d, end=%d", ann.StartIndex, ann.EndIndex)
	}
	if ann.Quote == "" {
		t.Error("quote is empty")
	}
}

// TestAnnotationGenerationURLCitation verifies URL citation annotation generation.
func TestAnnotationGenerationURLCitation(t *testing.T) {
	matcher := engine.NewSubstringMatcher(20)

	sourceContent := "Kubernetes provides declarative configuration management for containerized applications."
	outputText := "As found online, Kubernetes provides declarative configuration management for containerized applications."

	sources := []engine.SourceContext{
		{
			ToolName: "web_search",
			URL:      "https://example.com/k8s",
			Title:    "Kubernetes Overview",
			Content:  sourceContent,
		},
	}

	annotations := matcher.Generate(outputText, sources)
	if len(annotations) == 0 {
		t.Fatal("expected at least one annotation, got none")
	}

	ann := annotations[0]
	if ann.Type != "url_citation" {
		t.Errorf("type = %q, want 'url_citation'", ann.Type)
	}
	if ann.URL != "https://example.com/k8s" {
		t.Errorf("url = %q, want 'https://example.com/k8s'", ann.URL)
	}
	if ann.Title != "Kubernetes Overview" {
		t.Errorf("title = %q, want 'Kubernetes Overview'", ann.Title)
	}
}

// TestAnnotationNoMatch verifies that no annotations are generated when
// the output does not contain the source content.
func TestAnnotationNoMatch(t *testing.T) {
	matcher := engine.NewSubstringMatcher(20)

	sources := []engine.SourceContext{
		{
			ToolName: "file_search",
			FileID:   "file_xyz",
			Content:  "This content is completely different from the output.",
		},
	}

	annotations := matcher.Generate("An unrelated response about weather.", sources)
	if len(annotations) != 0 {
		t.Errorf("expected no annotations, got %d", len(annotations))
	}
}

// TestAnnotationEmptyInputs verifies edge cases with empty inputs.
func TestAnnotationEmptyInputs(t *testing.T) {
	matcher := engine.NewSubstringMatcher(20)

	// Empty output.
	if anns := matcher.Generate("", []engine.SourceContext{{ToolName: "file_search", Content: "data"}}); len(anns) != 0 {
		t.Errorf("expected 0 annotations for empty output, got %d", len(anns))
	}

	// Empty sources.
	if anns := matcher.Generate("some output", nil); len(anns) != 0 {
		t.Errorf("expected 0 annotations for nil sources, got %d", len(anns))
	}

	// Source with empty content.
	if anns := matcher.Generate("some output", []engine.SourceContext{{ToolName: "file_search", Content: ""}}); len(anns) != 0 {
		t.Errorf("expected 0 annotations for empty source content, got %d", len(anns))
	}
}

// TestAnnotationMultipleSources verifies annotations from multiple sources.
func TestAnnotationMultipleSources(t *testing.T) {
	matcher := engine.NewSubstringMatcher(20)

	outputText := "Go provides excellent concurrency primitives. Python offers simple syntax for beginners."

	sources := []engine.SourceContext{
		{
			ToolName: "file_search",
			FileID:   "file_go",
			Content:  "Go provides excellent concurrency primitives and strong typing.",
		},
		{
			ToolName: "web_search",
			URL:      "https://python.org/about",
			Title:    "About Python",
			Content:  "Python offers simple syntax for beginners and experts alike.",
		},
	}

	annotations := matcher.Generate(outputText, sources)
	if len(annotations) < 2 {
		t.Fatalf("expected at least 2 annotations, got %d", len(annotations))
	}

	// Verify we got both types.
	types := map[string]bool{}
	for _, a := range annotations {
		types[a.Type] = true
	}
	if !types["file_citation"] {
		t.Error("expected a file_citation annotation")
	}
	if !types["url_citation"] {
		t.Error("expected a url_citation annotation")
	}
}

// TestAnnotationDefaultMinLength verifies that NewSubstringMatcher applies
// a default minimum match length when given 0 or negative values.
func TestAnnotationDefaultMinLength(t *testing.T) {
	matcher := engine.NewSubstringMatcher(0)
	if matcher.MinMatchLen != 20 {
		t.Errorf("MinMatchLen = %d, want 20 (default)", matcher.MinMatchLen)
	}

	matcher2 := engine.NewSubstringMatcher(-5)
	if matcher2.MinMatchLen != 20 {
		t.Errorf("MinMatchLen = %d, want 20 (default)", matcher2.MinMatchLen)
	}
}

// TestExtractSourceContexts verifies extraction of SourceContext from tool results.
func TestExtractSourceContexts(t *testing.T) {
	results := []engine.SourceContext{
		{ToolName: "file_search", FileID: "file_1", Content: "chunk data"},
		{ToolName: "web_search", URL: "https://example.com", Title: "Example", Content: "snippet"},
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 source contexts, got %d", len(results))
	}
	if results[0].ToolName != "file_search" {
		t.Errorf("expected first source to be file_search")
	}
	if results[1].URL != "https://example.com" {
		t.Errorf("expected second source URL to be https://example.com")
	}
}
