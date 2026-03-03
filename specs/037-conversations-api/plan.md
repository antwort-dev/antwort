# Implementation Plan: Conversations API

**Branch**: `037-conversations-api` | **Date**: 2026-03-03 | **Spec**: [spec.md](spec.md)

## Summary

Add explicit conversation management to antwort. A new `ConversationStore` interface (alongside `ResponseStore`) supports CRUD operations for named conversations and item management. The HTTP adapter gains 6 new endpoints under `/v1/conversations`. The engine integrates via an optional `conversation_id` field on the create response request, using conversation items as LLM context. Backward compatible with `previous_response_id`.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Go standard library only
**Storage**: In-memory ConversationStore (default). PostgreSQL adapter (future, extends existing pgx store).
**Testing**: `go test` with table-driven tests and httptest
**Project Type**: Web service (new API endpoints + engine integration)

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | PASS | ConversationStore (5 methods) |
| II. Zero External Deps | PASS | stdlib only |
| III. Nil-Safe | PASS | ConversationStore nil = conversations disabled |
| V. Validate Early | PASS | conversation_id validated before engine processing |
| Documentation | PASS | FR-018/019/020 mandate reference + tutorial pages |

## Design Decisions

### D1: ConversationStore Interface

New interface in `pkg/transport/handler.go`:

```go
type ConversationStore interface {
    SaveConversation(ctx, conv) error
    GetConversation(ctx, id) (*Conversation, error)
    DeleteConversation(ctx, id) error
    ListConversations(ctx, opts) (*ConversationList, error)
    ListConversationItems(ctx, convID, opts) (*ItemList, error)
}
```

5 methods (within constitution limit). Item addition happens via SaveConversation (updates item list).

### D2: Conversation Types

New types in `pkg/api/types.go`:
- `Conversation` struct with ID, Object, Name, CreatedAt, UpdatedAt, Metadata
- `ConversationID` generation with `conv_` prefix in `pkg/api/id.go`
- `ConversationList` pagination type

### D3: HTTP Routes

Added directly to the HTTP adapter mux (not via FunctionProvider, since conversations are a core API):

```
POST   /v1/conversations
GET    /v1/conversations
GET    /v1/conversations/{id}
DELETE /v1/conversations/{id}
POST   /v1/conversations/{id}/items
GET    /v1/conversations/{id}/items
```

### D4: Engine Integration

- Add `conversation_id` to `CreateResponseRequest`
- In the engine, when `conversation_id` is set and `previous_response_id` is not, load items from ConversationStore and convert to ProviderMessages
- After response completion, add input + output items to the conversation
- ConversationStore is an optional dependency on the Engine (nil = disabled)

### D5: In-Memory Store

`pkg/storage/memory/conversations.go`: RWMutex, map-based, tenant-scoped, soft delete. Follows the existing memory store pattern exactly.

## Project Structure

```text
pkg/api/types.go                    # Add Conversation, ConversationList types
pkg/api/id.go                       # Add conv_ ID generation
pkg/transport/handler.go            # Add ConversationStore interface
pkg/storage/memory/conversations.go # NEW: In-memory ConversationStore
pkg/transport/http/adapter.go       # Add conversation routes
pkg/transport/http/conversations.go # NEW: Conversation HTTP handlers
pkg/engine/engine.go                # Accept optional ConversationStore
pkg/engine/engine.go                # Integrate conversation_id into response flow
docs/modules/reference/pages/conversations-api.adoc  # NEW: API reference
docs/modules/tutorial/pages/conversations.adoc        # NEW: Usage guide
```
