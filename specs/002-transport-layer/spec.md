# Feature Specification: Transport Layer

**Feature Branch**: `002-transport-layer`
**Created**: 2026-02-17
**Status**: Draft
**Input**: User description: "Transport Layer: HTTP/SSE adapter with OpenResponses streaming protocol, request routing, middleware chain, and connection lifecycle management for the antwort API server"

## Overview

This specification defines the transport layer for antwort, which accepts client connections over HTTP, routes requests to the appropriate handler, streams responses via Server-Sent Events (SSE), and provides a middleware chain for cross-cutting concerns.

The transport layer is the bridge between external clients and antwort's internal processing engine. It deserializes incoming requests into the core protocol types defined in Spec 001, dispatches them for processing, and serializes responses back to the client in either synchronous (JSON) or streaming (SSE) format.

This spec covers the HTTP/SSE adapter only. gRPC support is deferred to a future spec.

## Clarifications

### Session 2026-02-17

- Q: Should the transport layer define the handler interface now, or wait for the core engine? -> A: Define it now. The three operations (create, get, delete) are directly from the OpenResponses spec and stable. Split into two interfaces: ResponseCreator (create, both tiers) and ResponseStore (get/delete, stateful only).
- Q: How should cancellation work? -> A: Context-only. Client disconnect is detected via the request context. Explicit cancel via DELETE looks up in-flight cancel functions. Timeouts use context wrapping. The engine just respects context cancellation.
- Q: Should auth and rate limiting middleware be part of this spec? -> A: No. The middleware chain mechanism is defined here, but auth and rate-limit middleware are extension points filled by Spec 06.
- Q: Should we follow the OpenAI `[DONE]` sentinel convention? -> A: Yes. Send `data: [DONE]` after the terminal event to match OpenAI client expectations.
- Q: Should ext_proc be included? -> A: No. Antwort operates as a standalone server, not as an Envoy filter.
- Q: Are external dependencies allowed for Spec 002? -> A: No. Standard library only, consistent with Spec 001. Use net/http for serving, log/slog for structured logging, Go 1.22 routing patterns for method+path dispatch.
- Q: Who is responsible for business-level request validation (ValidateRequest)? -> A: The handler (core engine). The transport layer only validates well-formed JSON. Business validation (model required, input non-empty, etc.) is the handler's responsibility.

## User Scenarios & Testing

### User Story 1 - Submit a Non-Streaming Request and Receive a JSON Response (Priority: P1)

A developer sends a `POST /v1/responses` request with `stream: false` (or omitted) and receives a complete JSON response containing the model's output. The response matches the OpenResponses schema exactly, so standard OpenAI-compatible clients work without modification.

**Why this priority**: This is the simplest end-to-end path through the transport layer. It validates request deserialization, handler dispatch, response serialization, and error formatting. Every other user story builds on this foundation.

**Independent Test**: Can be fully tested by sending an HTTP request to the server with a mock handler that returns a fixed response, then verifying the HTTP status code, content type, and response body match the expected OpenResponses format.

**Acceptance Scenarios**:

1. **Given** a running server with a mock handler, **When** a valid `POST /v1/responses` request is sent with `stream: false`, **Then** the server returns HTTP 200 with `Content-Type: application/json` and a valid Response object
2. **Given** a running server, **When** a request is sent with an invalid JSON body, **Then** the server returns HTTP 400 with an ErrorResponse containing `type: "invalid_request"` and a descriptive message
3. **Given** a running server, **When** a request is sent with a valid body but the handler returns an error, **Then** the server maps the error type to the correct HTTP status code (400 for invalid_request, 404 for not_found, 429 for too_many_requests, 500 for server_error)
4. **Given** a running server, **When** a request is sent to an unknown path, **Then** the server returns HTTP 404
5. **Given** a running server configured without a response store, **When** a `GET /v1/responses/{id}` request is sent, **Then** the server returns an appropriate error indicating the operation is not available

---

### User Story 2 - Submit a Streaming Request and Receive SSE Events (Priority: P1)

A developer sends a `POST /v1/responses` request with `stream: true` and receives a stream of Server-Sent Events following the OpenResponses protocol. Events arrive incrementally as the model generates output. The stream concludes with a terminal event followed by the `[DONE]` sentinel.

**Why this priority**: Streaming is the primary consumption mode for LLM APIs. Without correct SSE formatting and event delivery, the transport layer cannot serve real-time inference workloads.

**Independent Test**: Can be fully tested by sending a streaming request to the server with a mock handler that emits a sequence of events, then verifying the SSE wire format (event type headers, JSON data lines, blank line separators, `[DONE]` sentinel).

