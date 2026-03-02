package engine

import (
	"testing"

	"github.com/rhuss/antwort/pkg/tools"
)

func TestSubstringMatcher_ExactMatch(t *testing.T) {
	m := NewSubstringMatcher(10)
	sources := []SourceContext{
		{ToolName: "file_search", FileID: "file_abc", Content: "Kubernetes automates deployment and scaling"},
	}
	output := "As noted, Kubernetes automates deployment and scaling of containerized applications."

	anns := m.Generate(output, sources)
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(anns))
	}
	if anns[0].Type != "file_citation" {
		t.Errorf("type: got %q, want file_citation", anns[0].Type)
	}
	if anns[0].FileID != "file_abc" {
		t.Errorf("file_id: got %q, want file_abc", anns[0].FileID)
	}
	if anns[0].StartIndex < 0 || anns[0].EndIndex <= anns[0].StartIndex {
		t.Errorf("invalid range: start=%d end=%d", anns[0].StartIndex, anns[0].EndIndex)
	}
	quoted := output[anns[0].StartIndex:anns[0].EndIndex]
	if len(quoted) < 10 {
		t.Errorf("quote too short: %q", quoted)
	}
}

func TestSubstringMatcher_URLCitation(t *testing.T) {
	m := NewSubstringMatcher(10)
	sources := []SourceContext{
		{ToolName: "web_search", URL: "https://example.com", Title: "Example Page", Content: "The quick brown fox jumps over the lazy dog"},
	}
	output := "According to the source, the quick brown fox jumps over the lazy dog in this story."

	anns := m.Generate(output, sources)
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(anns))
	}
	if anns[0].Type != "url_citation" {
		t.Errorf("type: got %q, want url_citation", anns[0].Type)
	}
	if anns[0].URL != "https://example.com" {
		t.Errorf("url: got %q", anns[0].URL)
	}
	if anns[0].Title != "Example Page" {
		t.Errorf("title: got %q", anns[0].Title)
	}
}

func TestSubstringMatcher_NoMatch(t *testing.T) {
	m := NewSubstringMatcher(20)
	sources := []SourceContext{
		{ToolName: "file_search", FileID: "file_abc", Content: "completely different text"},
	}
	output := "The LLM generated a response about something entirely unrelated."

	anns := m.Generate(output, sources)
	if len(anns) != 0 {
		t.Errorf("expected 0 annotations for unmatched content, got %d", len(anns))
	}
}

func TestSubstringMatcher_MultipleSources(t *testing.T) {
	m := NewSubstringMatcher(10)
	sources := []SourceContext{
		{ToolName: "file_search", FileID: "file_1", Content: "Pods are the smallest deployable units"},
		{ToolName: "web_search", URL: "https://k8s.io", Title: "K8s", Content: "Services provide stable networking"},
	}
	output := "Pods are the smallest deployable units in Kubernetes. Services provide stable networking for Pods."

	anns := m.Generate(output, sources)
	if len(anns) != 2 {
		t.Fatalf("expected 2 annotations, got %d", len(anns))
	}

	// Verify types.
	types := map[string]bool{}
	for _, a := range anns {
		types[a.Type] = true
	}
	if !types["file_citation"] || !types["url_citation"] {
		t.Errorf("expected both citation types, got %v", types)
	}
}

func TestSubstringMatcher_EmptyInputs(t *testing.T) {
	m := NewSubstringMatcher(10)

	if anns := m.Generate("", nil); anns != nil {
		t.Error("expected nil for empty output")
	}
	if anns := m.Generate("some text", nil); anns != nil {
		t.Error("expected nil for nil sources")
	}
	if anns := m.Generate("some text", []SourceContext{}); anns != nil {
		t.Error("expected nil for empty sources")
	}
}

func TestSubstringMatcher_EmptyContent(t *testing.T) {
	m := NewSubstringMatcher(10)
	sources := []SourceContext{
		{ToolName: "file_search", FileID: "file_abc", Content: ""},
	}
	anns := m.Generate("some output text", sources)
	if len(anns) != 0 {
		t.Errorf("expected 0 annotations for empty content, got %d", len(anns))
	}
}

