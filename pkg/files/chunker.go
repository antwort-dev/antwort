package files

import "unicode"

// Chunker splits text into segments suitable for embedding.
type Chunker interface {
	// Chunk splits the given text into chunks.
	Chunk(text string) []Chunk
}

// FixedSizeChunker splits text into fixed-size chunks with configurable overlap.
// Sizes are specified in approximate tokens (1 token ~ 4 characters).
type FixedSizeChunker struct {
	maxChunkChars int
	overlapChars  int
}

// NewFixedSizeChunker creates a chunker with the given max size and overlap in tokens.
// Tokens are approximated as 4 characters each.
func NewFixedSizeChunker(maxTokens, overlapTokens int) *FixedSizeChunker {
	if maxTokens <= 0 {
		maxTokens = 800
	}
	if overlapTokens < 0 {
		overlapTokens = 0
	}
	return &FixedSizeChunker{
		maxChunkChars: maxTokens * 4,
		overlapChars:  overlapTokens * 4,
	}
}

func (c *FixedSizeChunker) Chunk(text string) []Chunk {
	if text == "" {
		return nil
	}

	runes := []rune(text)
	total := len(runes)
	if total == 0 {
		return nil
	}

	var chunks []Chunk
	pos := 0
	index := 0

	for pos < total {
		end := pos + c.maxChunkChars
		if end > total {
			end = total
		}

		// Try to break at a whitespace boundary to avoid splitting words.
		if end < total {
			breakAt := end
			for breakAt > pos+c.maxChunkChars/2 {
				if unicode.IsSpace(runes[breakAt]) {
					end = breakAt
					break
				}
				breakAt--
			}
		}

		chunkText := string(runes[pos:end])
		chunks = append(chunks, Chunk{
			Index:     index,
			Text:      chunkText,
			StartChar: pos,
			EndChar:   end,
		})

		index++

		// Advance position, subtracting overlap.
		advance := end - pos - c.overlapChars
		if advance <= 0 {
			advance = 1 // Guarantee forward progress.
		}
		pos += advance
	}

	return chunks
}
