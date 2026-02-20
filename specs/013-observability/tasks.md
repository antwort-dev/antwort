# Tasks: Observability (Prometheus Metrics)

## Phase 1: Metrics Package (P1)

- [ ] T001 [US1] Add `github.com/prometheus/client_golang` to go.mod. Create `pkg/observability/metrics.go` with all metric definitions: antwort_requests_total (counter), antwort_request_duration_seconds (histogram), antwort_streaming_connections_active (gauge), antwort_provider_requests_total (counter), antwort_provider_latency_seconds (histogram), antwort_provider_tokens_total (counter), antwort_tool_executions_total (counter), antwort_ratelimit_rejected_total (counter). Use LLM-tuned buckets {0.1, 0.5, 1, 2, 5, 10, 30, 60, 120} (FR-004 to FR-012).
- [ ] T002 [US1] Create `pkg/observability/middleware.go`: HTTP middleware that records request count, duration, and streaming gauge. Wraps http.ResponseWriter to capture status code (FR-004, FR-005, FR-006).
- [ ] T003 Write `pkg/observability/metrics_test.go`: verify metrics are registered, middleware records request count and duration, streaming gauge increments/decrements.

**Checkpoint**: Metrics package ready.

---

## Phase 2: Server Integration (P1)

- [ ] T004 [US1] [US2] Wire metrics into `cmd/server/main.go`: add /metrics endpoint using promhttp.Handler(), add metrics middleware to handler chain, add /metrics to auth bypass list (FR-001, FR-002).
- [ ] T005 [US2] Add provider metrics recording in engine: record provider latency and token counts after each Complete/Stream call (FR-007, FR-008, FR-009).
- [ ] T006 [US3] Add tool execution metrics in engine loop: record tool_executions_total on each Execute call (FR-010).
- [ ] T007 Add rate limit rejection metric in auth middleware (FR-011).

**Checkpoint**: All metrics flowing.

---

## Phase 3: Config + Polish

- [ ] T008 Add metrics config to Spec 012 config struct: observability.metrics.enabled, observability.metrics.path (FR-013).
- [ ] T009 [P] Run `go vet ./...` and `go test ./...`.
- [ ] T010 [P] Run `make conformance` to verify no regressions.

---

## Dependencies

- Phase 1: No dependencies.
- Phase 2: Depends on Phase 1.
- Phase 3: Depends on Phase 2.
