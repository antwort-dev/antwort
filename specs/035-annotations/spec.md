# Feature Specification: Annotation & Citation Generation

**Feature Branch**: `035-annotations`
**Created**: 2026-03-02
**Status**: Draft
**Input**: User description: "Automatically generate url_citation and file_citation annotations on response output text when web_search or file_search tool results informed the answer. Emit annotation SSE events during streaming."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - File Search Citations (Priority: P1)

A user asks a question that the LLM answers using content retrieved from uploaded documents via the file_search tool. The response includes file_citation annotations that identify which file and which passage was used, with character positions in the output text. The user can see exactly where each piece of information came from.

**Why this priority**: File search without citations is a black box. Users cannot verify accuracy, trace sources, or build trust in RAG-generated answers. Citations are the minimum requirement for trustworthy document-grounded responses.

**Independent Test**: Upload a document with known content, ask a question that retrieves it, verify the response contains file_citation annotations with correct file_id, quote text, and character positions.

**Acceptance Scenarios**:

1. **Given** a file indexed in a vector store, **When** the LLM uses file_search results in its answer, **Then** the response output text includes file_citation annotations linking to the source file
2. **Given** a file_citation annotation, **When** a client inspects it, **Then** it contains: the source file ID, a direct quote from the source, and start/end character indices into the output text
3. **Given** multiple file_search results from different files, **When** the LLM uses content from several files, **Then** each source gets its own annotation with the correct file reference
4. **Given** a streaming response, **When** file_citation annotations are generated, **Then** they are emitted as `response.output_text.annotation.added` SSE events

---

### User Story 2 - Web Search Citations (Priority: P1)

A user asks a question that the LLM answers using content retrieved from web search results. The response includes url_citation annotations that link to the source URLs with titles and character positions.

**Why this priority**: Web search answers without source URLs are unverifiable. URL citations let users click through to the original source and verify the information.

**Independent Test**: Ask a question that triggers web_search, verify the response contains url_citation annotations with URL, title, and character positions.

**Acceptance Scenarios**:

1. **Given** a response that uses web_search results, **When** the LLM incorporates information from search results, **Then** the response output text includes url_citation annotations linking to the source URLs
2. **Given** a url_citation annotation, **When** a client inspects it, **Then** it contains: the source URL, the page title, and start/end character indices into the output text
3. **Given** multiple web search results used in a response, **When** the LLM cites different sources, **Then** each source gets its own url_citation annotation
4. **Given** a streaming response, **When** url_citation annotations are generated, **Then** they are emitted as `response.output_text.annotation.added` SSE events

---

### User Story 3 - Mixed Citations (Priority: P2)

A user asks a question that the LLM answers using both file_search and web_search results in the same response. The output contains a mix of file_citation and url_citation annotations, each correctly referencing its source.

**Why this priority**: Real-world RAG workflows often combine local documents with web search. Users need to distinguish document-sourced from web-sourced information.

**Independent Test**: Set up a response with both file_search and web_search tools, ask a question that uses both, verify the response contains both annotation types with correct source references.

**Acceptance Scenarios**:

1. **Given** a response using both file_search and web_search tools, **When** the LLM uses results from both, **Then** file_citation and url_citation annotations coexist on the same output text
2. **Given** mixed annotations, **When** a client processes them, **Then** annotations do not overlap (character ranges are non-overlapping) and each correctly identifies its source type

---

### User Story 4 - Responses Without Citations (Priority: P2)

When the LLM answers a question without using any search tools (no file_search, no web_search), the response contains no annotations. Annotations are only generated when tool results inform the answer.

**Why this priority**: Annotations should never be fabricated. If no search tool was used, the response should have an empty annotations array, not invented citations.

**Independent Test**: Ask a general knowledge question without configuring any search tools, verify the response has an empty annotations array.

**Acceptance Scenarios**:

1. **Given** a response generated without any search tools, **When** a client reads the output, **Then** the annotations array is empty
2. **Given** a response where the LLM chose not to use available search tools, **When** the response is returned, **Then** no annotations are generated

---

### Edge Cases

