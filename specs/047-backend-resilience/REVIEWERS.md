# Review Guide: Backend Resilience (Circuit Breaker + Retry)

**Spec:** [spec.md](spec.md) | **Plan:** [plan.md](plan.md) | **Tasks:** [tasks.md](tasks.md)
**Generated:** 2026-03-29

---

## What This Spec Does

Adds a resilience layer between Antwort's engine and its LLM backend provider. When the backend has transient failures (502/503, connection drops, GPU OOM), Antwort retries automatically instead of propagating every error to clients. When the backend goes fully offline, a circuit breaker fails requests immediately rather than letting them pile up waiting for timeouts.

**In scope:** Circuit breaker (closed/open/half-open state machine), retry with exponential backoff and jitter, 429 Retry-After header parsing, streaming connection retry, Prometheus metrics, debug logging.

**Out of scope:** Mid-stream reconnection (too complex for the value), active health probing (deferred to Phase 2 if operators request it), multi-backend failover (separate concern), shared circuit breaker state across replicas, per-provider config (deferred until multi-provider support).

## Bigger Picture

This is the first resilience feature in Antwort. The project constitution (Principle I: Enterprise Production First) explicitly lists resilience as a required production concern. Until now, Antwort has been a transparent proxy for errors, pushing all retry logic to clients. The design was informed by [brainstorm 41](../../brainstorm/41-backend-resilience.md), which analyzed SMG's (Shepherd Model Gateway) production-grade approach and adapted it to Antwort's simpler single-backend model.

This spec is foundational for future resilience work. Phase 2 (active health probing) and Phase 3 (multi-backend failover) would build on the circuit breaker and error classification infrastructure established here. The per-instance circuit breaker design also aligns with Kubernetes HPA scaling, where each replica independently tracks backend health.

---

## Spec Review Guide (30 minutes)

> This guide helps you focus your 30 minutes on the parts of the spec and plan
> that need human judgment most. Each section points to specific locations and
> frames the review as questions.

### Understanding the approach (8 min)

