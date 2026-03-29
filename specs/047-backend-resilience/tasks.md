# Tasks: Backend Resilience (Circuit Breaker + Retry)

**Input**: Design documents from `/specs/047-backend-resilience/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Package structure and shared types

- [x] T001 Create `pkg/provider/resilience/` package directory structure
- [x] T002 [P] Add `ResilienceConfig` struct with defaults and YAML tags to `pkg/config/config.go`
- [x] T003 [P] Add `RetryAfter` field (`time.Duration`) to `APIError` in `pkg/api/errors.go`
- [x] T004 Add `Resilience ResilienceConfig` field to top-level `Config` struct in `pkg/config/config.go`
- [x] T005 Add resilience config validation to `pkg/config/validate.go` (failure_threshold > 0, max_attempts >= 1, durations > 0 when enabled)
- [x] T006 Add resilience config validation tests to `pkg/config/config_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Error classification and circuit breaker core that all user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T007 Implement error classifier in `pkg/provider/resilience/classify.go`: `Classify(error) Classification` returning Retryable, NonRetryable, or RateLimited; detect `*api.APIError` types, network errors (`net.Error`, `syscall.ECONNREFUSED`, `syscall.ECONNRESET`), and `context.DeadlineExceeded`
- [x] T008 [P] Implement `CircuitBreaker` state machine in `pkg/provider/resilience/circuit.go`: `New(threshold int64, resetTimeout time.Duration)`, `Allow() bool`, `RecordSuccess()`, `RecordFailure()`, `State() int32` using `sync/atomic` for lock-free concurrency
- [x] T009 [P] Unit tests for error classifier in `pkg/provider/resilience/classify_test.go`: test all error types (APIError with each ErrorType, network errors, context errors, nil, unknown errors)
- [x] T010 Unit tests for circuit breaker in `pkg/provider/resilience/circuit_test.go`: test state transitions (closed->open after threshold, open->half-open after timeout, half-open->closed on success, half-open->open on failure), concurrent access safety, consecutive failure reset on success

**Checkpoint**: Error classification and circuit breaker core ready. User story implementation can begin.

---

## Phase 3: User Story 1 - Transient Backend Failures Handled Transparently (Priority: P1)

**Goal**: Retry non-streaming requests on transient errors with exponential backoff and jitter, transparent to clients.

**Independent Test**: Configure resilience, mock backend to fail once then succeed, verify client receives successful response.

### Implementation for User Story 1

- [x] T011 [US1] Implement retry logic in `pkg/provider/resilience/retry.go`: `computeBackoff(attempt int, base, max time.Duration) time.Duration` with exponential backoff and jitter using `math/rand`; `sleepWithContext(ctx context.Context, d time.Duration) error` that respects context cancellation
- [x] T012 [US1] Implement `ResilientProvider` in `pkg/provider/resilience/resilience.go`: struct wrapping `provider.Provider` with `CircuitBreaker` and retry config; implement `Complete()` with retry loop integrating circuit breaker and error classification per plan flow diagram
- [x] T013 [US1] Implement `Wrap(inner provider.Provider, cfg config.ResilienceConfig) provider.Provider` constructor in `pkg/provider/resilience/resilience.go` that returns `inner` unchanged when `cfg.Enabled` is false
- [x] T014 [US1] Unit tests for retry logic in `pkg/provider/resilience/retry_test.go`: test backoff calculation (exponential growth, max cap, jitter bounds), context cancellation during sleep
- [x] T015 [US1] Unit tests for `ResilientProvider.Complete()` in `pkg/provider/resilience/resilience_test.go`: test successful passthrough, retry on retryable error, no retry on non-retryable error, all retries exhausted, circuit breaker integration (open state fast-fail)
- [x] T016 [US1] Wire resilience wrapper in `cmd/server/main.go`: after `createProvider()`, wrap with `resilience.Wrap(prov, cfg.Resilience)` before passing to `engine.New()`
- [x] T017 [US1] Add environment variable overrides for resilience config in `pkg/config/loader.go`: `ANTWORT_RESILIENCE_ENABLED`, `ANTWORT_RESILIENCE_FAILURE_THRESHOLD`, `ANTWORT_RESILIENCE_MAX_ATTEMPTS`

**Checkpoint**: Non-streaming retry with circuit breaker works. Transient 502/503/504 and connection errors are retried transparently.

---

## Phase 4: User Story 2 - Circuit Breaker Prevents Cascading Failures (Priority: P1)

**Goal**: When backend is down, fail fast after threshold instead of waiting for timeouts.

