# Feature Specification: Backend Resilience (Circuit Breaker + Retry)

**Feature Branch**: `047-backend-resilience`
**Created**: 2026-03-29
**Status**: Draft
**Input**: Brainstorm 41 (Backend Resilience), resolved decisions 2026-03-29

## User Scenarios & Testing

### User Story 1 - Transient Backend Failures Handled Transparently (Priority: P1)

An operator deploys Antwort in front of a vLLM backend. The backend occasionally returns 502/503 errors during rolling restarts or temporary GPU memory pressure. Today, every such error propagates directly to the client, forcing client-side retry logic. With resilience enabled, Antwort automatically retries transient failures with backoff, and the client receives a successful response without knowing a retry occurred.

**Why this priority**: This is the most common production failure mode. Transient errors from load balancers and backend restarts happen regularly in Kubernetes deployments. Transparent retry eliminates the need for every client to implement its own retry logic.

**Independent Test**: Can be fully tested by configuring resilience, sending a request while the backend returns a 503 on the first attempt, and verifying the client receives a successful response after automatic retry.

**Acceptance Scenarios**:

1. **Given** resilience is enabled with max 3 retry attempts, **When** the backend returns HTTP 503 on the first attempt but succeeds on the second, **Then** the client receives a successful response with no indication a retry occurred.
2. **Given** resilience is enabled, **When** the backend returns HTTP 400 (bad request), **Then** the error is returned to the client immediately without retry.
3. **Given** resilience is enabled with max 3 attempts, **When** the backend fails on all 3 attempts, **Then** the client receives the final error after all retries are exhausted.
4. **Given** resilience is enabled, **When** a non-streaming request encounters a connection refused error, **Then** the request is retried with exponential backoff and jitter.

---

### User Story 2 - Circuit Breaker Prevents Cascading Failures (Priority: P1)

An operator runs multiple Antwort replicas behind a load balancer. The backend goes completely offline (crashed, network partition). Without a circuit breaker, all incoming requests pile up waiting for connection timeouts, exhausting Antwort's capacity. With the circuit breaker, after a configurable number of consecutive failures, Antwort starts failing requests immediately with a clear error message, preserving gateway resources and giving operators a clear signal that the backend is down.

**Why this priority**: Cascading failures can take down the entire gateway. This is a production safety concern, not just a convenience feature.

**Independent Test**: Can be tested by configuring a circuit breaker threshold, sending requests while the backend is unreachable, and verifying that after the threshold is crossed, subsequent requests fail immediately without contacting the backend.

**Acceptance Scenarios**:

1. **Given** circuit breaker threshold is 5 consecutive failures, **When** the 5th consecutive request fails, **Then** the circuit opens and subsequent requests fail immediately with a descriptive error.
2. **Given** the circuit is open, **When** the reset timeout elapses, **Then** the circuit transitions to half-open and allows one probe request through to test recovery.
3. **Given** the circuit is half-open, **When** the probe request succeeds, **Then** the circuit closes and normal traffic resumes.
4. **Given** the circuit is half-open, **When** the probe request fails, **Then** the circuit reopens for another reset timeout period.

---

### User Story 3 - Rate-Limited Requests Respect Backend Signals (Priority: P2)

An operator uses Antwort with a rate-limited backend (commercial API or shared vLLM instance). When the backend returns HTTP 429 with a `Retry-After` header, Antwort waits the indicated duration and retries the request. The 429 response does not count as a backend failure for circuit breaker purposes, because the backend is healthy, just busy.

**Why this priority**: Rate limiting is distinct from failures. Treating 429 as a failure would trip the circuit breaker on a healthy backend, making the situation worse. Respecting `Retry-After` is the correct protocol behavior.

**Independent Test**: Can be tested by sending a request that triggers a 429 with a `Retry-After` header, verifying the retry waits the indicated duration, and confirming the circuit breaker failure count remains unchanged.

**Acceptance Scenarios**:

