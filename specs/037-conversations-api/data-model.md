# Data Model: 037-conversations-api

**Date**: 2026-03-03

## Entities

### Conversation

| Field     | Type           | Description                              |
|-----------|----------------|------------------------------------------|
| ID        | string         | Unique identifier, `conv_` + 24 alphanum |
| Object    | string         | Always `"conversation"`                  |
| Name      | string         | Optional human-readable label            |
| UserID    | string         | Owner identity (from auth context)       |
| CreatedAt | int64          | Unix timestamp of creation               |
| UpdatedAt | int64          | Unix timestamp of last item addition     |
| Metadata  | map[string]any | Optional user-defined key-value pairs    |

### ConversationItem

Links an item to a conversation with ordering.

| Field          | Type   | Description                                |
|----------------|--------|--------------------------------------------|
| ConversationID | string | Parent conversation ID                     |
| ItemID         | string | ID of the item (from Response.Input/Output)|
| Item           | Item   | The actual item data                       |
| Position       | int    | Chronological position within conversation |
| CreatedAt      | int64  | Unix timestamp when added                  |

## Interfaces

### ConversationStore (5 methods)

| Method    | Input                                     | Output                       |
|-----------|-------------------------------------------|------------------------------|
| Save      | ctx, conv *Conversation                   | error                        |
| Get       | ctx, id string                            | *Conversation, error         |
| Delete    | ctx, id string                            | error                        |
| List      | ctx, opts ListOptions                     | *ConversationList, error     |
| AddItems  | ctx, convID string, items []ConvItem      | error                        |

Item listing uses a separate method on the store or is retrieved via Get with items loaded.

**Implementations**: In-memory (default), PostgreSQL (adapter, future)

## List Types

### ConversationList

| Field   | Type             | Description        |
|---------|------------------|--------------------|
| Object  | string           | Always `"list"`    |
| Data    | []*Conversation  | Page of results    |
| HasMore | bool             | More pages exist   |
| FirstID | string           | First item cursor  |
| LastID  | string           | Last item cursor   |

## Relationships

```
User 1--* Conversation (user-scoped)
Conversation 1--* ConversationItem (ordered by position)
ConversationItem *--1 Item (from Response.Input or Response.Output)
Response 0..1--1 Conversation (optional conversation_id link)
```

## Request Extensions

### CreateResponseRequest (extended)

| Field          | Type   | Description                                  |
|----------------|--------|----------------------------------------------|
| ConversationID | string | Optional. Links response to a conversation.  |

### Response (extended)

| Field          | Type    | Description                                  |
|----------------|---------|----------------------------------------------|
| ConversationID | *string | Conversation this response belongs to.       |
