# Feature Specification: State Management & Storage

**Feature Branch**: `005-storage`
**Created**: 2026-02-18
**Status**: Draft
**Input**: User description: "State Management and Storage for the Antwort OpenResponses gateway, covering the storage interface for persisting responses, PostgreSQL adapter, in-memory adapter, conversation chain reconstruction, schema migrations, and multi-tenancy support via tenant context."

## Overview

This specification defines the persistence layer for antwort. It extends the existing `ResponseStore` interface (from Spec 002) with a `SaveResponse` operation, implements a PostgreSQL adapter for production deployments, and provides an in-memory adapter for testing and lightweight deployments. The storage layer enables the stateful API tier: response persistence, retrieval, deletion, and conversation chaining via `previous_response_id`.

Multi-tenancy is supported through tenant context propagation. Storage implementations extract the tenant identifier from the request context and scope all operations to that tenant. When no tenant is present (single-tenant mode), storage operates without scoping, preserving backward compatibility.

## Clarifications

### Session 2026-02-18

- Q: Should `BuildContext` (chain reconstruction) be part of the store interface or stay in the engine? -> A: The engine's `loadConversationHistory` (Spec 003) remains the default implementation, working with any store via GetResponse calls. The PostgreSQL adapter may optimize chain reconstruction internally (e.g., using a recursive CTE), but this is an implementation detail, not an interface requirement.
- Q: Should truncation logic be in the store? -> A: No. The store returns the full chain. The engine handles truncation based on the request's `truncation` setting.
- Q: Should streaming writes (saving partial responses during inference) be supported? -> A: No. Responses are saved only after inference completes. Partial saves add complexity with no clear benefit.
- Q: Soft delete or hard delete? -> A: Soft delete (set `deleted_at` timestamp). This preserves chain integrity for `previous_response_id` references. Deleted responses are excluded from retrieval but their chain links remain valid.
- Q: Should `tenant_id` be in the initial schema? -> A: Yes, with `DEFAULT ''` so single-tenant deployments work without changes. No migration required when auth is later enabled.
- Q: What about database credentials in Kubernetes? -> A: Connection strings should support Kubernetes Secret references. TLS for database connections is required in production. These are deployment concerns (Spec 07) but the storage adapter must support TLS connection parameters.
- Q: How does chain reconstruction access soft-deleted responses when GetResponse returns not-found? -> A: The store provides a separate `GetResponseForChain` method (or an internal variant) that includes soft-deleted responses. The engine's `loadConversationHistory` will be updated to use this chain-aware retrieval. Regular `GetResponse` (used by the HTTP GET endpoint) still excludes deleted responses.
- Q: When is SaveResponse called relative to the client response? -> A: SaveResponse is called AFTER the response is written to the client. If SaveResponse fails, the client already has the response, but it won't be retrievable later. A warning is logged on save failure.

## User Scenarios & Testing

### User Story 1 - Persist and Retrieve a Response (Priority: P1)

A developer sends a `POST /v1/responses` request with `store: true` (the default). After inference completes, the engine saves the response (including input items, output items, usage, and status) to the store. The developer can later retrieve the response via `GET /v1/responses/{id}` and receives the complete response object.

**Why this priority**: This is the fundamental storage operation. Without save and retrieve, the stateful API tier is non-functional.

**Independent Test**: Save a response with known content, retrieve it by ID, verify all fields match. Delete it, verify retrieval returns not found.

**Acceptance Scenarios**:

1. **Given** a store is configured, **When** a response is saved after inference, **Then** it can be retrieved by ID with all fields intact (input, output, status, usage, model, timestamps)
2. **Given** a saved response, **When** it is deleted, **Then** subsequent retrieval returns a not-found error
3. **Given** a response with `store: false`, **When** inference completes, **Then** the response is NOT saved to the store
4. **Given** a store is not configured (nil), **When** a response completes, **Then** the engine skips storage gracefully (no error)

---

### User Story 2 - Conversation Chaining with Stored Responses (Priority: P1)

A developer sends a follow-up request with `previous_response_id` pointing to a previously stored response. The engine retrieves the referenced response (and its chain of predecessors) from the store, reconstructs the conversation history, and sends the full context to the model. The new response is also stored with its `previous_response_id` link.

**Why this priority**: Conversation chaining is the primary use case for stateful storage. Without it, multi-turn conversations require the client to manage all history.

**Independent Test**: Store 3 chained responses (A -> B -> C). Send a request referencing C. Verify the engine reconstructs messages from A, B, and C in chronological order.

**Acceptance Scenarios**:

1. **Given** a chain of stored responses A -> B -> C, **When** a request references response C, **Then** the engine retrieves and reconstructs messages from A, B, and C in chronological order
2. **Given** a `previous_response_id` referencing a deleted response, **When** the engine reconstructs the chain, **Then** the deleted response's data is still available for chain reconstruction (soft delete preserves chain integrity)
3. **Given** a `previous_response_id` referencing a non-existent ID, **When** the engine attempts retrieval, **Then** it returns a not-found error

---

### User Story 3 - In-Memory Store for Testing and Lightweight Deployments (Priority: P2)