1. **Given** resilience is enabled, **When** the backend returns 429 with `Retry-After: 2`, **Then** Antwort waits approximately 2 seconds before retrying.
2. **Given** resilience is enabled, **When** the backend returns 429 with an HTTP-date `Retry-After` header, **Then** Antwort waits until that timestamp before retrying.
3. **Given** resilience is enabled, **When** the backend returns 429, **Then** the circuit breaker consecutive failure count is not incremented.
4. **Given** resilience is enabled, **When** the backend returns 429 without a `Retry-After` header, **Then** Antwort retries using standard exponential backoff.

---

### User Story 4 - Streaming Requests Retry Connection Failures (Priority: P2)

A client sends a streaming request. The initial connection to the backend fails (connection refused, TCP timeout). With resilience enabled, Antwort retries establishing the connection. However, once the streaming response begins (first SSE event received), any subsequent failure terminates the stream. The client can retry the entire request using `previous_response_id` to preserve conversation context.

**Why this priority**: Streaming connection failures are common during backend restarts. Retrying the connection phase is safe and valuable. Mid-stream recovery is explicitly out of scope due to the complexity of resuming a partial LLM response.

**Independent Test**: Can be tested by causing a connection failure on the first streaming attempt, verifying the connection is retried, and then verifying that a mid-stream disconnection terminates the response without retry.

**Acceptance Scenarios**:

1. **Given** resilience is enabled for a streaming request, **When** the initial connection fails, **Then** the connection is retried with backoff.
2. **Given** a streaming response is in progress (events have been sent), **When** the connection drops, **Then** the response fails with an error event and no retry is attempted.

---

### User Story 5 - Operator Visibility into Resilience Behavior (Priority: P3)

An operator monitors Antwort via Prometheus dashboards. They can see the current circuit breaker state (closed, open, half-open), the number of state transitions, retry counts, and retry exhaustion events. Debug logging (when enabled) shows individual retry attempts with timing and error details.

**Why this priority**: Observability is essential for operators to understand resilience behavior and tune configuration. Without visibility, operators cannot distinguish between backend issues and gateway issues.

**Independent Test**: Can be tested by triggering retries and circuit breaker state changes, then verifying the corresponding metrics are exposed on the Prometheus endpoint and debug log entries appear.

**Acceptance Scenarios**:

1. **Given** resilience is enabled with debug logging, **When** a retry occurs, **Then** a debug log entry records the attempt number, wait duration, and error that triggered the retry.
2. **Given** resilience is enabled, **When** the circuit breaker changes state, **Then** a metric records the transition and the current state is queryable.
3. **Given** resilience is enabled, **When** all retry attempts are exhausted for a request, **Then** a retry exhaustion metric is incremented.

---

### Edge Cases

