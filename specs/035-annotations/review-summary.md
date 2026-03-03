# Review Summary: 035-annotations

**Reviewed**: 2026-03-02 | **Verdict**: PASS | **Spec Version**: Draft

## For Reviewers

This spec adds automatic citation generation to antwort responses. When the agentic loop uses file_search or web_search tool results, the engine attaches file_citation and url_citation annotations to the output text with source references and character positions. During streaming, annotations are emitted as SSE events.

### Key Areas to Review

1. **Post-processing vs model-generated**: This spec uses post-processing (text matching between tool results and LLM output) rather than instructing the model to generate citations. Post-processing is deterministic but may miss paraphrased content. The spec handles this gracefully (annotations without quotes when no match found).

2. **Character position accuracy**: FR-007 requires accurate start_index/end_index values. This depends on the text matching algorithm's ability to locate tool result content within the LLM's output. Review whether "best available quote match" (edge case) is sufficient.

3. **SSE event timing**: FR-010 says annotation events come after output text is complete. This means annotations are batched at the end of streaming, not interleaved with text deltas. This is the correct approach since character positions reference the final text.

4. **Annotation data model extension**: The existing `Annotation` struct has type, text, start_index, end_index. Citations need additional fields (file_id, quote, url, title). The spec correctly defers the how to the plan.

### Constitution Compliance

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First Design | N/A | No new interfaces needed (engine enhancement) |
| II. Zero External Dependencies | PASS | No new dependencies, pure logic in engine |
| III. Nil-Safe Composition | PASS | FR-006: no annotations when no search tools used |
| IV. Typed Error Domain | PASS | No new error types needed |
| V. Validate Early, Fail Fast | N/A | Annotations are output, not input validation |
| VI. Protocol-Agnostic Provider | N/A | Engine concern, not provider |
| VII. Streaming First-Class | PASS | FR-009/FR-010: annotation SSE events explicitly addressed |
| VIII. Context Carries Cross-Cutting | N/A | No new context data |
| IX. Kubernetes-Native | N/A | No infrastructure changes |

### Coverage

- **4 user stories** (2x P1, 2x P2), each independently testable
- **15 functional requirements** covering annotation types, generation, streaming, matching, compatibility
- **6 success criteria**, all measurable
- **5 edge cases** identified

### Observations (non-blocking)

1. **Text matching complexity**: The spec assumes tool result content can be located in the LLM output via text matching. In practice, LLMs frequently paraphrase, reorder, or summarize source material. The "best available quote match" edge case is a pragmatic fallback, but the matching algorithm's quality will determine how useful citations are. This is an implementation concern for the plan.

2. **No annotation for code_interpreter**: The scope explicitly excludes annotations for tools other than file_search and web_search. This is correct for v1 but worth noting for future expansion.

3. **FR-008 non-overlapping ranges**: This constraint may be difficult to enforce if the LLM interleaves information from multiple sources within the same sentence. The plan should address how overlapping source material is handled (first-match wins, longest match, etc.).

### Red Flags

None.
