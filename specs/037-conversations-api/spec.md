# Feature Specification: Conversations API

**Feature Branch**: `037-conversations-api`
**Created**: 2026-03-03
**Status**: Draft
**Input**: User description: "Conversations API with explicit CRUD operations, item management, and named conversations. Backward compatible with response chain approach."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create and Use a Named Conversation (Priority: P1)

A user creates a named conversation, then sends multiple responses within that conversation. The system tracks all responses and items in order, allowing the user to retrieve the full conversation history at any time. The conversation name helps users organize and find conversations later.

**Why this priority**: Named conversations are the core value. Without create/use, the entire API has no purpose. This is the minimum viable feature that adds value over the implicit `previous_response_id` approach.

**Independent Test**: Create a conversation with a name, send two responses within it, retrieve the conversation and verify both responses appear in order with their items.

**Acceptance Scenarios**:

1. **Given** an authenticated user, **When** they create a conversation with a name, **Then** a conversation ID is returned with the specified name and empty item list
2. **Given** a conversation, **When** the user creates a response with `conversation_id` set, **Then** the response is linked to the conversation and appears in its item list
3. **Given** a conversation with multiple responses, **When** the user retrieves the conversation, **Then** all items are returned in chronological order
4. **Given** a conversation, **When** the user creates a response with `conversation_id`, **Then** the conversation history is automatically used as context (equivalent to `previous_response_id` chaining but without the user needing to track IDs)

---

### User Story 2 - List and Browse Conversations (Priority: P1)

A user lists their conversations with pagination, filtering, and sorting. This helps users find and manage conversations across sessions.

**Why this priority**: Without listing, users would need to remember conversation IDs. Listing is essential for any conversation management UI or multi-session workflow.

**Independent Test**: Create three conversations with different names, list all, verify pagination works with `after` cursor and `limit`.

**Acceptance Scenarios**:

1. **Given** multiple conversations, **When** the user lists conversations, **Then** all conversations are returned with metadata (name, created time, item count) in a paginated response
2. **Given** a list request with `limit=2`, **When** more than 2 conversations exist, **Then** `has_more` is true and cursors enable fetching the next page
3. **Given** conversations owned by different users, **When** a user lists conversations, **Then** only their own conversations are visible

---

### User Story 3 - Manage Conversation Items (Priority: P2)

A user lists items within a conversation with pagination, and can add items (messages) directly to a conversation without creating a full response. This enables clients to inject system messages, user context, or pre-seeded conversation history.

**Why this priority**: Item management gives fine-grained control over conversation content. Listing items with pagination handles long conversations. Adding items directly enables conversation setup and context injection patterns.

**Independent Test**: Create a conversation, add two user message items directly, list items and verify both appear in order with correct pagination.

**Acceptance Scenarios**:

1. **Given** a conversation with items, **When** the user lists items with `limit` and `after`, **Then** items are returned in chronological order with correct pagination
2. **Given** a conversation, **When** the user adds a message item directly, **Then** the item appears in the conversation's item list at the correct position
3. **Given** an item list request with `order=asc`, **When** items exist, **Then** items are returned oldest first (default is newest first)

---

### User Story 4 - Delete Conversations (Priority: P2)

A user deletes a conversation they no longer need. Deletion removes the conversation metadata and its item associations. The underlying responses remain accessible by their individual IDs (soft delete of the conversation container, not the content).

