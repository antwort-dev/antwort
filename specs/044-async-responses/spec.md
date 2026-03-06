# Feature Specification: Async Responses (Background Mode)

**Feature Branch**: `044-async-responses`
**Created**: 2026-03-05
**Status**: Draft
**Input**: User description: "brainstorm/39-async-responses.md"

## Clarifications

### Session 2026-03-05

- Q: Can failed background requests be retried or re-queued? → A: Failed is terminal; client resubmits a new request if needed.
- Q: Should failed background responses include partial output? → A: No, failed responses contain only error information, no partial output.

## User Scenarios & Testing

### User Story 1 - Fire-and-Forget Inference Request (Priority: P1)

An agent developer submits a complex inference request (multi-turn reasoning, large code generation) that will take minutes to complete. Rather than blocking the agent's control loop, the developer sets `background: true` on the request. The server accepts the request immediately and returns a response object with status `queued`. The agent continues its work and polls for the result later.

**Why this priority**: This is the core value proposition. Without the ability to submit and later retrieve background requests, no other background functionality matters.

**Independent Test**: Can be fully tested by submitting a request with `background: true` and verifying that the server returns immediately with a `queued` status response. Delivers immediate value by unblocking agent control loops.

**Acceptance Scenarios**:

1. **Given** a valid inference request with `background: true` and `store: true`, **When** the client submits the request, **Then** the server returns HTTP 200 with a response object containing `status: "queued"` and `background: true` within 1 second
2. **Given** a background request has been submitted, **When** the client polls GET /v1/responses/{id}, **Then** the server returns the current status (`queued`, `in_progress`, `completed`, or `failed`)
3. **Given** a background request has completed processing, **When** the client retrieves the response, **Then** the response contains the full output (identical to what a synchronous request would produce)

---

### User Story 2 - Distributed Worker Processing (Priority: P1)

A platform operator deploys Antwort with separate gateway and worker components. The gateway accepts HTTP requests and queues background work in shared storage. Worker pods poll for queued requests and process them independently. Gateway and worker pods scale independently based on their respective loads.

**Why this priority**: The distributed architecture is essential for production use. Background requests that compete with synchronous requests for resources defeat the purpose of async processing.

**Independent Test**: Can be tested by running gateway and worker as separate processes sharing a database. Submit a background request to the gateway and verify the worker picks it up and processes it to completion.

**Acceptance Scenarios**:

1. **Given** a gateway pod and a worker pod sharing the same database, **When** a background request is submitted to the gateway, **Then** the worker picks up the request and processes it to completion
2. **Given** multiple worker pods are running, **When** a background request is queued, **Then** exactly one worker claims and processes the request (no duplicate processing)
3. **Given** the server binary is started with `--mode=integrated`, **When** a background request is submitted, **Then** the same process handles both the HTTP request and the background processing (for development convenience)

---

### User Story 3 - Background Request Cancellation (Priority: P2)

