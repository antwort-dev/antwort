package engine

import (
	"strings"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/tools"
)

// SourceContext holds metadata about a tool result source for annotation generation.
type SourceContext struct {
	ToolName string // "file_search" or "web_search"
	FileID   string // Source file ID (file_search)
	URL      string // Source URL (web_search)
	Title    string // Page or file title
	Content  string // Source content (chunk text or search snippet)
}

// AnnotationGenerator generates annotations from output text and source contexts.
type AnnotationGenerator interface {
	Generate(outputText string, sources []SourceContext) []api.Annotation
}

// SubstringMatcher generates annotations by finding source content in the output text.
// It uses longest common substring matching to locate cited passages.
type SubstringMatcher struct {
	// MinMatchLen is the minimum substring length to consider a match.
	// Shorter matches are ignored to avoid false positives.
	MinMatchLen int
}

// NewSubstringMatcher creates a SubstringMatcher with the given minimum match length.
// If minLen is 0 or negative, defaults to 20 characters.
func NewSubstringMatcher(minLen int) *SubstringMatcher {
	if minLen <= 0 {
		minLen = 20
	}
	return &SubstringMatcher{MinMatchLen: minLen}
}

func (m *SubstringMatcher) Generate(outputText string, sources []SourceContext) []api.Annotation {
	if outputText == "" || len(sources) == 0 {
		return nil
	}

	var annotations []api.Annotation
	outputLower := strings.ToLower(outputText)

	for _, src := range sources {
		if src.Content == "" {
			continue
		}

		ann := m.matchSource(outputText, outputLower, src)
		if ann != nil {
			annotations = append(annotations, *ann)
		}
	}

	return annotations
}

func (m *SubstringMatcher) matchSource(outputText, outputLower string, src SourceContext) *api.Annotation {
	srcLower := strings.ToLower(src.Content)

	// Find the longest common substring between source and output.
	matchStart, matchLen := longestCommonSubstring(outputLower, srcLower)

	if matchLen >= m.MinMatchLen {
		quote := outputText[matchStart : matchStart+matchLen]
		return m.buildAnnotation(src, quote, matchStart, matchStart+matchLen)
	}

	// Fallback: try to find any sentence from source in the output.
	sentences := splitSentences(src.Content)
	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if len(sent) < m.MinMatchLen {
			continue
		}
		idx := strings.Index(outputLower, strings.ToLower(sent))
		if idx >= 0 {
			return m.buildAnnotation(src, outputText[idx:idx+len(sent)], idx, idx+len(sent))
		}
	}

	return nil
}

func (m *SubstringMatcher) buildAnnotation(src SourceContext, quote string, start, end int) *api.Annotation {
	switch src.ToolName {
	case "file_search":
		return &api.Annotation{
			Type:       "file_citation",
			FileID:     src.FileID,
			Quote:      quote,
			StartIndex: start,
			EndIndex:   end,
		}
	case "web_search":
		return &api.Annotation{
			Type:       "url_citation",
			URL:        src.URL,
			Title:      src.Title,
			StartIndex: start,
			EndIndex:   end,
		}
	}
	return nil
}

// ExtractSourceContexts converts tool result metadata into SourceContext entries.
func ExtractSourceContexts(results []tools.ToolResult) []SourceContext {
	var sources []SourceContext
	for _, r := range results {
		if r.Metadata == nil || r.IsError {
			continue
		}
		toolName := r.Metadata["tool"]
		if toolName == "" {
			continue
		}
		sources = append(sources, SourceContext{
			ToolName: toolName,
			FileID:   r.Metadata["file_id"],
			URL:      r.Metadata["url"],
			Title:    r.Metadata["title"],
			Content:  r.Metadata["content"],
		})
	}
	return sources
}

// longestCommonSubstring finds the longest common substring between a and b.
// Returns the start index in a and the length of the match.
func longestCommonSubstring(a, b string) (int, int) {
	if len(a) == 0 || len(b) == 0 {
		return 0, 0
	}

	aRunes := []rune(a)
	bRunes := []rune(b)
	m, n := len(aRunes), len(bRunes)

	// Use a rolling row to reduce memory from O(m*n) to O(n).
	prev := make([]int, n+1)
	curr := make([]int, n+1)

	bestLen := 0
	bestEndA := 0

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if aRunes[i-1] == bRunes[j-1] {
				curr[j] = prev[j-1] + 1
				if curr[j] > bestLen {
					bestLen = curr[j]
					bestEndA = i
				}
			} else {
				curr[j] = 0
			}
		}
		prev, curr = curr, prev
		for k := range curr {
			curr[k] = 0
		}
	}

	// Convert rune positions back to byte positions.
	startRune := bestEndA - bestLen
	startByte := 0
	for i := 0; i < startRune; i++ {
		startByte += len(string(aRunes[i]))
	}
	matchBytes := 0
	for i := startRune; i < bestEndA; i++ {
		matchBytes += len(string(aRunes[i]))
	}

	return startByte, matchBytes
}

// splitSentences splits text into sentences at period, question mark, or exclamation boundaries.
func splitSentences(text string) []string {
	var sentences []string
	start := 0
	for i, r := range text {
		if r == '.' || r == '?' || r == '!' {
			if i > start {
				sentences = append(sentences, text[start:i+1])
			}
			start = i + 1
		}
	}
	if start < len(text) {
		sentences = append(sentences, text[start:])
	}
	return sentences
}