**Why this priority**: Users need to clean up old conversations. Deletion should be safe (doesn't destroy response data that might be referenced elsewhere).

**Independent Test**: Create a conversation with responses, delete the conversation, verify the conversation is gone but the responses are still accessible by ID.

**Acceptance Scenarios**:

1. **Given** a conversation, **When** the user deletes it, **Then** the conversation no longer appears in list results
2. **Given** a deleted conversation, **When** the user tries to retrieve it, **Then** a 404 response is returned
3. **Given** a deleted conversation with responses, **When** the user retrieves a response by ID, **Then** the response is still accessible (responses are independent of conversation lifecycle)
4. **Given** a conversation owned by another user, **When** the user attempts to delete it, **Then** the request is rejected (appears as not found)

---

### User Story 5 - Backward Compatibility with Response Chains (Priority: P1)

Existing clients using `previous_response_id` continue to work unchanged. The Conversations API is additive. A response created with `previous_response_id` and no `conversation_id` behaves exactly as before. If both are provided, `conversation_id` takes precedence for conversation grouping, but `previous_response_id` still provides the message history to the LLM.

**Why this priority**: Breaking existing clients is not acceptable. The Conversations API must be purely additive.

**Independent Test**: Send a response with `previous_response_id` (no `conversation_id`), verify the existing chaining behavior works identically to before this feature.

**Acceptance Scenarios**:

1. **Given** a response with `previous_response_id` and no `conversation_id`, **When** processed, **Then** the behavior is identical to the pre-existing implementation
2. **Given** a response with `conversation_id` set, **When** processed, **Then** the response is associated with the conversation and conversation history is used as context
3. **Given** a response with both `conversation_id` and `previous_response_id`, **When** processed, **Then** the response is linked to the conversation and `previous_response_id` provides the message history

---

### Edge Cases

- What happens when a user creates a conversation with a name that already exists? A new conversation is created (names are not unique). Each conversation has a unique ID regardless of name.
- What happens when `conversation_id` references a non-existent conversation? The request fails with a 404 error.
- What happens when `conversation_id` references another user's conversation? The request fails with a 404 error (same as not found).
- What happens when a conversation has no items? Listing items returns an empty array. The conversation itself is still valid.
- What happens when a response within a conversation fails? The response is still recorded in the conversation's item list with its failed status.

## Requirements *(mandatory)*

### Functional Requirements

**Conversation CRUD**

- **FR-001**: The system MUST provide an endpoint to create a conversation with an optional name and optional metadata
- **FR-002**: Conversation identifiers MUST follow the project's ID format conventions (prefix + random characters)
- **FR-003**: The system MUST provide an endpoint to retrieve a single conversation by ID, including metadata and item count
- **FR-004**: The system MUST provide an endpoint to list conversations for the authenticated user, with cursor-based pagination (after, limit, order)
- **FR-005**: The system MUST provide an endpoint to delete a conversation by ID
- **FR-006**: Conversations MUST be scoped to the authenticated user. Cross-user access MUST be prevented.

**Conversation Items**

- **FR-007**: The system MUST provide an endpoint to list items within a conversation with cursor-based pagination
- **FR-008**: The system MUST provide an endpoint to add a message item directly to a conversation
- **FR-009**: Items within a conversation MUST be ordered chronologically by creation time
- **FR-010**: When a response is created with `conversation_id`, all input items and output items from that response MUST be automatically added to the conversation

**Response Integration**

- **FR-011**: The system MUST accept an optional `conversation_id` field on the create response request
- **FR-012**: When `conversation_id` is set, the system MUST use the conversation's item history as context for the LLM (equivalent to automatic `previous_response_id` chaining)
- **FR-013**: When `conversation_id` is set without `previous_response_id`, the system MUST reconstruct the message history from all items in the conversation
- **FR-014**: When both `conversation_id` and `previous_response_id` are set, `previous_response_id` MUST provide the message history (explicit chain takes precedence over conversation-derived history)
- **FR-015**: Existing `previous_response_id` behavior MUST remain unchanged when `conversation_id` is not provided

**Conversation Metadata**

- **FR-016**: Each conversation MUST track: identifier, name (optional), owner identity, creation timestamp, last updated timestamp, and total item count
- **FR-017**: Deleting a conversation MUST NOT delete the underlying responses. Responses remain accessible by their individual IDs.

**Documentation (per constitution v1.5.0)**

- **FR-018**: The feature MUST include an API reference page documenting all conversation endpoints with request/response schemas
- **FR-019**: The existing API reference page MUST be updated to document the `conversation_id` field on the create response request
- **FR-020**: The feature MUST include a tutorial or guide showing common conversation patterns (create, multi-turn chat, list history)

### Key Entities

- **Conversation**: A named container for a sequence of interaction items. Has an ID, optional name, owner, timestamps, and metadata. User-scoped.
- **ConversationItem**: A reference linking an item (input message, output message, tool call, tool result) to a conversation with a position/order. Items are stored on the responses; the conversation tracks the ordering.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can create, list, retrieve, and delete conversations within 1 second per operation
- **SC-002**: A conversation with 100 items returns the first page of items in under 1 second
- **SC-003**: Existing clients using `previous_response_id` without `conversation_id` experience zero behavioral changes
- **SC-004**: Users can list conversations with pagination, and the total across all pages matches the actual conversation count
- **SC-005**: Deleting a conversation does not affect the accessibility of individual responses within it
- **SC-006**: Documentation covers all conversation endpoints and the `conversation_id` integration with create response

## Assumptions

- The existing `ResponseStore` interface can be extended or complemented with a `ConversationStore` for conversation-specific persistence
- The `previous_response_id` field and `loadConversationHistory` function continue to work as before
- Conversation item ordering uses creation timestamps; no manual reordering is needed
- Conversation names are human-readable labels, not unique identifiers

## Dependencies

- **Spec 005 (Storage)**: ResponseStore interface and persistence patterns
- **Spec 007 (Auth)**: User identity for conversation scoping
- **Spec 028 (List Endpoints)**: Pagination patterns (cursor-based, ListOptions)
- **Spec 003 (Core Engine)**: Response creation flow where `conversation_id` integration hooks in

## Scope Boundaries

### In Scope

- Conversation CRUD endpoints (create, get, list, delete)
- Item listing with pagination within a conversation
- Direct item addition to conversations
- `conversation_id` field on create response request
- Automatic history reconstruction from conversation items
- User-scoped conversation isolation
- Backward compatibility with `previous_response_id`
- API reference documentation page for all conversation endpoints
- Update to existing API reference with `conversation_id` field on create response
- Tutorial or quickstart section showing conversation usage patterns

### Out of Scope

- Conversation sharing between users (multi-user conversations)
- Conversation forking or branching
- Conversation export/import
- Conversation templates or pre-built conversation flows
- Real-time conversation updates (webhooks, subscriptions)
- Conversation search by content (only list/filter by metadata)
- Conversation archiving (delete is the only cleanup mechanism)
