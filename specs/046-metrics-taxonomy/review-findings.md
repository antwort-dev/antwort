# Deep Review Findings

**Date:** 2026-03-29
**Branch:** 046-metrics-taxonomy
**Rounds:** 1
**Gate Outcome:** PASS
**Invocation:** superpowers

## Summary

| Severity | Found | Fixed | Remaining |
|----------|-------|-------|-----------|
| Critical | 2 | 2 | 0 |
| Important | 3 | 3 | 0 |
| Minor | 8 | - | 8 |
| **Total** | **13** | **5** | **8** |

**Agents completed:** 5/5 (0 external tools)
**Agents failed:** none

## Round 1

### FINDING-1
- **Severity:** Critical
- **Confidence:** 95
- **File:** pkg/files/provider.go:18-24, pkg/observability/metrics.go:240-247
- **Category:** architecture
- **Source:** architecture-agent (also: correctness-agent)
- **Description:** `antwort_files_uploaded_total` registered both locally (label `mime_type`) and centrally (label `content_type`). Same metric name with different label sets causes Prometheus registration panic at startup.
- **Resolution:** fixed (round 1) - renamed local metric to `antwort_files_uploads_by_mime_total`

### FINDING-2
- **Severity:** Critical
- **Confidence:** 95
- **File:** pkg/files/provider.go:26-33, pkg/observability/metrics.go:248-255
- **Category:** architecture
- **Source:** architecture-agent
- **Description:** `antwort_files_ingestion_duration_seconds` same dual-registration conflict. Different label sets and bucket configurations.
- **Resolution:** fixed (round 1) - renamed local metric to `antwort_files_ingestion_by_mime_duration_seconds`

### FINDING-3
- **Severity:** Important
- **Confidence:** 90
- **File:** pkg/engine/background.go:115-124, pkg/engine/engine.go:211
- **Category:** correctness
- **Source:** correctness-agent (also: architecture-agent, test-agent)
- **Description:** `BackgroundQueued` gauge always set to 0, never reflecting actual queue depth. Made the metric useless for monitoring.
- **Resolution:** fixed (round 1) - Inc on queue in handleBackground, Dec on claim in pollOnce

### FINDING-4
- **Severity:** Important
- **Confidence:** 80
- **File:** pkg/engine/loop.go:88-98
- **Category:** correctness
- **Source:** test-agent
- **Description:** `EngineIterationDuration` not recorded for the final answer turn (no tool calls exit), creating systematic bias.
- **Resolution:** fixed (round 1) - added duration recording before final answer return in both streaming and non-streaming loops

### FINDING-5
- **Severity:** Important
- **Confidence:** 85
- **File:** pkg/engine/engine.go:447-451
- **Category:** correctness
- **Source:** correctness-agent
- **Description:** Abnormal stream closure (channel closed without done event) recorded as "completed" in response metrics, inflating success count.
- **Resolution:** fixed (round 1) - changed to record as "incomplete" status

## Remaining Findings (Minor - Advisory)

### FINDING-6 (Minor)
- **File:** pkg/observability/metrics.go (various)
- **Category:** security (cardinality)
- **Description:** `store_id`, `worker_id`, `model`, `tool_name`, `content_type` labels have unbounded cardinality potential. Spec explicitly defers cardinality control to operators via Prometheus relabeling.
- **Resolution:** accepted by spec design. Document in metrics reference.

### FINDING-7 (Minor)
- **File:** pkg/engine/engine.go, pkg/engine/loop.go
- **Category:** architecture (duplication)
- **Description:** Response metrics recording duplicated 6-8 times across files. A `recordResponseMetrics()` helper would reduce duplication.
- **Resolution:** deferred. Current approach is explicit and matches existing codebase patterns.

### FINDING-8 (Minor)
- **File:** pkg/storage/memory/memory.go
- **Category:** production-readiness
- **Description:** Metrics recorded inside mutex-held critical sections. Low-priority optimization.
- **Resolution:** deferred. Prometheus operations are sub-microsecond atomics.

### FINDING-9 (Minor)
- **File:** pkg/observability/metrics_test.go
- **Category:** test-quality
- **Description:** No value-assertion tests for spec 046 metrics. Registration test only verifies existence.
- **Resolution:** deferred to integration test tasks (T015, T021, T027, T032, T037).
