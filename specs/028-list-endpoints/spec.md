# Feature Specification: List Responses and Input Items Endpoints

**Feature Branch**: `028-list-endpoints`
**Created**: 2026-02-25
**Status**: Draft

## Overview

The gateway supports creating, retrieving, and deleting responses, but has no way to list stored responses or inspect their input items. The Agents SDK, Codex CLI, and frameworks use list and input_items endpoints for conversation management, history browsing, and debugging. This specification adds two new read-only endpoints that complete the response CRUD surface.

## User Scenarios & Testing

### User Story 1 - List Stored Responses (Priority: P1)

A developer retrieves a paginated list of stored responses, optionally filtered by model and sorted by creation time. This enables conversation history browsing, usage tracking, and management UIs.

**Why this priority**: The Agents SDK and Codex CLI expect this endpoint. Without it, there's no way to discover what responses exist in the store.

**Independent Test**: Create several responses, list them with pagination, verify correct ordering and cursor behavior.

**Acceptance Scenarios**:

1. **Given** stored responses, **When** the list endpoint is called without filters, **Then** it returns responses ordered by creation time (newest first) with cursor pagination
2. **Given** stored responses from different models, **When** the list endpoint is called with a model filter, **Then** only responses matching that model are returned
3. **Given** more responses than the page size, **When** the list endpoint is called, **Then** the response includes `has_more: true` and cursors for the next page
4. **Given** a cursor from a previous list response, **When** the list endpoint is called with `after` or `before`, **Then** the next/previous page of results is returned
5. **Given** no stored responses, **When** the list endpoint is called, **Then** an empty list is returned with `has_more: false`

---

### User Story 2 - Retrieve Input Items (Priority: P1)

A developer inspects what input items were sent with a stored response. This enables debugging (what did the user actually send?), audit trails, and conversation reconstruction.

**Why this priority**: Frameworks that rebuild conversation context need to access the original input items without re-parsing the full response.

**Independent Test**: Create a response with multiple input items, retrieve them via the input_items endpoint, verify they match.

**Acceptance Scenarios**:

1. **Given** a stored response with input items, **When** the input_items endpoint is called, **Then** it returns the input items in order with pagination
2. **Given** a response ID that doesn't exist, **When** the input_items endpoint is called, **Then** a 404 error is returned
3. **Given** a response with many input items, **When** the input_items endpoint is called with a limit, **Then** pagination cursors are provided

---

### User Story 3 - Tenant Isolation (Priority: P1)

When auth is enabled, list and input_items endpoints only return responses owned by the authenticated tenant. A user cannot list or inspect another tenant's responses.

**Why this priority**: Same isolation guarantee as GET and DELETE.

**Independent Test**: Create responses under different tenants, verify each tenant only sees their own.

**Acceptance Scenarios**:

1. **Given** auth enabled with tenant isolation, **When** a tenant lists responses, **Then** only that tenant's responses are returned
2. **Given** auth enabled, **When** a tenant requests input_items for another tenant's response, **Then** a 404 is returned (not 403, to avoid leaking existence)

---

### Edge Cases

- What happens when storage is not configured? Return 501 (same as GET and DELETE).
- What happens with invalid cursor values? Return 400 with a descriptive error.
- What happens when the `limit` parameter exceeds the maximum? Cap at the maximum (default 100).
- What happens when `order` is not `asc` or `desc`? Return 400.

## Requirements

### Functional Requirements

**List Responses Endpoint**

- **FR-001**: The system MUST provide a `GET /v1/responses` endpoint that returns a paginated list of stored responses
- **FR-002**: The endpoint MUST support cursor-based pagination with `after`, `before`, and `limit` query parameters
- **FR-003**: The response format MUST follow the OpenAI list format: `{"object": "list", "data": [...], "has_more": bool, "first_id": "...", "last_id": "..."}`
- **FR-004**: The endpoint MUST support a `model` query parameter to filter responses by model name
- **FR-005**: The endpoint MUST support an `order` query parameter (`asc` or `desc`, default `desc`)
- **FR-006**: The default page size MUST be 20, with a configurable maximum of 100

**Input Items Endpoint**

- **FR-007**: The system MUST provide a `GET /v1/responses/{id}/input_items` endpoint that returns the input items of a stored response
- **FR-008**: The endpoint MUST support cursor-based pagination with `after`, `before`, and `limit` query parameters
- **FR-009**: The response format MUST follow the same OpenAI list format as the list responses endpoint
- **FR-010**: If the response ID does not exist, the endpoint MUST return 404

**Storage Interface**

- **FR-011**: The response store interface MUST be extended with methods for listing responses and retrieving input items
- **FR-012**: Both endpoints MUST return 501 when no storage backend is configured

**Auth and Isolation**

- **FR-013**: When auth is enabled, both endpoints MUST enforce tenant isolation (same rules as existing GET and DELETE)

**Spec Alignment**

- **FR-014**: The OpenAPI specification MUST be updated to include both new endpoints

## Success Criteria

- **SC-001**: The OpenAI Python SDK's `client.responses.list()` works against the list endpoint
- **SC-002**: The OpenAI Python SDK's `client.responses.input_items.list(response_id)` works against the input_items endpoint
- **SC-003**: Pagination produces correct results across multiple pages with no duplicates or gaps
- **SC-004**: All existing tests continue to pass with zero regressions

## Assumptions

- Cursor values are response IDs (for list) or item indices (for input_items). The cursor format is opaque to clients.
- The list endpoint returns response objects without the full output array (summary view). The full response is available via GET /v1/responses/{id}.
- Input items are stored as part of the response object (they already are, via `resp.Input`).
- The in-memory store supports listing and filtering. The PostgreSQL store can use SQL queries.

## Dependencies

- **Spec 005 (Storage)**: ResponseStore interface
- **Spec 002 (Transport Layer)**: HTTP handler registration

## Scope Boundaries

### In Scope

- GET /v1/responses with pagination, model filter, ordering
- GET /v1/responses/{id}/input_items with pagination
- Storage interface extension (ListResponses, GetInputItems methods)
- In-memory store implementation
- OpenAPI spec update
- Integration tests

### Out of Scope

- PostgreSQL store implementation (can be added later, in-memory is sufficient for now)
- Full-text search across responses
- Filtering by status, date range, or other fields beyond model
- Bulk delete
