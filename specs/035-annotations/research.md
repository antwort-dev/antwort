# Research: 035-annotations

**Date**: 2026-03-02

## R1: Annotation Data Model Extension

**Decision**: Extend the existing `Annotation` struct with optional citation-specific fields rather than creating separate types. Use a discriminated union pattern via the `Type` field.

**Rationale**: The current `Annotation` struct has `Type`, `Text`, `StartIndex`, `EndIndex`. Adding `FileID`, `Quote`, `URL`, `Title` as optional fields (with `omitempty`) keeps a single type while supporting both citation types. This matches the OpenAI SDK's polymorphic annotation model. Creating separate Go types would complicate JSON serialization and the annotations array.

**Extended fields**:
- `FileID` (string): Source file ID for file_citation
- `Quote` (string): Quoted passage from the source for file_citation
- `URL` (string): Source URL for url_citation
- `Title` (string): Page title for url_citation

**Alternatives considered**:
- Separate `FileCitation` and `URLCitation` types: Requires type-switching and complicates the `[]Annotation` array serialization. OpenAI uses a flat annotation object.
- Nested `Citation` struct: Over-engineering for 4 optional fields.

## R2: Source Metadata Extraction from Tool Results

**Decision**: Modify tool result handling to carry structured metadata alongside the plain text output. Add a `Metadata` field to `ToolResult` for source information.

**Rationale**: The current `ToolResult.Output` is a plain string. Parsing URLs and file IDs out of formatted text is fragile and error-prone. Adding a `Metadata map[string]string` field to `ToolResult` lets providers pass structured source data (file_id, url, title, content) alongside the human-readable output. The engine consumes metadata for annotation generation without parsing text.

**Alternative considered**:
- Parse source info from formatted text output: Fragile, depends on formatting conventions, breaks if format changes. Rejected.
- New `ToolResultWithSources` type: Unnecessary, a metadata map on the existing type is simpler.

## R3: Text Position Mapping Strategy

**Decision**: Use substring matching between tool result content/snippets and the LLM's output text to find citation positions. Fall back to no-position annotations when matching fails.

**Rationale**: The LLM may paraphrase, summarize, or reorder source material. Exact substring matching works when the LLM quotes directly. For paraphrased content, the system generates annotations at the paragraph or sentence level where the tool result was likely used (by tracking which tool call's turn produced which output text segment). This is a best-effort approach that provides value without requiring semantic similarity computation.

**Matching algorithm**:
1. For each tool result with source metadata, find the longest common substring between the source content and the output text
2. If a match of sufficient length is found (configurable minimum, default 20 characters), use its position
3. If no match is found, annotate the entire output text segment that followed the tool result's turn

**Alternatives considered**:
- Semantic similarity matching: Requires an embedding model call per annotation. Too expensive for every response.
- Model-instructed citations: Prompt the LLM to include citation markers. Depends on model capability, unreliable, changes the output format.

## R4: Annotation Generation Hook Point

**Decision**: Generate annotations in the engine loop after text accumulation is complete, before emitting `EventOutputTextDone`. This applies to both streaming and non-streaming modes.

**Rationale**: Annotations require the complete output text and character positions. In streaming mode, text arrives as deltas. The engine already accumulates text into `accumulatedText`. The annotation generator runs once per output item, after all text is accumulated, before the final events are emitted.

**Integration points**:
- Non-streaming: In `runAgenticLoop()`, after the final provider response, before building the Response object
- Streaming: In `consumeStreamTurn()`, after text accumulation is complete, before emitting `EventOutputTextDone`

## R5: SSE Event for Annotations

**Decision**: Add `response.output_text.annotation.added` event type. Emit one event per annotation, after `EventOutputTextDone`, before `EventContentPartDone`.

**Rationale**: Annotations reference character positions in the final text. They must be emitted after the text is complete. Emitting before `EventContentPartDone` ensures clients receive annotations as part of the content part lifecycle. One event per annotation matches the OpenAI streaming pattern.

**Event payload**:
```json
{
  "type": "response.output_text.annotation.added",
  "sequence_number": N,
  "item_id": "item_...",
  "output_index": 0,
  "content_index": 0,
  "annotation_index": 0,
  "annotation": {
    "type": "file_citation",
    "file_id": "file_...",
    "quote": "...",
    "start_index": 42,
    "end_index": 87
  }
}
```

## R6: Tool Result Provenance Tracking

**Decision**: Track which tool calls contributed to each output text segment by maintaining a per-turn source context in the engine loop.

**Rationale**: The agentic loop processes tool results in turns. Each turn's tool results inform the next LLM response. By recording the tool results available at each turn, the annotation generator knows which sources the LLM had access to when generating each output segment. This avoids attributing content to sources that weren't available at that point in the conversation.

**Implementation**: The engine loop already tracks `resultItems` per turn. Pass these to the annotation generator along with the accumulated text.