A developer runs antwort in a testing or development environment without a database. The in-memory store provides the same interface as the PostgreSQL store but without durability. Responses are stored in memory and lost when the process restarts. Optional size limits prevent unbounded memory growth.

**Why this priority**: An in-memory store enables testing the full stateful API without database infrastructure and supports lightweight deployments.

**Independent Test**: Create an in-memory store, save and retrieve responses, verify all operations work. Verify LRU eviction when max size is reached.

**Acceptance Scenarios**:

1. **Given** an in-memory store, **When** responses are saved and retrieved, **Then** all operations behave identically to the persistent store
2. **Given** an in-memory store with a max size of 100, **When** the 101st response is saved, **Then** the oldest response is evicted
3. **Given** an in-memory store, **When** the process restarts, **Then** all previously stored responses are lost (no durability)

---

### User Story 4 - PostgreSQL Persistence with Schema Migrations (Priority: P2)

An operator deploys antwort with a PostgreSQL database. On startup, the storage adapter optionally runs schema migrations to create or update the required tables. The adapter manages a connection pool and supports health checks for Kubernetes readiness probes.

**Why this priority**: PostgreSQL is the production storage backend. Migrations and health checks are essential for operational readiness.

**Independent Test**: Start a PostgreSQL instance, run migrations, save and retrieve responses, verify schema is correctly created. Verify health check returns healthy.

**Acceptance Scenarios**:

1. **Given** a fresh PostgreSQL database, **When** migrations run on startup, **Then** the required tables and indexes are created
2. **Given** a running PostgreSQL adapter, **When** a health check is requested, **Then** it returns healthy if the database is reachable and unhealthy otherwise
3. **Given** a PostgreSQL connection pool, **When** many concurrent requests access the store, **Then** the pool manages connections within configured limits

---

### User Story 5 - Multi-Tenant Response Isolation (Priority: P3)

An operator deploys antwort with authentication enabled. Each authenticated user (or tenant) can only see their own responses. Tenant isolation is enforced at the storage layer: all queries are scoped to the tenant extracted from the request context. Responses stored without a tenant (before auth was enabled) remain accessible only in single-tenant mode.

**Why this priority**: Multi-tenancy is important for shared deployments but requires the auth system (Spec 006) to provide tenant identity.

**Independent Test**: Store responses for tenant A and tenant B. Verify tenant A cannot retrieve tenant B's responses. Verify responses with no tenant are accessible only when no auth is configured.

**Acceptance Scenarios**:

1. **Given** two tenants A and B, **When** tenant A stores a response, **Then** tenant B cannot retrieve it
2. **Given** a deployment with no auth configured, **When** responses are stored, **Then** they have an empty tenant identifier and are accessible to all
3. **Given** an existing deployment that enables auth, **When** old responses have empty tenant IDs, **Then** they are only visible when no tenant filtering is active (backward compatible)

---

### Edge Cases

- What happens when the database is unreachable during SaveResponse? The engine returns a server error. The response is still sent to the client (inference already completed) but is not persisted. A warning is logged.
- What happens when SaveResponse is called with a duplicate response ID? The store returns a conflict error. The engine logs a warning but does not fail the request (the response was already sent to the client).
- What happens when the chain reconstruction encounters a cycle? The engine's existing cycle detection (Spec 003, `history.go`) handles this, not the store.
- What happens when the in-memory store reaches its max size during concurrent writes? Eviction is synchronized (mutex-protected). Only one eviction runs at a time.
- What happens when a migration fails? The application startup fails with a clear error message. The operator must fix the database state before retrying.
- What happens when GetResponse is called for a soft-deleted response? It returns not-found. The soft-deleted data is only accessible for chain reconstruction purposes.

## Requirements

### Functional Requirements

**Storage Interface**

- **FR-001**: The system MUST extend the existing `ResponseStore` interface with a `SaveResponse` operation that persists a completed response including all fields (ID, status, model, input items, output items, usage, error, previous_response_id, timestamps, extensions)
- **FR-002**: The `GetResponse` operation MUST return a complete response object for a given ID, or a not-found error if the response does not exist or has been deleted
- **FR-003**: The `DeleteResponse` operation MUST soft-delete a response (mark as deleted) rather than removing it. Soft-deleted responses MUST be excluded from GetResponse but MUST remain accessible for chain reconstruction.
- **FR-004**: The storage interface MUST include a `HealthCheck` operation that verifies the store connection is functional
- **FR-005**: The storage interface MUST include a `Close` operation that releases database connections and resources

**Response Persistence**

- **FR-006**: The engine MUST save the response after inference completes when `store: true` (the default) and a store is configured
- **FR-007**: The engine MUST skip storage when `store: false` is set or when no store is configured (nil-safe)
- **FR-008**: The saved response MUST include both input items and output items, enabling full conversation reconstruction from stored data
- **FR-009**: The response ID MUST be the primary key. Duplicate saves MUST return a conflict error.

**In-Memory Adapter**

- **FR-010**: The system MUST provide an in-memory store implementation that satisfies the storage interface
- **FR-011**: The in-memory store MUST support optional LRU eviction with a configurable maximum size
- **FR-012**: The in-memory store MUST be safe for concurrent access

