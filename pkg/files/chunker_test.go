package files

import (
	"strings"
	"testing"
)

func TestFixedSizeChunker_Chunk(t *testing.T) {
	tests := []struct {
		name           string
		maxTokens      int
		overlapTokens  int
		text           string
		wantCount      int
		wantFirstText  string
		checkOverlap   bool // if true, verify overlap between consecutive chunks
		checkCoverage  bool // if true, verify all text is covered
	}{
		{
			name:          "empty input",
			maxTokens:     100,
			overlapTokens: 0,
			text:          "",
			wantCount:     0,
		},
		{
			name:          "short text fits in one chunk",
			maxTokens:     100,
			overlapTokens: 0,
			text:          "Hello, world!",
			wantCount:     1,
			wantFirstText: "Hello, world!",
		},
		{
			name:          "text at exact boundary",
			maxTokens:     3, // 3 tokens = 12 chars
			overlapTokens: 0,
			text:          "Hello world!", // 12 chars exactly
			wantCount:     1,
			wantFirstText: "Hello world!",
		},
		{
			name:          "text slightly over boundary",
			maxTokens:     3, // 12 chars max
			overlapTokens: 0,
			text:          "Hello world! X",
			wantCount:     2,
		},
		{
			name:          "text requiring overlap",
			maxTokens:     5, // 20 chars max
			overlapTokens: 1, // 4 chars overlap
			text:          "The quick brown fox jumps over the lazy dog near the river bank",
			checkOverlap:  true,
			checkCoverage: true,
		},
		{
			name:          "whitespace-aware splitting",
			maxTokens:     4, // 16 chars max
			overlapTokens: 0,
			text:          "abcdefghij klmno pqrstuvwxyz",
			wantCount:     2,
		},
		{
			name:          "no overlap",
			maxTokens:     5,
			overlapTokens: 0,
			text:          strings.Repeat("word ", 20),
			checkCoverage: true,
		},
		{
			name:          "default max tokens for zero",
			maxTokens:     0,
			overlapTokens: 0,
			text:          "short",
			wantCount:     1,
			wantFirstText: "short",
		},
		{
			name:          "negative overlap treated as zero",
			maxTokens:     5,
			overlapTokens: -1,
			text:          "Hello world this is a test",
			checkCoverage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewFixedSizeChunker(tt.maxTokens, tt.overlapTokens)
			chunks := chunker.Chunk(tt.text)

			if tt.wantCount > 0 && len(chunks) != tt.wantCount {
				t.Errorf("chunk count: got %d, want %d", len(chunks), tt.wantCount)
			}
			if tt.wantCount == 0 && tt.text == "" && chunks != nil {
				t.Errorf("expected nil for empty input, got %d chunks", len(chunks))
			}
			if tt.wantFirstText != "" && len(chunks) > 0 && chunks[0].Text != tt.wantFirstText {
				t.Errorf("first chunk text: got %q, want %q", chunks[0].Text, tt.wantFirstText)
			}

			// Verify chunk indices are sequential.
			for i, c := range chunks {
				if c.Index != i {
					t.Errorf("chunk[%d].Index = %d, want %d", i, c.Index, i)
				}
			}

			// Verify start/end positions are monotonically increasing.
			for i := 1; i < len(chunks); i++ {
				if chunks[i].StartChar <= chunks[i-1].StartChar {
					t.Errorf("chunk[%d].StartChar (%d) should be > chunk[%d].StartChar (%d)",
						i, chunks[i].StartChar, i-1, chunks[i-1].StartChar)
				}
			}

			// Verify first chunk starts at 0 and last chunk ends at text length.
			runes := []rune(tt.text)
			if len(chunks) > 0 {
				if chunks[0].StartChar != 0 {
					t.Errorf("first chunk StartChar: got %d, want 0", chunks[0].StartChar)
				}
				if chunks[len(chunks)-1].EndChar != len(runes) {
					t.Errorf("last chunk EndChar: got %d, want %d", chunks[len(chunks)-1].EndChar, len(runes))
				}
			}

			if tt.checkOverlap && len(chunks) > 1 {
				for i := 1; i < len(chunks); i++ {
					// Overlap means the next chunk starts before the previous chunk ends.
					if chunks[i].StartChar >= chunks[i-1].EndChar {
						t.Errorf("expected overlap: chunk[%d].StartChar (%d) >= chunk[%d].EndChar (%d)",
							i, chunks[i].StartChar, i-1, chunks[i-1].EndChar)
					}
				}
			}

			if tt.checkCoverage && len(chunks) > 0 {
				// Verify that all characters are covered by at least one chunk.
				covered := make([]bool, len(runes))
				for _, c := range chunks {
					for j := c.StartChar; j < c.EndChar && j < len(covered); j++ {
						covered[j] = true
					}
				}
				for i, c := range covered {
					if !c {
						t.Errorf("rune at position %d not covered by any chunk", i)
						break
					}
				}
			}
		})
	}
}

func TestFixedSizeChunker_Unicode(t *testing.T) {
	// Verify chunker handles multi-byte Unicode correctly.
	text := strings.Repeat("\u00e9\u00e0\u00fc", 20) // accented chars, 3 bytes each in UTF-8
	chunker := NewFixedSizeChunker(5, 0)              // 20 chars max per chunk
	chunks := chunker.Chunk(text)

	if len(chunks) == 0 {
		t.Fatal("expected chunks for unicode text")
	}

	// Reconstruct text from non-overlapping chunks (no overlap, so concatenation should work).
	var rebuilt strings.Builder
	for _, c := range chunks {
		rebuilt.WriteString(c.Text)
	}
	if rebuilt.String() != text {
		t.Errorf("reconstructed text does not match original")
	}
}

func TestFixedSizeChunker_SingleCharChunks(t *testing.T) {
	// With a very small max, ensure forward progress and no infinite loop.
	chunker := NewFixedSizeChunker(1, 1) // 4 chars max, 4 chars overlap (would cause no progress without guard)
	chunks := chunker.Chunk("abcd efgh")

	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
	// The key guarantee is that the chunker terminates and produces output.
	// With overlap == maxChunkChars, the advance guard (advance=1) prevents infinite loop.
}