Read [User Stories 1 and 2](spec.md#user-scenarios--testing) for the core retry and circuit breaker behavior. Then skim the [Design Details](plan.md#design-details) section for the Complete() and Stream() flow diagrams. As you read, consider:

- Does the decorator pattern (wrapping `provider.Provider`) feel right, or would you prefer the resilience logic closer to the HTTP client in `openaicompat`?
- Is the decision to make resilience opt-in correct? Should a production gateway have resilience on by default, even with conservative settings?
- The spec assumes per-instance circuit breaker state is sufficient because replicas converge quickly. Does this hold for your deployment topology (e.g., many replicas behind a load balancer)?

### Key decisions that need your eyes (12 min)

**Retry before circuit breaker** ([research.md, Error Classification](research.md#error-classification))

Each retry attempt counts as a separate attempt for circuit breaker tracking. This means a request with 3 retries that all fail contributes 3 failures toward the threshold, not 1. This matches SMG's approach but means the circuit could trip faster under load.
- Does this make the circuit breaker too sensitive? Would you prefer retries to count as a single logical request for circuit breaker purposes?

**429 excluded from circuit breaker** ([FR-011](spec.md#functional-requirements))

Rate-limited responses (429) never increment the circuit breaker failure count. The rationale is that rate limiting means "healthy but busy," not "broken." However, sustained 429s could indicate a capacity problem that operators want visibility into.
- Is excluding 429 from circuit breaker entirely correct, or should sustained 429s (e.g., 50 consecutive) eventually trigger some protection?

**Streaming retry boundary** ([FR-007, FR-008](spec.md#functional-requirements))

Only the connection phase of streaming requests is retried. Once the first SSE event is received, any failure terminates the stream. The client retries using `previous_response_id`.
- Is this boundary clear enough for operators to understand? Would a metric showing "streams that failed mid-flight" help operators distinguish connection issues from mid-stream drops?

**Modifying APIError** ([research.md, Retry-After Header Access](research.md#retry-after-header-access))

The plan adds a `RetryAfter time.Duration` field to `api.APIError`. This touches a shared type used across many packages.
- Is adding a field to a shared type acceptable, or would a wrapper error type in the resilience package be cleaner?

### Areas where I'm less certain (5 min)

- [Half-open concurrency](spec.md#user-story-2---circuit-breaker-prevents-cascading-failures-priority-p1): The spec says "allows one probe request" in half-open state, but doesn't explicitly state what happens to other concurrent requests during half-open. The plan uses CAS to give one goroutine the probe slot, implying others get fast-fail. This should probably be explicit in the spec.

- [Retry-After cap](data-model.md#retrypolicy): The plan introduces a `retry_after_max` (default 30s) to cap `Retry-After` values. This isn't in the spec's functional requirements. Is this an implementation detail that belongs only in the plan, or should it be a spec requirement?

- [Jitter algorithm](plan.md#design-details): The plan uses random jitter in `[0, backoffBase)` but doesn't specify the algorithm. Full jitter, equal jitter, and decorrelated jitter have different properties under load. For a single-backend gateway this likely doesn't matter, but it's worth confirming.

### Risks and open questions (5 min)

- If the circuit breaker trips during an agentic loop (turn 3 of 10), the response fails and earlier turns of work are lost. Is this acceptable, or should the engine return a partial response with the work completed so far? (See [edge case 5](spec.md#edge-cases))

- The resilience layer wraps `ListModels()` without retry or circuit breaker protection. If `ListModels()` fails, does this create an inconsistent experience where model listing fails but inference requests retry? Should `ListModels()` also respect the circuit breaker?

- Configuration defaults (5 failures, 30s reset, 3 attempts) were chosen from SMG defaults and industry patterns. Are these appropriate for LLM workloads where individual requests can take 30-120 seconds? A 100ms backoff seems short relative to typical LLM inference times.

---

## Code Review Guide (30 minutes)

> This section guides a code reviewer through the implementation changes,
> focusing on high-level questions that need human judgment.

**Changed files:** 7 source files, 4 test files, 3 doc files, 1 server wiring change

### Understanding the changes (8 min)

Start with `pkg/provider/resilience/resilience.go`: this is the core `ResilientProvider` that wraps any `provider.Provider` with `Complete()` and `Stream()` retry loops.
Then read `pkg/provider/resilience/circuit.go` for the state machine, and `classify.go` for error classification.
Finally, glance at `cmd/server/main.go:92` to see the one-line wiring.

- Does the decorator pattern (wrapping `provider.Provider`) feel natural here, or would middleware at the HTTP client level in `openaicompat` be cleaner?
- Is the retry loop structure in `Complete()` and `Stream()` clear enough, or does the shared `handleError()` method obscure the flow?

### Key decisions that need your eyes (12 min)

**Lock-free circuit breaker** (`pkg/provider/resilience/circuit.go:39-46`, relates to [FR-001](spec.md#functional-requirements))

The circuit breaker uses `sync/atomic` with `CompareAndSwap` for all state transitions instead of a mutex. This enables zero-allocation hot paths in the closed state.
- Question: Is the CAS-based half-open probe slot (only one goroutine wins the probe) correct under high concurrency? Could a lost CAS race cause a request to fast-fail when it should have been allowed?

**Retry-After cap** (`pkg/provider/resilience/resilience.go:201-211`, relates to [FR-010](spec.md#functional-requirements))

The implementation caps `Retry-After` at `retry_after_max` (default 30s). This prevents a misbehaving backend from stalling requests indefinitely.
- Question: Is 30s the right default cap? For LLM workloads where inference can take 60-120s, should this be higher?

**apiErrAs direct type assertion** (`pkg/provider/resilience/resilience.go:214-224`)

The `apiErrAs` helper uses direct `(*api.APIError)` type assertion rather than `errors.As()`. This works because providers always return `*api.APIError` directly (never wrapped).
- Question: If a future middleware wraps provider errors with `fmt.Errorf("...%w", err)`, this would silently break. Should we use `errors.As()` defensively?

**Metrics transitions counter unused** (`pkg/observability/metrics.go`, `ResilienceCircuitBreakerTransitionsTotal`)

The `transitions_total` counter is registered but never incremented. The `recordCircuitTransition()` method only updates the state gauge.
- Question: Should we record transitions with from/to labels, or is the state gauge sufficient for operators?

### Areas where I'm less certain (5 min)

- `pkg/provider/resilience/resilience.go:169`: `RecordFailure()` is called inside the retry loop, so each failed attempt increments the circuit breaker's failure count independently. Under high concurrency with 3 retries, a single request contributes 3 failures toward the threshold. This is by design ([FR-013](spec.md#functional-requirements)), but could make the circuit breaker more sensitive than operators expect.

- `pkg/provider/openaicompat/errors.go:60-77`: The `parseRetryAfter` function handles both seconds-integer and HTTP-date formats. The HTTP-date parsing uses `http.ParseTime` which supports RFC1123, RFC850, and ANSI C formats. I'm not 100% sure all LLM backends format the header consistently.

### Deviations and risks (5 min)

No deviations from [plan.md](plan.md) were identified. The implementation follows the planned architecture exactly: decorator pattern, global config, per-instance circuit breaker, provider-level retry.

- The `ResilienceCircuitBreakerTransitionsTotal` counter was initially declared but not incremented. This was fixed during deep review (round 1). Transitions are now tracked with from/to labels.

---

## Deep Review Report

> Automated multi-perspective code review results. This section summarizes
> what was checked, what was found, and what remains for human review.

**Date:** 2026-03-29 | **Rounds:** 1/3 | **Gate:** PASS

### Review Agents

| Agent | Findings | Status |
|-------|----------|--------|
| Correctness | 3 | completed |
| Architecture & Idioms | 4 | completed |
| Security | 0 | completed |
| Production Readiness | 0 | completed |
| Test Quality | 0 (advisory) | completed |
| CodeRabbit (external) | 2 (code) | completed |

### Findings Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 2 | 2 | 0 |
| Minor | 3 | 3 | 0 |

### What was fixed automatically

Fixed error unwrapping in `retryAfterWait` (replaced custom `apiErrAs` with standard `errors.As` for correct wrapped error handling). Implemented circuit breaker transition counting with from/to labels and debug logging on state changes (the counter metric was declared but never incremented). Removed unused `CircuitOpenError` dead code. Added defensive guard against panic on zero backoff base. Corrected a misleading "should not be reached" comment.

### What still needs human attention

All Critical and Important findings were resolved. No remaining findings. The test quality agent provided advisory suggestions for additional edge case tests (concurrent half-open probes, backoff overflow at extreme attempts, MaxAttempts=1) that are not blocking but would strengthen coverage.

### Recommendation

All findings addressed. Code is ready for human review with no known blockers.

---
*Full context in linked [spec](spec.md) and [plan](plan.md).*