**Acceptance Scenarios**:

1. **Given** a running server with a mock handler that emits text delta events, **When** a `POST /v1/responses` request is sent with `stream: true`, **Then** the server returns HTTP 200 with `Content-Type: text/event-stream` and each event is formatted as `event: {type}\ndata: {json}\n\n`
2. **Given** a streaming response in progress, **When** the handler emits a `response.completed` event, **Then** the server sends the event followed by `data: [DONE]\n\n` and closes the connection
3. **Given** a streaming response in progress, **When** the handler emits a `response.failed` event, **Then** the server sends the event followed by `data: [DONE]\n\n` and closes the connection
4. **Given** a streaming response in progress, **When** the client disconnects, **Then** the server detects the disconnection and cancels the handler's context
5. **Given** a streaming response, **When** events are emitted, **Then** each event is flushed to the client immediately (no buffering delays)

---

### User Story 3 - Retrieve and Delete Stored Responses (Priority: P2)

A developer retrieves a previously created response by ID via `GET /v1/responses/{id}`, or deletes one via `DELETE /v1/responses/{id}`. These endpoints are available only when the server is configured with a response store (stateful mode).

**Why this priority**: These endpoints complete the CRUD surface of the OpenResponses API. They depend on the create path (US1) being functional and require a response store, making them second-priority.

**Independent Test**: Can be fully tested with a mock response store, sending GET and DELETE requests and verifying correct status codes and response bodies.

**Acceptance Scenarios**:

1. **Given** a server with a mock response store containing a response with ID `resp_abc123`, **When** a `GET /v1/responses/resp_abc123` request is sent, **Then** the server returns HTTP 200 with the stored Response object
2. **Given** a server with a mock response store, **When** a `GET /v1/responses/resp_nonexistent` request is sent, **Then** the server returns HTTP 404 with an ErrorResponse containing `type: "not_found"`
3. **Given** a server with a mock response store, **When** a `DELETE /v1/responses/resp_abc123` request is sent, **Then** the server returns HTTP 204 confirming deletion
4. **Given** a server with a mock response store, **When** a `DELETE /v1/responses/resp_nonexistent` request is sent, **Then** the server returns HTTP 404

---

### User Story 4 - Middleware Chain for Cross-Cutting Concerns (Priority: P2)

A server operator configures middleware to add cross-cutting behavior to all requests. The transport layer provides built-in middleware for panic recovery, request ID assignment, and structured logging. Additional middleware can be added for custom concerns.

**Why this priority**: Middleware is essential for production operation (recovery from panics, request tracing, observability), but the core request/response path (US1, US2) must work first.

**Independent Test**: Can be tested by configuring middleware that records invocation order, sending a request, and verifying that middleware executed in the correct sequence. Recovery middleware can be tested by triggering a panic in the handler and verifying a 500 response is returned instead of a connection drop.

**Acceptance Scenarios**:

1. **Given** a server with recovery middleware, **When** the handler panics during request processing, **Then** the server returns HTTP 500 with an ErrorResponse instead of dropping the connection
2. **Given** a server with request ID middleware, **When** a request is processed, **Then** the response includes a unique request ID header and the request ID is available to the handler via the request context
3. **Given** a server with logging middleware, **When** a request completes, **Then** a structured log entry is emitted containing the method, path, status code, duration, and request ID
4. **Given** a server with multiple middleware, **When** a request is processed, **Then** middleware executes in the configured order (recovery outermost, then request ID, then logging, then custom middleware)

---

### User Story 5 - Cancel In-Flight Streaming Response (Priority: P3)

A developer cancels an in-flight streaming response by sending `DELETE /v1/responses/{id}` while the response is still being generated. The server cancels the handler's context, causing it to stop processing and emit a `response.cancelled` event.

**Why this priority**: Cancellation is important for resource management but is a secondary concern compared to the primary request/response and streaming paths.

**Independent Test**: Can be tested by starting a slow streaming response with a mock handler, sending a DELETE request to the same response ID, and verifying that the handler's context is cancelled and the stream terminates with a cancellation event.

**Acceptance Scenarios**:

1. **Given** a streaming response in progress for response ID `resp_abc123`, **When** a `DELETE /v1/responses/resp_abc123` request is sent, **Then** the handler's context is cancelled and the streaming response terminates
2. **Given** a streaming response that has been cancelled, **When** the response stream terminates, **Then** a `response.cancelled` event is sent followed by `data: [DONE]\n\n`
3. **Given** no in-flight response for ID `resp_xyz`, **When** a `DELETE /v1/responses/resp_xyz` is sent, **Then** the request falls through to the response store (if configured) for standard deletion