- What happens when the request context is cancelled during a retry wait? The retry is abandoned immediately and the cancellation propagates to the client.
- What happens when the circuit breaker is open and a background worker (spec 044) attempts a provider call? The worker's call fails fast with a circuit-open error, and the background response is marked as failed.
- What happens when resilience is not configured? Antwort behaves exactly as it does today: errors propagate directly, no retries, no circuit breaking. Zero behavioral change.
- What happens when the `Retry-After` header specifies a duration longer than the remaining request context timeout? The retry is skipped (waiting would exceed the client's deadline), and the 429 error is returned.
- What happens during the agentic loop when a mid-loop provider call fails? The provider call is retried per resilience config. If all retries fail and the circuit trips, the engine receives a single error and the response fails. Earlier turns of work are not discarded.

## Requirements

### Functional Requirements

- **FR-001**: System MUST support a circuit breaker with three states: closed (normal operation), open (fast-fail), and half-open (recovery probe).
- **FR-002**: Circuit breaker MUST transition from closed to open after a configurable number of consecutive failures.
- **FR-003**: Circuit breaker MUST transition from open to half-open after a configurable reset timeout elapses.
- **FR-004**: Circuit breaker MUST transition from half-open to closed after a successful probe request, or back to open after a failed probe.
- **FR-005**: When the circuit is open, requests MUST fail immediately with a descriptive error indicating the backend is unavailable, without contacting the backend.
- **FR-006**: System MUST support configurable retry with exponential backoff and jitter for non-streaming requests.
- **FR-007**: System MUST support retry of the connection phase (pre-first-event) for streaming requests.
- **FR-008**: System MUST NOT retry streaming requests after the first SSE event has been sent to the client.
- **FR-009**: System MUST classify errors as retryable or non-retryable. Retryable: HTTP 502, 503, 504, connection refused, connection reset, context deadline exceeded. Non-retryable: HTTP 4xx (except 429).
- **FR-010**: System MUST parse the `Retry-After` response header (both seconds and HTTP-date formats) from 429 responses and wait the indicated duration before retrying.
- **FR-011**: HTTP 429 responses MUST NOT count toward the circuit breaker's consecutive failure threshold.
- **FR-012**: When `Retry-After` specifies a wait duration that exceeds the remaining request context timeout, the system MUST skip the retry and return the 429 to the client.
- **FR-013**: Retry attempts MUST each count as separate attempts for circuit breaker failure tracking (retries happen before the circuit breaker evaluation).
- **FR-014**: Resilience MUST be opt-in. When no resilience configuration is present, the system MUST behave identically to its current behavior (direct error propagation).
- **FR-015**: Resilience configuration MUST be a single global block, not per-provider.
- **FR-016**: System MUST expose Prometheus metrics for circuit breaker state, state transitions, consecutive failure count, retry attempts, and retry exhaustion events.
- **FR-017**: System MUST emit debug log entries (via existing debug logging categories) for retry attempts including attempt number, wait duration, and triggering error.
- **FR-018**: System MUST emit a log entry when the circuit breaker changes state, including the previous state, new state, and reason.
- **FR-019**: Circuit breaker state MUST be per-instance (not shared across replicas). Each Antwort instance maintains its own circuit breaker.
- **FR-020**: The resilience layer MUST be transparent to the engine. The agentic loop and response processing logic MUST NOT be aware of retries or circuit breaker state.

### Key Entities

- **Circuit Breaker**: Tracks the health state of the backend provider. Has a state (closed/open/half-open), consecutive failure count, and last failure timestamp.
- **Retry Policy**: Defines retry behavior including maximum attempts, backoff base duration, maximum backoff duration, and jitter strategy.
- **Error Classification**: Categorizes provider errors as retryable (transient) or non-retryable (permanent), with special handling for rate-limited (429) responses.
- **Resilience Configuration**: Global configuration block enabling and tuning circuit breaker and retry behavior.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Transient backend errors (502/503/504, connection failures) are resolved transparently via retry in at least 90% of cases where the backend recovers within the retry window.
- **SC-002**: When the backend is offline, requests fail within 100ms (fast-fail via open circuit) instead of waiting for connection timeouts.
- **SC-003**: Rate-limited requests (429 with Retry-After) are retried after the indicated wait without triggering circuit breaker state changes.
- **SC-004**: Operators can observe circuit breaker state and retry activity through Prometheus metrics without additional tooling.
- **SC-005**: Antwort with resilience disabled behaves identically to the current behavior, with no measurable performance overhead.
- **SC-006**: Configuration requires fewer than 10 lines of YAML to enable resilience with sensible defaults.

## Assumptions

- The backend exposes standard HTTP status codes (4xx/5xx) that reliably indicate error categories.
- Connection-level errors (refused, reset, timeout) are distinguishable from application-level errors at the Go HTTP client layer.
- Exponential backoff with jitter is sufficient for retry spacing; no adaptive or token-bucket rate limiting is needed at this phase.
- Per-instance circuit breaker state provides adequate convergence across replicas since all replicas target the same backend.
- The existing Prometheus metrics infrastructure (spec 046) and debug logging infrastructure (spec 026) are in place and can be extended.

## Out of Scope

- Mid-stream reconnection for streaming responses (too complex, clients use `previous_response_id` to retry)
- Active health probing (deferred to a future phase if operators need faster recovery detection)
- Multi-backend failover and load balancing (separate architectural concern)
- Shared circuit breaker state across replicas (per-instance is sufficient)
- Outbound rate limiting (belongs in infrastructure layer, not the gateway)
- Per-provider resilience configuration (deferred until multi-provider support lands)