An agent developer realizes a background request is no longer needed (the user navigated away, the task was superseded, or the agent's strategy changed). The developer cancels the request by deleting it. If the request is still queued or in progress, processing stops.

**Why this priority**: Without cancellation, long-running background requests consume resources unnecessarily. This is important for resource management but not required for basic functionality.

**Independent Test**: Can be tested by submitting a background request and immediately sending a DELETE request, then verifying the response transitions to `cancelled` status.

**Acceptance Scenarios**:

1. **Given** a background request with status `queued`, **When** the client sends DELETE /v1/responses/{id}, **Then** the request is cancelled and status changes to `cancelled`
2. **Given** a background request with status `in_progress`, **When** the client sends DELETE /v1/responses/{id}, **Then** the in-flight processing is cancelled (context cancellation) and status changes to `cancelled`
3. **Given** a background request with status `completed`, **When** the client sends DELETE /v1/responses/{id}, **Then** the response is deleted normally (same as synchronous response deletion)

---

### User Story 4 - Background Request Listing and Filtering (Priority: P2)

An agent developer or operator wants to see all background requests and their statuses. They filter the response list by status (`queued`, `in_progress`, `completed`, `failed`) and by background mode to monitor the system and identify stuck or failed requests.

**Why this priority**: Observability into background request state is essential for operations but not required for basic submit-and-poll functionality.

**Independent Test**: Can be tested by submitting multiple background requests in different states and verifying the list endpoint returns correct filtered results.

**Acceptance Scenarios**:

1. **Given** multiple background requests exist with different statuses, **When** the client sends GET /v1/responses?status=queued, **Then** only responses with status `queued` are returned
2. **Given** a mix of background and synchronous responses exist, **When** the client sends GET /v1/responses?background=true, **Then** only background responses are returned

---

### User Story 5 - Graceful Shutdown with Background Drain (Priority: P3)

A platform operator performs a rolling update or scales down worker pods. In-flight background requests are given time to complete before the process exits. Requests that cannot complete within the drain timeout are marked as `failed` so they can be retried or investigated.

**Why this priority**: Graceful shutdown prevents data loss and orphaned requests during routine operations, but is less critical than core submission and processing.

**Independent Test**: Can be tested by starting a worker, submitting a long-running background request, sending SIGTERM, and verifying the request completes or is marked as failed with a reason.

**Acceptance Scenarios**:

1. **Given** a worker is processing background requests, **When** SIGTERM is received, **Then** the worker stops accepting new requests and waits for in-flight requests to complete (up to a configurable drain timeout)
2. **Given** a worker receives SIGTERM with in-flight requests, **When** the drain timeout expires before all requests complete, **Then** remaining requests are marked as `failed` with a reason indicating shutdown timeout

---

### Edge Cases

- What happens when `background: true` is set with `store: false`? Validation error: background mode requires storage to persist and retrieve results.
- What happens when `background: true` is set with `stream: true`? Validation error: background mode and streaming are mutually exclusive.
- What happens when a worker crashes mid-processing? The request remains `in_progress` in storage. A stale request detection mechanism identifies requests that have been `in_progress` longer than a configurable timeout and marks them as `failed`. The client can then submit a new request if needed.
- What happens when the queue is full (all workers busy)? New background requests remain `queued` in storage. The system does not reject requests based on queue depth. Operators monitor queue depth via the list endpoint.
- What happens when background response TTL expires? Expired background responses are cleaned up automatically. Completed, failed, and cancelled responses older than the configured TTL are removed.

## Requirements

### Functional Requirements

- **FR-001**: System MUST accept `background` as a boolean field on response creation requests
- **FR-002**: When `background: true`, system MUST return a response with status `queued` immediately (within the same HTTP request, before processing begins)
- **FR-003**: System MUST validate that `background: true` requires `store: true`. If `store: false` is set with `background: true`, system MUST return a validation error
- **FR-004**: System MUST validate that `background: true` and `stream: true` are mutually exclusive. If both are set, system MUST return a validation error
- **FR-005**: System MUST support three operational modes via a `--mode` flag: `gateway` (accepts HTTP only), `worker` (processes background requests only), `integrated` (combines gateway and worker in one process)
- **FR-006**: In gateway mode, background requests MUST be persisted to storage with status `queued` and returned immediately to the client
- **FR-007**: In worker mode, the system MUST poll storage for `queued` requests, claim them atomically (preventing duplicate processing), and process them through the full engine pipeline (inference, agentic loop, tool execution)
- **FR-008**: Workers MUST update response status as processing progresses: `queued` to `in_progress` to `completed` (or `failed`). Failed responses MUST contain only error information, not partial output
- **FR-009**: Clients MUST be able to retrieve background response status and results via GET /v1/responses/{id}
- **FR-010**: Clients MUST be able to cancel queued or in-progress background requests via DELETE /v1/responses/{id}, which triggers context cancellation for in-flight processing. The `cancelled` status applies only to background responses; synchronous response deletion remains unchanged
- **FR-011**: The response list endpoint MUST support filtering by `status` and `background` query parameters
- **FR-012**: Workers MUST implement graceful shutdown: stop accepting new work on SIGTERM, drain in-flight requests up to a configurable timeout, and mark undrained requests as `failed`
- **FR-013**: System MUST detect stale `in_progress` requests (from crashed workers) and mark them as `failed` after a configurable staleness timeout. Stale detection runs as part of each worker's poll cycle
- **FR-014**: Background responses MUST support configurable TTL-based automatic cleanup for completed, failed, and cancelled responses
- **FR-015**: Background processing MUST support the full agentic loop (multi-turn tool calls, code execution), not just single-turn inference
- **FR-016**: Background request lifecycle events MUST be auditable (queued, started, completed, failed, cancelled)

### Key Entities

- **Background Request**: A response creation request with `background: true`. Persisted to storage immediately with status `queued`. Contains all fields of a normal response creation request.
- **Response Status**: The processing state of a response. Values: `queued` (accepted, waiting for a worker), `in_progress` (a worker is processing it), `completed` (processing finished successfully), `failed` (processing encountered an error or timed out, terminal), `cancelled` (explicitly cancelled by the client, terminal), `incomplete` (existing status for synchronous responses). All statuses except `queued` and `in_progress` are terminal. There is no automatic retry; clients must submit a new request.
- **Worker**: A process (or goroutine in integrated mode) that polls for and processes queued background requests. Each worker claims requests atomically to prevent duplicate processing.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Background requests return an acknowledgment to the client within 1 second of submission, regardless of the complexity of the underlying inference task
- **SC-002**: Completed background responses contain identical output to what the same request would produce synchronously (functional equivalence)
- **SC-003**: When multiple workers are available, no background request is processed more than once (zero duplicate processing)
- **SC-004**: On graceful shutdown, in-flight background requests either complete successfully or are marked as `failed` with a reason (zero silently orphaned requests)
- **SC-005**: Gateway and worker components can be scaled independently, allowing operators to match resource allocation to workload characteristics
- **SC-006**: Stale requests from crashed workers are detected and marked as failed within the configured staleness timeout
- **SC-007**: Background response TTL cleanup prevents unbounded storage growth from accumulated completed requests

## Assumptions

- A persistent storage backend is available and used as the shared queue between gateway and worker components. In-memory storage is sufficient for integrated mode development/testing.
- The existing response storage interface can be extended with the additional query capabilities needed for background processing (claim, status filtering, staleness detection) without breaking existing implementations.
- The existing agentic loop engine (spec 004) can be invoked by the worker without modification, since it already operates on request/response types independent of the HTTP transport layer.
- Audit logging (spec 042) integration follows the same pattern as existing features: audit events emitted at lifecycle transition points.
- The `--mode` flag is a server startup configuration, not a runtime toggle. A process runs in one mode for its entire lifetime.
- Default configuration values: worker poll interval of 5 seconds, drain timeout of 30 seconds, staleness timeout of 10 minutes, background response TTL of 24 hours.
- The gateway+worker deployment pattern constitutes a new deployment pattern per the constitution's documentation standards. A quickstart demonstrating this pattern is a deliverable of this spec.

## Dependencies

- **Spec 005 (Storage)**: Background responses must be stored and queryable by status. Storage adapter needs claim/filtering extensions.
- **Spec 004 (Agentic Loop)**: Workers invoke the agentic loop engine for full multi-turn processing.
- **Spec 042 (Audit Logging)**: Background lifecycle events are auditable.

## Out of Scope

- Webhook callbacks on background request completion
- SSE streaming to background responses (connect-later pattern)
- Priority queues or request prioritization
- External job queue systems (Redis, NATS, RabbitMQ)
- Persistent queue that survives storage backend unavailability
- Background request metrics and dashboards (operators use list endpoint for now)
