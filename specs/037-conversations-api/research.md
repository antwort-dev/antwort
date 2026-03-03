# Research: 037-conversations-api

**Date**: 2026-03-03

## R1: ConversationStore Interface Pattern

**Decision**: Define `ConversationStore` as a separate interface in `pkg/transport/handler.go` alongside `ResponseStore`. Follow the same pattern: CRUD + list + tenant scoping.

**Rationale**: ConversationStore is a transport concern (the HTTP adapter needs it to serve conversation endpoints). Placing it next to ResponseStore keeps the storage abstraction consistent. The interface has 6 methods (Save, Get, Delete, List, AddItem, ListItems), which exceeds the constitution's 5-method limit. Split into two: `ConversationStore` (4 methods: Save, Get, Delete, List) and item operations on the same interface via a method that returns the conversation with items loaded.

**Alternative considered**:
- Put in a new `pkg/conversations/` package: Over-segmented for what is essentially a storage extension.
- Extend ResponseStore: Wrong abstraction; conversations are not responses.

## R2: Conversation ID Format

**Decision**: Use `conv_` prefix with 24 random alphanumeric characters, consistent with `resp_`, `item_`, `file_`, `batch_`.

## R3: Conversation-Response Linking

**Decision**: Add `conversation_id` as an optional field on `CreateResponseRequest` and `Response`. When set, the engine stores the response's input and output items as conversation items. The conversation tracks item ordering by position (monotonically increasing integer).

**Rationale**: Items are already stored on responses. The conversation just tracks ordering (which items, in what order). This avoids duplicating item data.

## R4: History Reconstruction from Conversation

**Decision**: When `conversation_id` is set without `previous_response_id`, the engine loads all items from the conversation (via ConversationStore.ListItems), converts them to ProviderMessages, and uses them as the message history. This replaces the response-chain-following logic of `loadConversationHistory`.

**Rationale**: Conversations provide a flat, ordered item list. No chain-following needed. This is simpler and more predictable than `previous_response_id` chains.

## R5: Route Placement

**Decision**: Add conversation routes directly to the HTTP adapter's mux, alongside response routes. Not via FunctionProvider (conversations are a core API, not a tool).

Routes:
- `POST /v1/conversations`
- `GET /v1/conversations`
- `GET /v1/conversations/{id}`
- `DELETE /v1/conversations/{id}`
- `POST /v1/conversations/{id}/items`
- `GET /v1/conversations/{id}/items`

## R6: In-Memory Implementation

**Decision**: In-memory ConversationStore follows the same pattern as the memory ResponseStore: RWMutex, map-based lookup, soft delete, tenant scoping, LRU optional.
