# Data Model: Backend Resilience (Circuit Breaker + Retry)

## Entities

### CircuitBreaker

Tracks backend health state per provider instance. Uses atomic operations for lock-free concurrency.

| Field | Type | Description |
|-------|------|-------------|
| state | int32 (atomic) | 0=closed, 1=open, 2=half-open |
| consecutiveFailures | int64 (atomic) | Failure count, reset on success |
| lastFailureTime | int64 (atomic) | Unix nano timestamp of last failure |
| failureThreshold | int64 | Config: failures before open (default: 5) |
| resetTimeout | time.Duration | Config: time in open before half-open probe (default: 30s) |

**State Transitions**:

```text
closed --[consecutiveFailures >= threshold]--> open
open   --[resetTimeout elapsed]--------------> half-open
half-open --[probe succeeds]-----------------> closed
half-open --[probe fails]--------------------> open
```

**Invariants**:
- Only one probe request in half-open (enforced by CAS)
- Success in closed state resets consecutiveFailures to 0
- 429 responses never increment consecutiveFailures

### RetryPolicy

Defines retry behavior for a request. Immutable after construction.

| Field | Type | Description |
|-------|------|-------------|
| maxAttempts | int | Total attempts including original (default: 3) |
| backoffBase | time.Duration | Initial backoff duration (default: 100ms) |
| backoffMax | time.Duration | Maximum backoff cap (default: 2s) |
| retryAfterMax | time.Duration | Maximum Retry-After wait (default: 30s) |

**Backoff Calculation**:
- Attempt N wait = min(backoffBase * 2^(N-1) + jitter, backoffMax)
- Jitter: random duration in [0, backoffBase) added to each wait
- 429 with Retry-After: use header value instead (capped at retryAfterMax)

### ResilienceConfig

Global configuration block parsed from YAML. Maps to RetryPolicy + CircuitBreaker construction.

| Field | YAML key | Type | Default | Description |
|-------|----------|------|---------|-------------|
| Enabled | enabled | bool | false | Master switch |
| FailureThreshold | failure_threshold | int | 5 | Consecutive failures to trip circuit |
| ResetTimeout | reset_timeout | duration | 30s | Open state duration before half-open probe |
| MaxAttempts | max_attempts | int | 3 | Total attempts (1 = no retry) |
| BackoffBase | backoff_base | duration | 100ms | Initial backoff |
| BackoffMax | backoff_max | duration | 2s | Maximum backoff cap |
| RetryAfterMax | retry_after_max | duration | 30s | Maximum 429 Retry-After wait |

**YAML example (minimal)**:
```yaml
resilience:
  enabled: true
```

**YAML example (tuned)**:
```yaml
resilience:
  enabled: true
  failure_threshold: 10
  reset_timeout: 60s
  max_attempts: 5
  backoff_base: 200ms
  backoff_max: 5s
  retry_after_max: 60s
```

### Error Classification

Not a persisted entity. A function that maps errors to retry decisions.

| Error Category | Retryable | Circuit Breaker Impact |
|---------------|-----------|----------------------|
| HTTP 502/503/504 | Yes | Increments failure count |
| Connection refused/reset | Yes | Increments failure count |
| Context deadline exceeded | Yes | Increments failure count |
| HTTP 429 (rate limited) | Yes (with Retry-After) | No impact |
| HTTP 400/401/403/404/422 | No | No impact |
| Circuit open rejection | No | No impact (already counted) |
| Context cancelled | No | No impact |

## Relationships

```text
ResilienceConfig --[constructs]--> CircuitBreaker
ResilienceConfig --[constructs]--> RetryPolicy
ResilientProvider --[wraps]--> provider.Provider
ResilientProvider --[uses]--> CircuitBreaker
ResilientProvider --[uses]--> RetryPolicy
ResilientProvider --[uses]--> ErrorClassifier
```

## No Persistence

All state is in-memory per Antwort instance. Circuit breaker state resets on restart. No storage dependencies.
