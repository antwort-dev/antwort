# Brainstorm 41: Backend Resilience

**Date**: 2026-03-21
**Participants**: Roland Huss
**Inspiration**: SMG (Shepherd Model Gateway) reliability architecture
**Goal**: Evaluate resilience patterns for Antwort's backend provider layer, informed by SMG's production-grade approach to circuit breakers, retries, and health monitoring.

## Motivation

Antwort currently has no resilience features for backend communication. If the vLLM backend goes down, times out, or returns errors, Antwort propagates the failure directly to the client. In production deployments (especially multi-replica), this creates several problems:

1. **Cascading failures**: A slow backend causes request pile-up in the gateway
2. **No recovery detection**: Once a backend fails, there's no mechanism to detect when it recovers
3. **No retry for transient errors**: Network blips, 502/503 from load balancers, or temporary GPU OOM all fail immediately
4. **No backend health visibility**: Operators have no signal about backend health from Antwort's perspective

SMG addresses all of these with a layered resilience system. Not all of it applies to Antwort (SMG manages worker pools, Antwort talks to a single backend URL), but the patterns are valuable.

## SMG's Approach (Reference)

SMG implements three resilience layers per worker:

### 1. Circuit Breaker (per worker)
- Three states: **closed** (healthy), **open** (failing, reject fast), **half-open** (testing recovery)
- Transitions based on consecutive failure/success counts
- Metrics: `smg_worker_cb_state`, `smg_worker_cb_transitions_total`, `smg_worker_cb_consecutive_failures`
- When open: requests fail fast without touching the backend

### 2. Retry with Backoff
- Configurable retry count with exponential backoff
- Only retries on transient errors (connection refused, timeout, 502/503/504)
- Does NOT retry on 4xx or business logic errors
- Metrics: `smg_worker_retries_total`, `smg_worker_retries_exhausted_total`

### 3. Active + Passive Health Monitoring
- **Active**: Periodic health check requests to backend
- **Passive**: Track success/failure of real requests
- Unhealthy workers removed from rotation, re-added when health checks pass

## What Applies to Antwort

Antwort's model is simpler than SMG's (typically one backend URL per provider, not a pool of workers). But the patterns still apply:

### Circuit Breaker: YES, adapted

Even with a single backend, a circuit breaker prevents request pile-up during outages:

```
Closed (normal) -> Open (backend down, fail fast) -> Half-Open (test with one request)
```

Benefits:
- Fail fast instead of hanging on connection timeouts
- Reduce load on a struggling backend
- Clear signal to operators via metrics/logs
- Configurable thresholds (e.g., 5 consecutive failures to open, 1 success to close)

Implementation sketch:
```go
// pkg/provider/circuit.go
type CircuitBreaker struct {
    state           atomic.Int32 // 0=closed, 1=open, 2=half-open
    failures        atomic.Int64
    threshold       int64
    resetTimeout    time.Duration
    lastFailure     atomic.Int64 // unix timestamp
}

func (cb *CircuitBreaker) Allow() bool { ... }
func (cb *CircuitBreaker) RecordSuccess() { ... }
func (cb *CircuitBreaker) RecordFailure() { ... }
```

This wraps any `provider.Provider` transparently. No external dependencies needed (Go stdlib only).

### Retry with Backoff: YES, selective

Retry makes sense for:
- Connection refused / reset
- HTTP 502, 503, 504 (upstream errors)
- Context deadline exceeded (with shorter per-attempt timeout)

