# Deep Review Findings

**Date:** 2026-03-29
**Branch:** 047-backend-resilience
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** superpowers

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 0 | 0 | 0 |
| Important | 2 | 2 | 0 |
| Minor | 3 | 3 | 0 |
| **Total** | **5** | **5** | **0** |

**Agents completed:** 5/5 (+ 1 external tool)
**Agents failed:** none

## Round 1

### FINDING-1
- **Severity:** Important
- **Confidence:** 90
- **File:** pkg/provider/resilience/resilience.go:214-224
- **Category:** architecture
- **Source:** correctness-agent (also: architecture-agent, coderabbit)
- **Description:** `apiErrAs` used direct type assertion instead of `errors.As`, missing wrapped errors in the error chain.
- **Resolution:** fixed (round 1) - replaced with `errors.As(err, &apiErr)`

### FINDING-2
- **Severity:** Important
- **Confidence:** 95
- **File:** pkg/provider/resilience/resilience.go:238-242, pkg/observability/metrics.go:335
- **Category:** architecture
- **Source:** architecture-agent (also: production-readiness-agent)
- **Description:** `ResilienceCircuitBreakerTransitionsTotal` counter declared but never incremented. `recordCircuitTransition()` was identical to `recordCircuitState()`.
- **Resolution:** fixed (round 1) - renamed to `recordCircuitTransitionFrom(prevState)`, now captures previous state and increments transitions counter with from/to labels, plus debug logging on state change.

### FINDING-3
- **Severity:** Minor
- **Confidence:** 90
- **File:** pkg/provider/resilience/circuit.go:130-137
- **Category:** architecture
- **Source:** architecture-agent
- **Description:** `CircuitOpenError` type defined but never used (dead code). Actual circuit open error created inline via `circuitOpenError()` method.
- **Resolution:** fixed (round 1) - removed unused type and its `fmt` import.

### FINDING-4
- **Severity:** Minor
- **Confidence:** 90
- **File:** pkg/provider/resilience/retry.go:25
- **Category:** correctness
- **Source:** correctness-agent
- **Description:** `rand.Int63n(int64(base))` panics if `base` is 0. While config validation prevents this, a defensive guard is better.
- **Resolution:** fixed (round 1) - added `if base > 0` guard around jitter calculation.

### FINDING-5
- **Severity:** Minor
- **Confidence:** 80
- **File:** pkg/provider/resilience/resilience.go:104
- **Category:** correctness
- **Source:** correctness-agent
- **Description:** Comment "Should not be reached, but safeguard" is misleading. The code can be reached via RateLimited classification on every attempt.
- **Resolution:** fixed (round 1) - updated comment to "All retry attempts exhausted (e.g., rate-limited on every attempt)."