**Independent Test**: Send requests while backend is unreachable, verify fast-fail after threshold, verify recovery via half-open probe.

### Implementation for User Story 2

- [x] T018 [US2] Define `CircuitOpenError` sentinel in `pkg/provider/resilience/circuit.go` implementing `error` interface with descriptive message including provider name and reset timeout
- [x] T019 [US2] Map `CircuitOpenError` to `api.APIError` with `ErrorTypeServerError` and clear message in `pkg/provider/resilience/resilience.go` Complete/Stream methods
- [ ] T020 [US2] Integration test in `test/integration/resilience_test.go`: start antwort with resilience config and mock backend; test circuit breaker lifecycle: send requests against unreachable backend, verify fast-fail after threshold, verify half-open probe after timeout, verify recovery on probe success
- [ ] T021 [US2] Integration test for fast-fail timing in `test/integration/resilience_test.go`: verify circuit-open responses return in <100ms (SC-002)

**Checkpoint**: Circuit breaker prevents cascading failures. Fast-fail verified under 100ms.

---

## Phase 5: User Story 3 - Rate-Limited Requests Respect Backend Signals (Priority: P2)

**Goal**: Parse `Retry-After` headers from 429 responses, wait indicated duration, retry without tripping circuit breaker.

**Independent Test**: Mock backend returns 429 with Retry-After header, verify retry waits correct duration, circuit breaker unaffected.

### Implementation for User Story 3

- [x] T022 [US3] Parse `Retry-After` header in `pkg/provider/openaicompat/errors.go`: enhance `MapHTTPError()` to extract `Retry-After` header (seconds format and HTTP-date format via `http.ParseTime`) and populate `APIError.RetryAfter` field
- [x] T023 [US3] Handle `RateLimited` classification in `ResilientProvider.Complete()` and `Stream()` in `pkg/provider/resilience/resilience.go`: use `RetryAfter` duration if available (fallback to standard backoff), skip if duration exceeds remaining context deadline (FR-012), do not call `cb.RecordFailure()` (FR-011)
- [x] T024 [US3] Unit tests for Retry-After parsing in `pkg/provider/openaicompat/errors_test.go`: test seconds format, HTTP-date format, missing header, invalid header
- [x] T025 [US3] Unit tests for rate-limited retry behavior in `pkg/provider/resilience/resilience_test.go`: test 429 retry with RetryAfter, 429 without RetryAfter (fallback to backoff), 429 with RetryAfter exceeding context deadline (skip retry), circuit breaker failure count unchanged after 429

**Checkpoint**: 429 responses handled correctly. Retry-After respected, circuit breaker unaffected.

---

## Phase 6: User Story 4 - Streaming Requests Retry Connection Failures (Priority: P2)

**Goal**: Retry streaming connection failures (pre-first-event). No retry once events start flowing.

**Independent Test**: Mock backend refuses streaming connection once then accepts, verify client receives stream. Verify mid-stream drop is not retried.

### Implementation for User Story 4

- [x] T026 [US4] Implement `Stream()` method on `ResilientProvider` in `pkg/provider/resilience/resilience.go`: retry loop for connection phase (when `inner.Stream()` returns error), pass through channel unchanged on success (no mid-stream retry per FR-008)
- [x] T027 [US4] Unit tests for `ResilientProvider.Stream()` in `pkg/provider/resilience/resilience_test.go`: test connection retry on error, successful passthrough, circuit breaker integration for streaming, no retry when channel returned successfully

**Checkpoint**: Streaming connection failures retried transparently. Mid-stream drops propagate to client.

---

## Phase 7: User Story 5 - Operator Visibility into Resilience Behavior (Priority: P3)

**Goal**: Expose circuit breaker state and retry activity via Prometheus metrics and debug logging.

**Independent Test**: Trigger retries and circuit state changes, verify metrics on `/metrics` endpoint and debug log entries.

### Implementation for User Story 5

- [x] T028 [P] [US5] Register resilience Prometheus metrics in `pkg/observability/metrics.go`: `antwort_resilience_circuit_breaker_state` (GaugeVec, label: provider), `antwort_resilience_circuit_breaker_transitions_total` (CounterVec, labels: provider, from, to), `antwort_resilience_retry_attempts_total` (CounterVec, labels: provider, outcome), `antwort_resilience_retry_exhausted_total` (CounterVec, label: provider)
- [x] T029 [US5] Add metrics recording to `ResilientProvider` in `pkg/provider/resilience/resilience.go`: record circuit breaker state gauge on each request, record transitions in `CircuitBreaker` state change methods, record retry attempt outcome (success/failure/rate_limited), record retry exhaustion
- [x] T030 [US5] Add debug logging to `ResilientProvider` in `pkg/provider/resilience/resilience.go`: log retry attempts with attempt number, wait duration, and triggering error using `debug.Log("providers", ...)` (FR-017); log circuit breaker state changes with previous state, new state, and reason (FR-018)
- [ ] T031 [US5] Integration test for metrics in `test/integration/resilience_test.go`: trigger retries and circuit state changes, scrape `/metrics` endpoint, verify all 4 metric families present with expected label values

