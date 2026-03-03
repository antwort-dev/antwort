# API Contract: Conversations API

**Version**: 1.0 | **Base Path**: `/v1`

## POST /v1/conversations

Create a new conversation.

**Request Body**:
```json
{
  "name": "Project discussion",
  "metadata": {"project": "antwort"}
}
```

All fields are optional.

**Response** (201 Created):
```json
{
  "id": "conv_abc123def456ghi789jkl012",
  "object": "conversation",
  "name": "Project discussion",
  "created_at": 1709366400,
  "updated_at": 1709366400,
  "metadata": {"project": "antwort"}
}
```

---

## GET /v1/conversations

List conversations for the authenticated user.

**Query Parameters**: `after`, `limit` (default 20, max 100), `order` (asc/desc, default desc)

**Response** (200 OK): Standard list format with `object: "list"`, `data`, `has_more`, `first_id`, `last_id`.

---

## GET /v1/conversations/{id}

Retrieve a conversation by ID.

**Response** (200 OK): Same as single conversation object.

**Errors**: 404 if not found or belongs to another user.

---

## DELETE /v1/conversations/{id}

Delete a conversation. Underlying responses remain accessible.

**Response** (200 OK):
```json
{
  "id": "conv_abc123def456ghi789jkl012",
  "object": "conversation",
  "deleted": true
}
```

---

## POST /v1/conversations/{id}/items

Add an item directly to a conversation.

**Request Body**:
```json
{
  "type": "message",
  "role": "user",
  "content": "What is Kubernetes?"
}
```

**Response** (200 OK): The created item object.

---

## GET /v1/conversations/{id}/items

List items in a conversation with pagination.

**Query Parameters**: `after`, `limit`, `order`

**Response** (200 OK): Standard list format with items in chronological order.

---

## POST /v1/responses (extended)

The existing create response endpoint gains an optional `conversation_id` field:

```json
{
  "model": "/mnt/models",
  "conversation_id": "conv_abc123def456ghi789jkl012",
  "input": "Tell me more about Pods"
}
```

When `conversation_id` is set, the response is linked to the conversation and conversation history is used as LLM context.