---

### Edge Cases

- What happens when the request body exceeds the maximum allowed size? The server returns HTTP 413 with an ErrorResponse before attempting to parse the body.
- What happens when the client sends a request with an unsupported Content-Type? The server returns HTTP 415 with an ErrorResponse.
- What happens when the server shuts down while streaming responses are in progress? The server initiates graceful shutdown, waiting for in-flight responses to complete or reach a deadline before forcefully closing connections.
- What happens when WriteEvent fails mid-stream (client disconnected but context not yet cancelled)? The server detects the write failure, cancels the handler context, and stops sending events.
- What happens when the response ID in a GET/DELETE path is malformed? The server returns HTTP 400 with a validation error.
- What happens when a streaming request's handler returns an error before any events are sent? The server returns a standard JSON error response (not SSE format) since no streaming has begun.

## Requirements

### Functional Requirements

**Request Handling**

- **FR-001**: System MUST accept `POST /v1/responses` requests with `Content-Type: application/json` and deserialize the body into a CreateResponseRequest
- **FR-002**: System MUST accept `GET /v1/responses/{id}` requests and return the stored response as JSON, when a response store is configured
- **FR-003**: System MUST accept `DELETE /v1/responses/{id}` requests. The system first checks the in-flight registry for an active streaming response with that ID (and cancels it if found). If no in-flight response exists, the request is delegated to the response store for standard deletion (when configured).
- **FR-004**: System MUST validate that the request body is well-formed JSON before dispatching to the handler. Business-level validation (model required, input constraints, etc.) is the handler's responsibility, not the transport layer's.
- **FR-005**: System MUST enforce a configurable maximum request body size and reject oversized requests with HTTP 413
- **FR-006**: For `POST /v1/responses`, the system MUST reject requests with a Content-Type other than `application/json` with HTTP 415
- **FR-006a**: System MUST return HTTP 405 Method Not Allowed for valid paths with unsupported HTTP methods (e.g., `PUT /v1/responses`, `PATCH /v1/responses/{id}`)

**Response Serialization**

- **FR-007**: For non-streaming requests (`stream: false` or omitted), the system MUST return the complete Response as JSON with `Content-Type: application/json`
- **FR-008**: For streaming requests (`stream: true`), the system MUST return events as Server-Sent Events with `Content-Type: text/event-stream`
- **FR-009**: Each SSE event MUST be formatted as `event: {event_type}\ndata: {json_payload}\n\n` where the event type is the StreamEventType value and the JSON payload is the serialized StreamEvent
- **FR-010**: After the terminal event (`response.completed`, `response.failed`, or `response.cancelled`), the system MUST send `data: [DONE]\n\n` and close the connection
- **FR-011**: Each event MUST be flushed to the client immediately after serialization (no buffering across events)
- **FR-012**: SSE responses MUST include `Cache-Control: no-cache` and `Connection: keep-alive` headers

**Error Handling**

- **FR-013**: Handler errors MUST be mapped to HTTP status codes: `invalid_request` to 400, `not_found` to 404, `too_many_requests` to 429, `server_error` and `model_error` to 500. Transport-level errors (body too large, unsupported Content-Type, method not allowed) MUST use the `invalid_request` error type with the corresponding HTTP status code (413, 415, 405).
- **FR-014**: All error responses MUST use the ErrorResponse wrapper format defined in Spec 001
- **FR-015**: If a handler returns an error before any streaming events have been sent, the system MUST respond with a standard JSON error response (not SSE)
- **FR-016**: If an error occurs after streaming has begun, the system MUST send a `response.failed` event with the error details, followed by `data: [DONE]\n\n`

**Handler Interface**

- **FR-017**: The system MUST define a ResponseCreator interface for creating responses (the core operation available in both stateless and stateful modes)
- **FR-018**: The system MUST define a ResponseStore interface for retrieving and deleting responses (stateful mode only)
- **FR-019**: The HTTP adapter MUST accept a ResponseCreator (required) and an optional ResponseStore. When the store is not provided, GET and DELETE endpoints return an error indicating the operation is unavailable.
- **FR-020**: The system MUST define a ResponseWriter interface with distinct operations for streaming and non-streaming responses: one operation for sending individual streaming events, one for sending a complete non-streaming response, and one for flushing buffered data. Calling the streaming event operation after a terminal event has been sent MUST return an error. The streaming and non-streaming operations are mutually exclusive on a single writer instance.

**Connection Lifecycle**