**Checkpoint**: Full observability for resilience layer via metrics and debug logging.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, E2E tests, and final validation

- [x] T032 [P] Add resilience configuration block to `docs/modules/reference/pages/config-reference.adoc` with all fields, defaults, and YAML examples
- [x] T033 [P] Add `ANTWORT_RESILIENCE_*` environment variables to `docs/modules/reference/pages/environment-variables.adoc`
- [x] T034 [P] Create resilience tutorial at `docs/modules/tutorial/pages/resilience.adoc` covering: enabling resilience, tuning thresholds, monitoring circuit breaker state, interpreting retry metrics
- [ ] T035 E2E test in `test/e2e/resilience_test.go`: test with compiled server binary and mock-backend; verify retry on transient error, circuit breaker fast-fail, and 429 Retry-After handling (build tag `e2e`)
- [x] T036 Verify `Wrap()` returns unwrapped provider when `Enabled: false` and confirm zero overhead (no wrapper in the call path) with a unit test in `pkg/provider/resilience/resilience_test.go`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on Setup (T001-T006)
- **User Stories (Phase 3-7)**: All depend on Foundational (T007-T010)
  - US1 (P1): First priority, enables all subsequent stories
  - US2 (P1): Can proceed in parallel with US1 (separate files)
  - US3 (P2): Depends on US1 (extends Complete/Stream retry logic)
  - US4 (P2): Depends on US1 (extends Stream method)
  - US5 (P3): Depends on US1+US2 (adds metrics/logging to existing code)
- **Polish (Phase 8)**: Depends on all user stories complete

### User Story Dependencies

- **US1** (Retry): Can start after Foundational. Core retry logic.
- **US2** (Circuit Breaker): Can start after Foundational. Adds circuit-open error type.
- **US3** (Rate Limiting): Depends on US1 (extends retry handling with 429 logic).
- **US4** (Streaming): Depends on US1 (extends ResilientProvider with Stream method).
- **US5** (Observability): Depends on US1+US2 (adds metrics to existing code paths).

### Within Each User Story

- Implementation before tests where tests depend on implementation behavior
- Core logic before integration with other subsystems

### Parallel Opportunities

- T002 and T003 can run in parallel (different files: config.go, errors.go)
- T007, T008 can run in parallel (different files: classify.go, circuit.go)
- T009, T010 can run in parallel (test files for above)
- US1 and US2 can overlap (US2 adds circuit error type while US1 builds retry loop)
- T028 can run in parallel with other US5 tasks (metrics.go is separate file)
- T032, T033, T034 can all run in parallel (different doc files)

---

## Parallel Example: Foundational Phase

```bash
# Launch foundational tasks in parallel:
Task: "T007 - Implement error classifier in pkg/provider/resilience/classify.go"
Task: "T008 - Implement CircuitBreaker state machine in pkg/provider/resilience/circuit.go"

# Then launch tests in parallel:
Task: "T009 - Unit tests for error classifier"
Task: "T010 - Unit tests for circuit breaker"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T006)
2. Complete Phase 2: Foundational (T007-T010)
3. Complete Phase 3: User Story 1 (T011-T017)
4. **STOP and VALIDATE**: Test retry behavior independently
5. Non-streaming requests with transient errors are now retried transparently

### Incremental Delivery

1. Setup + Foundational -> Core infrastructure ready
2. Add US1 (Retry) -> Test independently -> Transient error handling works (MVP!)
3. Add US2 (Circuit Breaker) -> Test independently -> Cascading failure protection
4. Add US3 (Rate Limiting) -> Test independently -> 429 Retry-After support
5. Add US4 (Streaming) -> Test independently -> Streaming connection retry
6. Add US5 (Observability) -> Test independently -> Full metrics and logging
7. Polish -> Documentation and E2E tests

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Constitution requires E2E tests (v1.8.0) in addition to unit and integration tests
- All resilience code in `pkg/provider/resilience/` uses Go stdlib only (constitution Principle II)
- Debug logging uses existing `providers` category, no new category needed