func TestSubstringMatcher_CaseInsensitive(t *testing.T) {
	m := NewSubstringMatcher(10)
	sources := []SourceContext{
		{ToolName: "file_search", FileID: "file_abc", Content: "KUBERNETES AUTOMATES DEPLOYMENT"},
	}
	output := "kubernetes automates deployment and scaling"

	anns := m.Generate(output, sources)
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation (case-insensitive match), got %d", len(anns))
	}
}

func TestExtractSourceContexts(t *testing.T) {
	results := []tools.ToolResult{
		{
			CallID: "call_1",
			Output: "some output",
			Metadata: map[string]string{
				"tool":    "file_search",
				"file_id": "file_abc",
				"content": "chunk text",
			},
		},
		{
			CallID: "call_2",
			Output: "search results",
			Metadata: map[string]string{
				"tool":    "web_search",
				"url":     "https://example.com",
				"title":   "Example",
				"content": "snippet",
			},
		},
		{
			CallID:  "call_3",
			Output:  "no metadata",
			IsError: false,
		},
		{
			CallID:  "call_4",
			Output:  "error result",
			IsError: true,
			Metadata: map[string]string{
				"tool": "file_search",
			},
		},
	}

	contexts := ExtractSourceContexts(results)
	if len(contexts) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(contexts))
	}
	if contexts[0].ToolName != "file_search" || contexts[0].FileID != "file_abc" {
		t.Errorf("context 0: got %+v", contexts[0])
	}
	if contexts[1].ToolName != "web_search" || contexts[1].URL != "https://example.com" {
		t.Errorf("context 1: got %+v", contexts[1])
	}
}

func TestSubstringMatcher_NoAnnotationsWithoutToolMetadata(t *testing.T) {
	m := NewSubstringMatcher(10)
	// No sources at all.
	anns := m.Generate("The response text contains information.", nil)
	if len(anns) != 0 {
		t.Errorf("expected 0 annotations without sources, got %d", len(anns))
	}
}

func TestAnnotationPositionsAreValid(t *testing.T) {
	m := NewSubstringMatcher(10)
	sources := []SourceContext{
		{ToolName: "file_search", FileID: "f1", Content: "container orchestration platform"},
	}
	output := "Kubernetes is a container orchestration platform for managing workloads."

	anns := m.Generate(output, sources)
	if len(anns) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(anns))
	}

	a := anns[0]
	if a.StartIndex < 0 || a.EndIndex > len(output) || a.StartIndex >= a.EndIndex {
		t.Fatalf("invalid positions: start=%d end=%d len=%d", a.StartIndex, a.EndIndex, len(output))
	}

	// Verify substring extraction.
	extracted := output[a.StartIndex:a.EndIndex]
	if extracted != a.Quote {
		t.Errorf("extracted %q does not match quote %q", extracted, a.Quote)
	}
}

func TestDefaultMinMatchLen(t *testing.T) {
	m := NewSubstringMatcher(0)
	if m.MinMatchLen != 20 {
		t.Errorf("default min match len: got %d, want 20", m.MinMatchLen)
	}

	m2 := NewSubstringMatcher(-5)
	if m2.MinMatchLen != 20 {
		t.Errorf("negative min match len: got %d, want 20", m2.MinMatchLen)
	}
}

func TestLongestCommonSubstring(t *testing.T) {
	tests := []struct {
		name    string
		a, b    string
		wantLen int
	}{
		{"exact", "hello world", "hello world", 11},
		{"prefix", "hello world", "hello", 5},
		{"suffix", "hello world", "world", 5},
		{"middle", "the quick brown fox", "quick brown", 11},
		{"no match", "abc", "xyz", 0},
		{"empty a", "", "hello", 0},
		{"empty b", "hello", "", 0},
		{"both empty", "", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotLen := longestCommonSubstring(tt.a, tt.b)
			if gotLen != tt.wantLen {
				t.Errorf("longestCommonSubstring(%q, %q) len = %d, want %d", tt.a, tt.b, gotLen, tt.wantLen)
			}
		})
	}
}
