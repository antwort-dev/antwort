# Review Guide: Production Metrics Taxonomy

**Spec:** specs/046-metrics-taxonomy/spec.md | **Plan:** specs/046-metrics-taxonomy/plan.md
**Generated:** 2026-03-29

---

## What This Spec Does

Adds 23 new Prometheus metrics to Antwort, expanding the 12 core metrics from spec 013 into a comprehensive taxonomy covering 5 subsystem layers: Responses API, Engine/Agentic Loop, Storage, Files/Vector Store, and Background Workers. This gives operators visibility into subsystems that are currently opaque, such as agentic loop iteration depth, storage latency, and background worker queue health.

**In scope:** 23 new Prometheus metric definitions, instrumentation at observation points across 6 packages, unit and integration tests, metrics reference documentation.

**Out of scope:** Provider resilience metrics (circuit breaker, retries) are explicitly deferred to a backend resilience spec. Grafana dashboards and alerting rules are operator-managed. OpenTelemetry tracing is a separate concern.

## Bigger Picture

Spec 013 built the observability foundation (HTTP, provider, tool metrics). As Antwort grew through specs 034-044, new subsystems (files, vector store, background workers, agentic loops) shipped without metrics. This spec closes that gap. It is a natural follow-up, not a redesign. The existing `pkg/observability/` package, `prometheus/client_golang` dependency, and `/metrics` endpoint are reused without modification.

This spec has no downstream dependents. It is purely additive and can be shipped without affecting any other feature. The background resilience spec (brainstorm 41) may add provider-level metrics later, following the same pattern.

---

## Spec Review Guide (30 minutes)

> Focus your time on the parts that need human judgment most. Each section points to specific locations and frames the review as questions.

### Understanding the approach (8 min)

Read `spec.md` sections "Overview" and "Requirements > Metric Standards" (FR-024 through FR-026). As you read, consider:

- Does the 5-layer taxonomy match your mental model of Antwort's architecture? Are there subsystems missing that operators would want to monitor?
- The spec requires all histograms to use the same LLM-tuned buckets (0.1s to 120s). Is this appropriate for storage operations, which are typically much faster (sub-millisecond for memory store)?
- The spec says metrics are "always registered" with no feature flags. Does this match operational expectations, or should subsystem metrics be opt-in?

### Key decisions that need your eyes (12 min)

**Shared histogram buckets for all durations** (spec.md FR-024)

All duration histograms use `LLMBuckets` (0.1, 0.5, 1, 2, 5, 10, 30, 60, 120 seconds). This is optimized for LLM inference latencies but may be a poor fit for storage operations (microseconds) or file ingestion (minutes).
- Question for reviewer: Should storage metrics use finer-grained buckets (e.g., 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1s)? The spec explicitly says "consistent buckets" but consistency may sacrifice precision.

**Direct instrumentation over decorator pattern** (plan.md "Implementation Approach")

The plan instruments storage operations by modifying the store implementations directly rather than using a metrics wrapper/decorator around the `ResponseStore` interface.
- Question for reviewer: A decorator pattern would be cleaner architecturally (single instrumentation point, no store modification). Is the simplicity of direct instrumentation worth the trade-off of touching every store method in both memory and postgres implementations?

**Conversation depth metric definition** (spec.md FR-010)

FR-010 defines `antwort_engine_conversation_depth` as counting "items in rehydrated conversations." This could mean message count, turn count, or total items including tool results.
- Question for reviewer: Should "depth" count messages (user + assistant), or all items (including tool call/result items which can inflate the count significantly)?

**No cardinality controls in code** (spec.md Clarifications)

The spec explicitly defers cardinality management to operators via Prometheus relabeling. High-cardinality labels like `model` and `tool_name` are used as-is.
- Question for reviewer: Is this acceptable for your deployment? A deployment with 50+ model names could generate significant metric volume. Should there be a configurable allowlist?

### Areas where I'm less certain (5 min)

- `spec.md` FR-013 (`storage_responses_stored` gauge): Keeping an accurate gauge of stored responses requires incrementing on save and decrementing on delete. Soft deletes, LRU eviction, and TTL cleanup in the memory store may cause drift between the gauge and actual count. A periodic reconciliation or callback-based gauge might be more accurate.

- `spec.md` FR-023 (`background_worker_heartbeat_age_seconds`): This gauge per worker_id will grow stale if a worker dies. Prometheus will continue reporting the last value. Should there be a mechanism to clean up gauges for dead workers, or is this an acceptable trade-off?

- `plan.md` response metrics location: The plan places response-level metrics (responses_total, responses_active) in `engine.go` CreateResponse. But background mode returns immediately with status "queued" and the actual completion happens later in the worker. How should duration and final status be recorded for background responses?

### Risks and open questions (5 min)

- The `store_id` label on vector store metrics uses the vector store identifier. In the current code, vector stores are identified by collection name. Is this stable and predictable enough for operators to build dashboards around, or could it change between deployments?

- FR-005 (`responses_tokens_total`) records tokens with labels `model` and `type` (input/output). This overlaps significantly with the existing `antwort_provider_tokens_total` metric from spec 013 which has labels `provider`, `model`, `direction`. Is the overlap intentional (different perspectives), or should the response-level metric be removed to avoid confusion?

- The spec mentions 35 total metrics (SC-001) but counts 12 existing + 23 new. The existing count of 12 includes 4 OTel `gen_ai_*` metrics. Are the `gen_ai_*` metrics counted in the "35 total" or are they separate (which would make the real total 39)?

---
*Full context in linked spec and plan.*