- What happens when the LLM paraphrases search results so heavily that exact quotes cannot be extracted? The system generates annotations with the best available quote match. If no match is found, the annotation includes the source reference without a quote.
- What happens when the same source is cited multiple times in one response? Each citation occurrence gets its own annotation with distinct character positions.
- What happens when file_search returns results but the LLM ignores them and answers from its own knowledge? No annotations are generated for unused tool results.
- What happens when the output text is very short and a single source covers the entire answer? One annotation spanning the full output text range is valid.
- What happens when a file referenced in a citation has been deleted? The file_citation still contains the file_id. It is the client's responsibility to handle missing files gracefully.

## Requirements *(mandatory)*

### Functional Requirements

**Annotation Types**

- **FR-001**: The system MUST support `file_citation` annotations with: source file identifier, quoted passage from the source, and start/end character indices in the output text
- **FR-002**: The system MUST support `url_citation` annotations with: source URL, page title, and start/end character indices in the output text
- **FR-003**: Annotations MUST appear in the `annotations` array of the output text content part

**Citation Generation**

- **FR-004**: The system MUST generate file_citation annotations when file_search tool results are used by the LLM in its response
- **FR-005**: The system MUST generate url_citation annotations when web_search tool results are used by the LLM in its response
- **FR-006**: The system MUST NOT generate annotations when no search tool results were used in the response
- **FR-007**: Citation character positions (start_index, end_index) MUST accurately reference the substring in the output text that corresponds to the cited source
- **FR-008**: When multiple sources inform a single response, each source MUST get its own annotation with non-overlapping character ranges

**Streaming**

- **FR-009**: During streaming responses, annotations MUST be emitted as `response.output_text.annotation.added` SSE events
- **FR-010**: Annotation SSE events MUST be emitted after the output text content is complete (annotations reference character positions in the final text)

**Source Matching**

- **FR-011**: For file_citation, the `quote` field MUST contain text extracted from the file_search result that matches the portion of the output it annotates
- **FR-012**: For url_citation, the `title` field MUST contain the title from the web_search result
- **FR-013**: For url_citation, the `url` field MUST contain the URL from the web_search result

**Compatibility**

- **FR-014**: Annotations MUST be compatible with the existing OpenResponses API annotation format so that standard OpenAI SDKs can parse them without modification
- **FR-015**: The annotation generation MUST work with both streaming and non-streaming responses

### Key Entities

- **Annotation**: A citation attached to output text. Contains type (file_citation or url_citation), source reference, and character position range in the output text.
- **FileCitation**: Annotation subtype with file_id and quote from the source document.
- **URLCitation**: Annotation subtype with url and title from the web search result.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of responses that use file_search results include at least one file_citation annotation referencing the source file
- **SC-002**: 100% of responses that use web_search results include at least one url_citation annotation with a valid source URL
- **SC-003**: Annotation character positions (start_index, end_index) correctly delimit the cited text within the output, verifiable by substring extraction
- **SC-004**: Responses that do not use search tools contain zero annotations (no false citations)
- **SC-005**: Existing clients using the OpenAI SDK can read annotation data without code changes
- **SC-006**: Streaming responses emit annotation SSE events within 1 second of the final output text event

## Assumptions

- The file_search tool (Spec 018) returns chunk text and file_id in its results
- The web_search tool (Spec 017) returns URL, title, and snippet in its results
- The LLM's output text contains passages that can be traced back to tool results via text matching
- The existing annotation data model (type, text, start_index, end_index) can be extended with citation-specific fields (file_id, quote, url, title)
- The SSE event type `response.output_text.annotation.added` is already defined in the event taxonomy (Spec 023)

## Dependencies

- **Spec 004 (Agentic Loop)**: The engine loop that processes tool results and generates responses
- **Spec 018 (File Search)**: Source of file_search tool results with file_id and chunk text
- **Spec 017 (Web Search)**: Source of web_search tool results with URL, title, snippet
- **Spec 023 (Tool Lifecycle Events)**: SSE event taxonomy including annotation events
- **Spec 034 (Files API)**: File metadata for citation references

## Scope Boundaries

### In Scope

- file_citation annotations for file_search results
- url_citation annotations for web_search results
- SSE event emission for annotations during streaming
- Citation character position calculation
- Source-to-output text matching for quote extraction
- Annotation data model extension for citation-specific fields

### Out of Scope

- Citation accuracy verification (checking whether the LLM faithfully represented the source)
- Citation formatting in the client UI (client responsibility)
- Annotations for tool types other than file_search and web_search
- Model-generated citations (this spec uses post-processing, not model instruction)
- Cross-response citation deduplication