**PostgreSQL Adapter**

- **FR-013**: The system MUST provide a PostgreSQL store implementation that satisfies the storage interface
- **FR-014**: The PostgreSQL adapter MUST manage a connection pool with configurable maximum connections, idle connections, and connection lifetime
- **FR-015**: The PostgreSQL adapter MUST support automatic schema migrations on startup (configurable, not mandatory)
- **FR-016**: The PostgreSQL adapter MUST store items (input, output) as structured data that supports efficient querying
- **FR-017**: The PostgreSQL adapter MUST support TLS connections to the database

**Multi-Tenancy**

- **FR-018**: Storage operations MUST be scoped to a tenant when a tenant identifier is present in the request context
- **FR-019**: When no tenant identifier is present (single-tenant mode), storage operations MUST operate without scoping (backward compatible)
- **FR-020**: The tenant identifier MUST be stored alongside each response for filtering purposes
- **FR-021**: A tenant MUST NOT be able to access, modify, or delete another tenant's responses

**Chain Reconstruction**

- **FR-022**: The engine's existing chain reconstruction logic (Spec 003 `loadConversationHistory`) MUST work with the store for chain traversal
- **FR-023**: The storage interface MUST provide a chain-aware retrieval operation that returns responses including soft-deleted ones. This operation is used exclusively by chain reconstruction. The regular `GetResponse` operation (used by HTTP GET endpoint) MUST exclude soft-deleted responses.
- **FR-024**: SaveResponse MUST be called after the response is written to the client. Save failures MUST NOT affect the client response. A warning MUST be logged on save failure.

### Key Entities

- **StoredResponse**: A persisted response with all its fields. Keyed by response ID. Includes tenant ID for multi-tenancy scoping.
- **ResponseStore** (extended): The interface for response persistence. Extended from Spec 002 with SaveResponse, HealthCheck, and Close operations.
- **TenantContext**: A request-scoped value carrying the tenant identifier. Extracted from the authenticated request by auth middleware (Spec 006).

## Success Criteria

### Measurable Outcomes

- **SC-001**: A complete save-retrieve-delete cycle works correctly: a response saved after inference can be retrieved by ID with all fields intact, and deleted responses return not-found
- **SC-002**: Conversation chaining works with stored responses: a chain of 3+ stored responses produces correct chronological message reconstruction
- **SC-003**: The in-memory store passes all the same functional tests as the PostgreSQL store (interface compliance)
- **SC-004**: The PostgreSQL adapter supports concurrent access from multiple engine instances sharing the same database
- **SC-005**: Schema migrations create the required tables and indexes on a fresh database without manual intervention
- **SC-006**: Multi-tenant isolation prevents cross-tenant response access: tenant A cannot retrieve tenant B's responses
- **SC-007**: Soft-deleted responses remain available for chain reconstruction: deleting an intermediate response in a chain does not break the chain
- **SC-008**: Health checks accurately reflect database connectivity status

## Assumptions

- The `ResponseStore` interface from Spec 002 (`GetResponse`, `DeleteResponse`) will be extended with `SaveResponse`, `HealthCheck`, and `Close`. This is a backward-compatible change (existing implementations add the new methods).
- The PostgreSQL adapter is the first external dependency in the project. It uses a PostgreSQL driver library, which is permitted by the constitution for adapter implementations (Principle II).
- The engine's existing `loadConversationHistory` (Spec 003) continues to work for chain reconstruction. The PostgreSQL adapter does not need to implement a separate BuildContext method.
- Multi-tenancy scoping depends on the auth system (Spec 006) to inject tenant identity into the context. Until auth is implemented, the storage layer operates in single-tenant mode.
- The `tenant_id` column is included in the initial schema with `DEFAULT ''` so no migration is required when auth is later enabled.
- Schema migrations are embedded in the application binary (no external migration tool required).

## Dependencies

- **Spec 001 (Core Protocol)**: Response, Item, Usage, and Error types that are persisted.
- **Spec 002 (Transport Layer)**: The `ResponseStore` interface that this spec extends.
- **Spec 003 (Core Engine)**: The engine's `CreateResponse` flow where SaveResponse is called, and `loadConversationHistory` which uses GetResponse for chain reconstruction.

## Scope Boundaries

### In Scope

- Extending `ResponseStore` with SaveResponse, HealthCheck, Close
- In-memory store implementation with LRU eviction
- PostgreSQL store implementation with connection pooling
- Schema definition and migration mechanism
- Soft delete for chain integrity
- Multi-tenancy via tenant context scoping
- Engine integration (calling SaveResponse after inference)
- Health check for Kubernetes readiness probes

### Out of Scope

- Tool state persistence (MCP session state, file indices) (Specs 10, 11)
- Caching layer (future optimization)
- Authentication and tenant identity injection (Spec 006)
- Database connection encryption key management (Spec 07)
- Response listing/search endpoints (not in OpenResponses core spec)
- Automatic retention policies (TTL-based cleanup)
- Read replicas or sharding strategies
