# Implementation Plan: Annotation & Citation Generation

**Branch**: `035-annotations` | **Date**: 2026-03-02 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/035-annotations/spec.md`

## Summary

Add automatic citation generation to antwort responses. When the agentic loop uses file_search or web_search tool results, the engine post-processes the output text to attach file_citation and url_citation annotations with source references and character positions. During streaming, annotations are emitted as `response.output_text.annotation.added` SSE events.

The implementation extends three existing packages: `pkg/api` (annotation type), `pkg/tools` (result metadata), and `pkg/engine` (annotation generation in the agentic loop). No new packages are created.

## Technical Context

**Language/Version**: Go 1.22+ (consistent with all existing specs)
**Primary Dependencies**: Go standard library only (string matching, no external NLP libraries)
**Storage**: No new storage. Annotations are ephemeral, attached to response output.
**Testing**: `go test` with table-driven tests. Integration tests using the existing mock provider pattern.
**Target Platform**: Linux containers on Kubernetes
**Project Type**: Web service (engine enhancement to existing antwort server)
**Performance Goals**: Annotation generation adds less than 50ms to response latency
**Constraints**: Must not modify the streaming text output. Annotations are appended after text is complete.

## Constitution Check

| Principle | Pre-Design | Post-Design | Notes |
|-----------|-----------|-------------|-------|
| I. Interface-First Design | PASS | PASS | AnnotationGenerator interface (1 method) |
| II. Zero External Dependencies | PASS | PASS | Pure stdlib string matching |
| III. Nil-Safe Composition | PASS | PASS | Nil annotation generator = no annotations (feature disabled) |
| IV. Typed Error Domain | PASS | PASS | No new error types; annotation failures are logged, never fatal |
| V. Validate Early | N/A | N/A | Annotations are output, not input |
| VII. Streaming First-Class | PASS | PASS | Dedicated SSE event type for annotations |
| VIII. Context | N/A | N/A | No new context values |

No violations. No complexity tracking needed.

## Design Decisions

### D1: Annotation Type Extension

Extend the existing `Annotation` struct in `pkg/api/types.go` with optional citation fields:

```go
type Annotation struct {
    Type       string `json:"type"`
    Text       string `json:"text,omitempty"`
    StartIndex int    `json:"start_index,omitempty"`
    EndIndex   int    `json:"end_index,omitempty"`
    // Citation-specific fields (optional, type-dependent)
    FileID string `json:"file_id,omitempty"`   // file_citation
    Quote  string `json:"quote,omitempty"`     // file_citation
    URL    string `json:"url,omitempty"`       // url_citation
    Title  string `json:"title,omitempty"`     // url_citation
}
```

### D2: Tool Result Metadata

Add `Metadata map[string]string` to `ToolResult` in `pkg/tools/executor.go`. Providers populate this with source info:

- file_search: `{"tool": "file_search", "file_id": "...", "content": "chunk text"}`
- web_search: `{"tool": "web_search", "url": "...", "title": "...", "content": "snippet"}`

### D3: Annotation Generator

New `pkg/engine/annotations.go` file with:

```go
type AnnotationGenerator interface {
    Generate(outputText string, sources []SourceContext) []api.Annotation
}
```

The `SubstringMatcher` implementation finds source content in the output text using longest common substring matching with a configurable minimum length.

### D4: Engine Loop Integration

Hook into the engine loop at two points:

1. **After tool execution**: Extract `SourceContext` from `ToolResult.Metadata`
2. **After text accumulation**: Call `AnnotationGenerator.Generate()` and attach results to `OutputContentPart.Annotations`

The generator is injected into the engine as an optional dependency (nil = disabled).

### D5: SSE Annotation Events

Add `EventAnnotationAdded = "response.output_text.annotation.added"` to `pkg/api/events.go`.

Emit one event per annotation after `EventOutputTextDone`, before `EventContentPartDone`. Add marshaling case in `StreamEvent.MarshalJSON()`.

### D6: Provider Metadata Population

Modify file_search and web_search providers to populate `ToolResult.Metadata`:

- **file_search** (`pkg/tools/builtins/filesearch/provider.go`): Add `file_id` from `SearchMatch.Metadata["file_id"]` and `content` from `SearchMatch.Content`
- **web_search** (`pkg/tools/builtins/websearch/provider.go`): Add `url`, `title`, `content` from `SearchResult` fields

## Project Structure

### Source Code Changes

```text
pkg/api/types.go                    # Extend Annotation struct (D1)
pkg/api/events.go                   # Add EventAnnotationAdded constant (D5)
pkg/api/events.go                   # Add MarshalJSON case for annotation event (D5)
pkg/tools/executor.go               # Add Metadata field to ToolResult (D2)
pkg/tools/builtins/filesearch/provider.go  # Populate ToolResult.Metadata (D6)
pkg/tools/builtins/websearch/provider.go   # Populate ToolResult.Metadata (D6)
pkg/engine/annotations.go           # NEW: AnnotationGenerator + SubstringMatcher (D3)
pkg/engine/annotations_test.go      # NEW: Tests for annotation generation
pkg/engine/loop.go                  # Integrate annotation generation (D4)
pkg/engine/engine.go                # Accept optional AnnotationGenerator (D4)
```

No new packages. All changes are in existing packages.
