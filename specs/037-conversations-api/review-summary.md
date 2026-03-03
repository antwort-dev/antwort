# Review Summary: 037-conversations-api

**Reviewed**: 2026-03-03 | **Verdict**: PASS | **Spec Version**: Draft

## For Reviewers

This spec adds explicit conversation management to antwort. Currently conversations are implicit (response chains via `previous_response_id`). This adds a Conversations API for creating, listing, and deleting named conversations, plus item management within conversations. Fully backward compatible.

### Key Areas to Review

1. **History reconstruction (FR-012/FR-013)**: When `conversation_id` is used, the engine reconstructs message history from all conversation items instead of following `previous_response_id` links. This is the main integration point with the existing engine.

2. **Interaction between conversation_id and previous_response_id (FR-014)**: When both are set, `previous_response_id` provides the message history (explicit chain wins). This avoids ambiguity about which history source to use.

3. **Soft delete (FR-017)**: Deleting a conversation removes the container but not the responses. This is safer but means orphaned responses may accumulate. Consider adding cleanup guidance in docs.

4. **User scoping (FR-006)**: Conversations are user-scoped via the existing auth identity system. Same pattern as files and responses.

### Constitution Compliance

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First Design | PASS | ConversationStore interface (new, 4-5 methods) |
| II. Zero External Dependencies | PASS | Extends existing storage layer |
| III. Nil-Safe Composition | PASS | ConversationStore nil = conversations disabled |
| IV. Typed Error Domain | PASS | Uses existing APIError types |
| V. Validate Early | PASS | conversation_id validated before processing |
| VIII. Context | PASS | User identity from context for scoping |

### Coverage

- 5 user stories (3x P1, 2x P2), each independently testable
- 17 functional requirements
- 5 success criteria
- 5 edge cases

### Observations (non-blocking)

1. **No unique name constraint**: Multiple conversations can have the same name. This is intentional (IDs are unique, names are labels) but may surprise users expecting unique names.

2. **Item ordering by timestamp**: If two items have the same creation timestamp (parallel tool calls), ordering is non-deterministic. In practice this is unlikely to matter since items within a single response are ordered by their position.

### Red Flags

None.
