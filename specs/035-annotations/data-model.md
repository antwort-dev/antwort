# Data Model: 035-annotations

**Date**: 2026-03-02

## Entity Changes

### Annotation (Extended)

The existing `Annotation` struct gains citation-specific optional fields.

| Field      | Type   | Required | Description                                        |
|------------|--------|----------|----------------------------------------------------|
| Type       | string | Yes      | `file_citation` or `url_citation`                  |
| Text       | string | No       | Display text for the annotation                    |
| StartIndex | int    | Yes      | Start character position in output text            |
| EndIndex   | int    | Yes      | End character position in output text              |
| FileID     | string | No       | Source file ID (file_citation only)                |
| Quote      | string | No       | Quoted passage from source (file_citation only)    |
| URL        | string | No       | Source URL (url_citation only)                     |
| Title      | string | No       | Page title (url_citation only)                     |

### ToolResult (Extended)

The existing `ToolResult` struct gains a metadata field for source information.

| Field    | Type              | Required | Description                                    |
|----------|-------------------|----------|------------------------------------------------|
| CallID   | string            | Yes      | Matches originating tool call                  |
| Output   | string            | Yes      | Human-readable result text                     |
| IsError  | bool              | Yes      | Indicates error                                |
| Metadata | map[string]string | No       | Source metadata (file_id, url, title, content) |

### SourceContext (New)

Tracks tool result sources available to the annotation generator.

| Field       | Type           | Description                                    |
|-------------|----------------|------------------------------------------------|
| ToolName    | string         | Tool that produced this source (file_search, web_search) |
| FileID      | string         | Source file ID (file_search)                   |
| URL         | string         | Source URL (web_search)                        |
| Title       | string         | Page or file title                             |
| Content     | string         | Source content (chunk text or search snippet)  |

## Interfaces

### AnnotationGenerator (1 method)

Generates annotations from output text and source context.

| Method   | Input                                    | Output         |
|----------|------------------------------------------|----------------|
| Generate | outputText string, sources []SourceContext | []Annotation   |

**Implementations**: SubstringMatcher (built-in)

## Relationships

```
ToolResult 1--* SourceContext (extracted from metadata)
SourceContext *--* Annotation (matched against output text)
OutputContentPart 1--* Annotation (attached to output)
```

## SSE Event Addition

### response.output_text.annotation.added

New event emitted during streaming, one per annotation.

| Field            | Type       | Description                         |
|------------------|------------|-------------------------------------|
| type             | string     | Event type constant                 |
| sequence_number  | int        | Monotonically increasing            |
| item_id          | string     | Parent output item ID               |
| output_index     | int        | Output item position                |
| content_index    | int        | Content part position               |
| annotation_index | int        | Position within annotations array   |
| annotation       | Annotation | The annotation data                 |