Retry does NOT make sense for:
- HTTP 400, 401, 422 (client errors)
- HTTP 429 (rate limited, needs different handling)
- Streaming responses (can't retry mid-stream)

Key design decision: **retry before or after circuit breaker?**
- Before: retries count as separate attempts for circuit breaker (SMG's approach)
- After: circuit breaker sees retries as one logical request
- Recommendation: before (matches SMG, gives circuit breaker accurate failure signal)

Configuration:
```yaml
providers:
  vllm:
    retry:
      max_attempts: 3          # default: 1 (no retry)
      backoff_base: 100ms      # exponential: 100ms, 200ms, 400ms
      backoff_max: 2s
      retryable_codes: [502, 503, 504]
```

### Health Monitoring: MAYBE, lightweight

Active health checks add complexity. For Antwort's typical single-backend setup:
- **Passive monitoring** (tracking real request outcomes) is sufficient
- Active health checks mainly useful for circuit breaker recovery detection
- Could piggyback on circuit breaker: when open, periodically send a lightweight request (e.g., `GET /health` or `GET /v1/models`) to detect recovery

### Rate Limiting (outbound): NO for now

SMG does per-tenant rate limiting because it manages multi-tenant access to shared GPU resources. Antwort's rate limiting needs are different:
- Inbound rate limiting belongs in the Kubernetes ingress or API gateway in front of Antwort
- Outbound rate limiting (protecting the backend) could be useful but adds complexity
- Defer unless there's a concrete use case

## Proposed Feature Scope

### Phase 1: Circuit Breaker + Basic Retry (spec-worthy)
- Circuit breaker wrapper for `provider.Provider`
- Configurable retry with exponential backoff for non-streaming requests
- Metrics: circuit breaker state, failure count, retry count
- Debug logging integration (spec 026 categories: `providers`)
- Configuration via global `resilience:` config block (per-provider config deferred to multi-provider support)

### Phase 2: Health Probing (future, if needed)
- Background health check goroutine when circuit breaker is open
- Configurable health endpoint (default: `GET /health`)
- Auto-recovery when health checks pass

### Phase 3: Multi-backend support (future, if needed)
- Multiple backend URLs per provider
- Round-robin or least-connections selection
- Per-backend circuit breakers
- This would move Antwort closer to SMG's worker pool model

## Design Constraints

- **Go stdlib only** (constitution Principle II): circuit breaker and retry are simple enough to implement without external libraries
- **No routing intelligence**: Antwort is not a load balancer. Cache-aware routing and worker pool management stay with SMG/llm-d
- **Transparent to engine**: The resilience layer wraps the provider, not the engine. The agentic loop doesn't need to know about retries
- **Streaming caveat**: Retries only apply to non-streaming requests or to the initial connection phase of streaming requests. Once SSE events start flowing, retry is not applicable

## Alternatives Considered

1. **Delegate to service mesh (Istio/Envoy)**: Kubernetes service meshes provide circuit breakers and retries. But not all deployments use a service mesh, and Antwort-level resilience gives better error messages and metrics.
2. **Use a Go resilience library (e.g., sony/gobreaker)**: Violates stdlib-only constitution. The pattern is simple enough to implement in ~100 lines.
3. **Do nothing, rely on client retry**: Pushes complexity to every client. Not acceptable for a production gateway.

## Open Questions (RESOLVED, 2026-03-29)

1. **Per-instance vs shared circuit breaker**: Per-instance. Replicas converge quickly since they all hit the same backend. Shared state would couple resilience to storage, adding complexity for minimal gain.
2. **Background mode interaction**: Workers have their own circuit breaker instance. Staleness detection (spec 044) is orthogonal; circuit breaker protects the provider call, staleness protects the response lifecycle.
3. **Mid-loop failures**: Retry the specific provider call, not the whole response. The model has already done work on earlier turns. If all retries exhausted and circuit trips, engine sees a single error and fails the response.

## Additional Decisions (2026-03-29)

4. **Streaming retry scope**: Retry on connection failure (pre-first-event) only. Mid-stream drops fail the response. Clients can retry with `previous_response_id` to preserve conversation context. Transparent mid-stream recovery is too complex for the edge case.
5. **429 Retry-After**: In scope. Parse `Retry-After` header, wait, retry. 429 retries do NOT count toward circuit breaker failure threshold (rate limiting means "healthy but busy," not "broken").
6. **Configuration location**: Global `resilience:` block, not per-provider. Antwort currently has a single provider. Per-provider config deferred to when multi-provider support lands.
7. **Default behavior**: Opt-in. Zero-config Antwort behaves as today (transparent proxy). Adding `resilience:` config enables the safety net.
8. **Architecture**: Circuit breaker wraps the provider, not the engine. Retries happen at provider level, circuit breaker state updates per-attempt, agentic loop is unaware.

## Spec Scope (047-backend-resilience)

| Feature | In scope | Notes |
|---|---|---|
| Circuit breaker (per-provider, per-instance) | Yes | closed/open/half-open, configurable thresholds |
| Retry with exponential backoff + jitter | Yes | Non-streaming + streaming pre-connection only |
| 429 Retry-After respect | Yes | Separate from circuit breaker failure counting |
| Retryable error classification | Yes | 502/503/504/connection errors = retryable; 4xx = not |
| Mid-stream reconnect | No | Too complex, clients retry with previous_response_id |
| Health probing | No | Phase 2 if operators request it |
| Multi-backend failover | No | Phase 3, separate concern |
| Shared circuit breaker state | No | Per-instance is sufficient |
