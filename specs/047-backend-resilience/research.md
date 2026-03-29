# Research: Backend Resilience (Circuit Breaker + Retry)

## Provider Wrapping Architecture

**Decision**: Implement resilience as a `provider.Provider` decorator that wraps any existing provider.

**Rationale**: The Provider interface has exactly the right surface area. `Complete()` and `Stream()` are the two methods that talk to the backend. `Name()`, `Capabilities()`, `ListModels()`, and `Close()` can delegate directly. The decorator pattern means zero changes to the engine, transport, or any other package.

**Alternatives considered**:
- Middleware in openaicompat HTTP client: Too low-level, wouldn't cover non-HTTP providers. Also couples resilience to a specific protocol adapter.
- Engine-level retry: Violates FR-020 (engine must be unaware). Would also require the engine to understand error classification.
- HTTP transport middleware: Would miss connection-level errors that occur before HTTP response.

## Provider Instantiation Point

**Decision**: Wrap the provider in `cmd/server/main.go` after `createProvider()` returns, before passing to `engine.New()`.

**Rationale**: `createProvider()` returns `provider.Provider`. The resilience wrapper takes a `provider.Provider` and returns a `provider.Provider`. One line of wiring:
```go
prov = resilience.Wrap(prov, resilienceConfig)
```

**Key finding**: The provider is created once and shared across all goroutines. The wrapper must be concurrency-safe. All state (circuit breaker counters, timestamps) uses atomic operations.

## Error Classification

**Decision**: Classify errors at the `*api.APIError` level using `ErrorType` field, plus Go-level network error detection via `errors.Is()` and `errors.As()`.

**Rationale**: Provider methods return `error` which is typically `*api.APIError`. The `ErrorType` field maps cleanly to retryable/non-retryable:
- `ErrorTypeServerError` (5xx, network) -> retryable (but need to check HTTP status for 502/503/504 specifically)
- `ErrorTypeTooManyRequests` (429) -> retryable with Retry-After, does NOT count for circuit breaker
- `ErrorTypeInvalidRequest` (4xx) -> non-retryable
- `ErrorTypeNotFound` (404) -> non-retryable
- `ErrorTypeModelError` -> non-retryable

For network errors (connection refused, reset, timeout), check with `errors.Is(err, syscall.ECONNREFUSED)`, `net.Error` interface for timeouts, and `errors.Is(err, context.DeadlineExceeded)`.

**Key finding**: `openaicompat/errors.go` has `MapHTTPError()` and `MapNetworkError()` that already classify errors. The APIError struct includes the HTTP status code in the `Code` field for some errors. Need to ensure status codes are preserved or enhance APIError to carry them.

## Retry-After Header Access

**Decision**: Enhance `*api.APIError` with an optional `RetryAfter` field (or use a wrapper error type) to propagate the header value from the HTTP response.

**Rationale**: Currently `MapHTTPError()` creates an APIError but discards the `Retry-After` header. The resilience layer needs this value. Options:
1. Add `RetryAfter time.Duration` to `api.APIError` (cleanest, one field)
2. Create a wrapper error in the resilience package that carries both the original error and the retry-after duration
3. Parse the header in the openaicompat layer and attach as metadata

Option 1 is simplest and doesn't create a new type. The field is zero-valued when not applicable.

**Alternative**: Option 2 avoids modifying api.APIError but adds type assertion complexity. Option 1 preferred for simplicity.

## Streaming Retry Boundary

**Decision**: Retry only the `Stream()` call itself (which establishes the connection and returns the channel). Once the channel is returned successfully, no retry.

**Rationale**: `provider.Stream()` returns `(<-chan ProviderEvent, error)`. The error return covers connection failures. Once the channel is returned, events flow. If the channel receives a `ProviderEventError`, that's mid-stream and should not be retried.

**Key finding**: In `openaicompat/client.go`, the streaming path creates the HTTP request, sends it, checks the status code, and then returns the channel. Connection errors and HTTP error responses occur before the channel is returned. This aligns perfectly with our retry boundary.

## Circuit Breaker State Machine

**Decision**: Three states with atomic transitions. No mutex needed for the happy path.

**Rationale**:
- `closed` (0): Normal. Each failure increments atomic counter. When counter >= threshold, CAS to `open` and record timestamp.
- `open` (1): Fast-fail. Check if `resetTimeout` has elapsed since last failure. If yes, CAS to `half-open`.
- `half-open` (2): Allow exactly one request (use CAS to prevent races). On success, CAS to `closed` and reset counter. On failure, CAS to `open` and update timestamp.

All operations use `sync/atomic` for lock-free concurrency. The only coordination point is half-open, where CAS ensures only one goroutine gets the probe slot.

## Metrics Naming Convention

**Decision**: Follow existing `antwort_` prefix pattern with `resilience_` subsystem.

**Rationale**: Existing metrics use `antwort_requests_total`, `antwort_request_duration_seconds`. Resilience metrics follow the same pattern:
- `antwort_resilience_circuit_breaker_state` (Gauge, labels: provider)
- `antwort_resilience_circuit_breaker_transitions_total` (Counter, labels: provider, from, to)
- `antwort_resilience_retry_attempts_total` (Counter, labels: provider, outcome)
- `antwort_resilience_retry_exhausted_total` (Counter, labels: provider)

All registered in `pkg/observability/metrics.go` `init()` following the existing pattern.

## Configuration Defaults

**Decision**: Sensible defaults that match common production patterns.

**Rationale**: Based on SMG defaults and industry standards:
- `failure_threshold`: 5 (consecutive failures to trip circuit)
- `reset_timeout`: 30s (time in open state before probing)
- `max_attempts`: 3 (total attempts including original)
- `backoff_base`: 100ms (first retry wait)
- `backoff_max`: 2s (cap on exponential growth)
- `retry_after_max`: 30s (cap on 429 Retry-After, prevent indefinite waits)

These can be tuned without code changes via the YAML config block.

## Debug Logging Category

**Decision**: Use existing `providers` category. No new category needed.

**Rationale**: Resilience wraps the provider. Debug output logically belongs to the `providers` category. Users already enable `ANTWORT_DEBUG=providers` to debug backend communication. Resilience events (retries, circuit state changes) are part of that same debugging flow.
