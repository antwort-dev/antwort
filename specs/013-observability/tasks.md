# Tasks: Observability (Prometheus Metrics)

## Phase 1: Metrics Package (P1) ✓ DONE

- [x] T001 [US1] Add prometheus client. Create metrics.go with antwort_* metric definitions and LLM-tuned buckets.
- [x] T002 [US1] Create middleware.go: HTTP middleware recording request count, duration, streaming gauge.
- [x] T003 Write metrics_test.go: 6 tests covering registration, middleware, streaming gauge, status codes.

---

## Phase 2: Server Integration (P1) - Partially done

- [x] T004 [US1] [US2] Wire /metrics endpoint and metrics middleware into cmd/server/main.go.
- [ ] T005 [US2] Add provider metrics recording in engine: record provider latency and token counts after each Complete/Stream call (FR-007, FR-008, FR-009).
- [ ] T006 [US3] Add tool execution metrics in engine loop: record tool_executions_total on each Execute call (FR-010).
- [ ] T007 Add rate limit rejection metric in auth middleware (FR-011).

---

## Phase 3: OTel GenAI Semantic Conventions (P2)

- [ ] T008 Add gen_ai_client_token_usage histogram with OTel GenAI attributes (gen_ai.operation.name, gen_ai.provider.name, gen_ai.token.type, gen_ai.request.model, gen_ai.response.model). Token buckets: 1, 4, 16, 64, 256, 1024, 4096, 16384 (FR-012a, FR-013a).
- [ ] T009 Add gen_ai_client_operation_duration_seconds histogram with OTel GenAI attributes (FR-012b).
- [ ] T010 Add gen_ai_server_time_to_first_token_seconds histogram. Requires timing changes in streaming path to record first chunk timestamp (FR-012c).
- [ ] T011 Add gen_ai_server_time_per_output_token_seconds histogram. Compute (total - TTFT) / (output_tokens - 1) (FR-012d).
- [ ] T012 Write tests for gen_ai.* metrics: verify OTel attributes, TTFT recording, per-token timing.

---

## Phase 4: Config + Polish

- [x] T013 Add observability.metrics config to config struct (enabled/path).
- [ ] T014 [P] Run `go vet ./...` and `go test ./...`.
- [ ] T015 [P] Run `make conformance` to verify no regressions.

---

## Dependencies

- Phase 1: Done.
- Phase 2: T005-T007 remaining (engine/auth instrumentation).
- Phase 3: Depends on Phase 2 (provider metrics infrastructure needed first).
- Phase 4: Depends on all.