- **FR-021**: When a client disconnects during a streaming response, the system MUST cancel the handler's context within 1 second
- **FR-022**: The system MUST support graceful shutdown, waiting for in-flight requests to complete within a configurable deadline (default: 30 seconds) before forcefully closing connections
- **FR-023**: For explicit cancellation via DELETE, the system MUST maintain a registry of in-flight streaming responses and their cancel functions, keyed by response ID

**Middleware**

- **FR-024**: The system MUST define a middleware mechanism that wraps the ResponseCreator interface to add cross-cutting behavior
- **FR-025**: The system MUST provide a recovery middleware that catches panics in the handler and returns HTTP 500 instead of dropping the connection
- **FR-026**: The system MUST provide a request ID middleware that assigns a unique identifier to each request, makes it available via the request context, and returns it in an `X-Request-ID` response header. If the incoming request already carries an `X-Request-ID` header, the middleware MUST use that value instead of generating a new one.
- **FR-027**: The system MUST provide a logging middleware that emits structured log entries for each request, including method, path, status code, duration, and request ID
- **FR-028**: Middleware MUST execute in a defined, configurable order: recovery (outermost), request ID, logging, then any custom middleware (innermost)

### Key Entities

- **ResponseCreator**: The handler contract for processing create-response requests. Accepts a request and a ResponseWriter, returning an error if processing fails. Used by both stateless and stateful deployments.
- **ResponseStore**: The handler contract for retrieving and deleting stored responses. Only used in stateful deployments with persistence.
- **ResponseWriter**: The output abstraction provided by the transport layer to the handler. Supports writing individual streaming events or a complete response. Handles SSE formatting internally.
- **Middleware**: A wrapper around ResponseCreator that adds cross-cutting behavior (recovery, request ID, logging). Composable in a chain.
- **In-Flight Registry**: A mapping from response ID to cancel function for in-flight streaming responses. Used to support explicit cancellation via DELETE while a response is being generated.

## Success Criteria

### Measurable Outcomes

- **SC-001**: An OpenAI-compatible client (using the standard OpenAI SDK or any SSE client) can send requests and receive both streaming and non-streaming responses without any client-side modifications
- **SC-002**: All five error types defined in Spec 001 map to the correct HTTP status codes, and error responses match the ErrorResponse format exactly
- **SC-003**: Streaming events are flushed immediately with no application-layer buffering; events arrive at the client as soon as network conditions allow
- **SC-004**: Client disconnection during streaming is detected and the handler context is cancelled within 1 second
- **SC-005**: A panic in the handler results in an HTTP 500 error response (not a dropped connection), and the server continues to accept new requests
- **SC-006**: Graceful shutdown completes within the configured deadline, with all in-flight requests either completed or cancelled
- **SC-007**: Each request has a unique request ID that appears in both the response headers and the structured log output, enabling end-to-end request tracing

## Assumptions

- The handler interface (ResponseCreator, ResponseStore) will be implemented by a core engine defined in a later spec. For Spec 002, all testing uses mock/stub handlers.
- The transport layer operates over HTTP/1.1 and HTTP/2. SSE works over both protocols without transport-specific adaptation.
- Request body size limit defaults to 10 MB (matching the MaxContentSize from Spec 001's ValidationConfig), configurable at server startup.
- The server listens on a single configurable address (host:port). TLS termination is handled externally (by a load balancer, Kubernetes ingress, or reverse proxy).
- The transport layer uses only Go standard library packages (zero external dependencies), consistent with Spec 001. This includes `net/http` for serving, `log/slog` for structured logging, and Go 1.22+ `http.ServeMux` routing patterns.
- Structured logging uses key-value pairs via `log/slog`. The specific output format (JSON, text) is a deployment concern, not specified here.

## Dependencies

- **Spec 001 (Core Protocol & Data Model)**: All request, response, event, and error types are defined in `pkg/api`. The transport layer depends on these types and does not redefine them.

## Scope Boundaries

### In Scope

- HTTP/SSE adapter with full OpenResponses streaming protocol
- Handler interfaces (ResponseCreator, ResponseStore, ResponseWriter)
- Middleware chain mechanism with built-in recovery, request ID, and logging middleware
- Connection lifecycle (graceful shutdown, client disconnect detection, explicit cancellation)
- Request routing for the three OpenResponses endpoints
- Error mapping from handler errors to HTTP status codes

### Out of Scope

- gRPC adapter (deferred to a future spec)
- Authentication and authorization middleware (Spec 06)
- Rate limiting middleware (Spec 06)
- Core engine / business logic (Spec 03+)
- Provider communication (Spec 03)
- Persistence / storage (Spec 05)
- TLS configuration (handled externally)
- Metrics and tracing middleware (Spec 07)
- CORS headers (antwort is a backend API; browser access is handled by a reverse proxy)
