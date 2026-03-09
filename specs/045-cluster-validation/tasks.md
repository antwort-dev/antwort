# Tasks: Real-Cluster Validation Harness

**Input**: Design documents from `/specs/045-cluster-validation/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: The feature IS a test suite. Test validation is inherent to every task.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create project structure, shared test helpers, and result collection framework

- [ ] T001 Create `test/cluster/` directory structure with `cluster_test.go` containing `TestMain`, environment variable parsing (`CLUSTER_ANTWORT_URL`, `CLUSTER_VLLM_URL`, `CLUSTER_API_KEY`, `CLUSTER_MODEL`, `CLUSTER_TIMEOUT`), cluster reachability check (HTTP GET to health endpoint, skip all tests if unreachable), and openai-go client factory helpers (`newAntwortClient()`, `newVLLMClient()`). Include build tag `//go:build cluster`.
- [ ] T002 [P] Create `test/cluster/results.go` with `ResultCollector` struct: thread-safe result collection via `Record(TestResult)`, latency percentile calculation (P50/P95/P99), `WriteJSON(dir string)` method that writes `ResultSummary` as timestamped JSON to `test/cluster/results/raw/`. Include `CategoryScore`, `LatencyStats`, `FailureDetail` types from data-model.md.
- [ ] T003 [P] Create `test/cluster/results/` directory with `.gitkeep` and `.gitignore` (ignore `raw/` and `*.md` except committed runs). Create `test/cluster/README.md` documenting environment variables, prerequisites (cluster access, deployed model), and how to run (`go test -tags cluster ./test/cluster/ -v`).
- [ ] T004 [P] Add `cluster-test` target to `Makefile`: runs `go test -tags cluster ./test/cluster/ -v -timeout 300s` with appropriate env var passthrough. Add `cluster-test-bfcl` target that adds `-run TestBFCL`.

**Checkpoint**: Test skeleton exists, ResultCollector works, `make cluster-test` runs (skips if no cluster).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: BFCL data loading, type conversion, and AST scoring that multiple user stories depend on

**CRITICAL**: US3 and US4 depend on this phase. US1 and US2 can start in parallel with Phase 2.

- [ ] T005 Create `test/cluster/bfcl_loader.go`: implement `LoadBFCLCases(dir string, category string) ([]BFCLCase, error)` that reads JSONL files from `test/cluster/testdata/bfcl/`, parses question/function/ground_truth fields. Implement `convertGorillaTool(fn json.RawMessage) api.Tool` that maps Gorilla types to OpenAPI (`dict->object`, `float->number`, `tuple->array`, `any->string`) and replaces dots with underscores in function names.
- [ ] T006 [P] Create `test/cluster/bfcl_scorer.go`: implement AST evaluation with `ScoreBFCL(expected []FunctionCall, got []ParsedCall) bool`. Include `simpleChecker` (1 call, name match, required params, value in acceptable list), `parallelChecker` (order-independent matching), `multipleChecker` (1 call from multiple options), `irrelevanceChecker` (no calls expected). Implement `standardizeString(s string) string` for case-insensitive comparison with punctuation stripping.
- [ ] T007 Create `test/cluster/testdata/bfcl/` with the fixed 180-case subset: download BFCL v4 data from `github.com/ShishirPatil/gorilla` repository, extract first 50 `simple_python`, 50 `multiple`, 30 `parallel`, 20 `parallel_multiple`, 30 `irrelevance` cases. Convert from Gorilla format to OpenAPI format. Store as JSONL files with matching ground truth in `answers/` subdirectory.

**Checkpoint**: BFCL loader parses test data, scorer evaluates function calls correctly.

---

## Phase 3: User Story 1 - Basic Inference Validation (Priority: P1) MVP

**Goal**: Verify Antwort proxies inference requests correctly to a real LLM backend.

**Independent Test**: `go test -tags cluster ./test/cluster/ -run TestBasic -v`

### Implementation for User Story 1

- [ ] T008 [US1] Write `test/cluster/basic_test.go` with `TestBasicNonStreaming`: use openai-go SDK to POST a non-streaming response to Antwort, verify response has valid ID (`resp_` prefix), model name matching `CLUSTER_MODEL`, non-empty output text, and usage statistics (prompt_tokens > 0, completion_tokens > 0). Record result to `ResultCollector` with latency.
- [ ] T009 [US1] Add `TestBasicStreaming` to `test/cluster/basic_test.go`: use openai-go SDK streaming API, measure TTFT (time from request to first `response.output_text.delta` event), collect all events, verify complete lifecycle (response.created, content deltas, response.completed), verify assembled text is non-empty and coherent. Record TTFT and total duration to `ResultCollector`.
- [ ] T010 [US1] Add `TestBasicMultipleRequests` to `test/cluster/basic_test.go`: send 10 sequential requests with temperature=0 and the same prompt, collect latencies, verify all responses are structurally valid. This provides the data for P50/P95/P99 latency calculation in reports.

**Checkpoint**: Basic inference validated against real cluster. `ResultCollector` has latency data.

---

## Phase 4: User Story 2 - Multi-Provider Comparison (Priority: P1)

**Goal**: Compare behavior across Antwort (Chat Completions), Antwort (Responses API), and direct vLLM paths.

**Independent Test**: `go test -tags cluster ./test/cluster/ -run TestMultiProvider -v`

### Implementation for User Story 2

- [ ] T011 [US2] Write `test/cluster/provider_test.go` with `TestMultiProviderNonStreaming`: define a shared prompt, send it through the Antwort client (URL from `CLUSTER_ANTWORT_URL`). If `CLUSTER_VLLM_URL` is set, also send directly to vLLM using a raw HTTP client. Compare: both return valid response structure, model names match, output text is non-empty. Record per-path latencies to `ResultCollector` with `ProviderPath` field.
- [ ] T012 [US2] Add `TestMultiProviderStreaming` to `test/cluster/provider_test.go`: same comparison for streaming. Measure TTFT per path. Record per-path TTFT to `ResultCollector`.
- [ ] T013 [US2] Add `TestMultiProviderOverhead` to `test/cluster/provider_test.go`: send 5 identical requests through each available path, calculate per-path average latency, log the delta between Antwort and direct vLLM as "gateway overhead". Skip if `CLUSTER_VLLM_URL` is not set.

**Checkpoint**: Multi-provider comparison works. Gateway overhead is measurable.

---

## Phase 5: User Story 3 - Tool Calling Validation (Priority: P2)

**Goal**: Verify Antwort's agentic loop handles tool calling with real LLM responses.

**Independent Test**: `go test -tags cluster ./test/cluster/ -run TestTools -v`

### Implementation for User Story 3

- [ ] T014 [US3] Write `test/cluster/tools_test.go` with `TestToolCallSimple`: define a `get_weather` function tool, send a prompt like "What is the weather in San Francisco?", verify the response contains a function_call output item with name `get_weather` and arguments containing a location field. Use non-streaming mode.
- [ ] T015 [US3] Add `TestToolCallNoCall` to `test/cluster/tools_test.go`: define tools but send a prompt that should not trigger any tool call (e.g., "What is 2+2?"), verify the response contains only text output with no function_call items.
- [ ] T016 [US3] Add `TestToolCallStreaming` to `test/cluster/tools_test.go`: same as T014 but with streaming enabled. Verify function_call SSE events are received (`response.function_call_arguments.delta`, `response.function_call_arguments.done`).

**Checkpoint**: Tool calling works with real model responses. Both streaming and non-streaming validated.

---

## Phase 6: User Story 4 - BFCL Benchmark Subset (Priority: P2)

**Goal**: Run standardized BFCL benchmark and produce scored results.

**Independent Test**: `go test -tags cluster ./test/cluster/ -run TestBFCL -v`

### Implementation for User Story 4

- [ ] T017 [US4] Write `test/cluster/bfcl_test.go` with `TestBFCLSimple`: load `simple_python` cases from `testdata/bfcl/`, run each as a table-driven subtest. For each case: send prompt with tools to Antwort, extract function_call output items, parse arguments JSON, score against ground truth using `simpleChecker`. Record pass/fail per case to `ResultCollector` with category `bfcl_simple`.
- [ ] T018 [US4] Add `TestBFCLMultiple`, `TestBFCLParallel`, `TestBFCLParallelMultiple`, `TestBFCLIrrelevance` to `test/cluster/bfcl_test.go`: each loads its category's test data, runs subtests, uses the appropriate scorer (multiple, parallel, irrelevance). Irrelevance tests verify NO function calls are produced.
- [ ] T019 [US4] Add `TestBFCLAll` to `test/cluster/bfcl_test.go`: guarded by `-bfcl-all` test flag (registered in `TestMain`). Downloads full BFCL dataset from GitHub if not cached locally, runs all ~4,700 cases across all non-live categories. Skip if flag not set. Use `testing.Short()` to skip long-running tests.

**Checkpoint**: BFCL benchmark produces per-category scores. Fixed subset is reproducible.

---

## Phase 7: User Story 5 - Validation Results as Documentation (Priority: P2)

**Goal**: Generate publishable markdown reports and JSON summaries from test runs.

**Independent Test**: Run any test category, verify reports are generated in `test/cluster/results/`.

### Implementation for User Story 5

- [ ] T020 [US5] Update `test/cluster/cluster_test.go` TestMain teardown to call `ResultCollector.WriteJSON()` after all tests complete. Write to `test/cluster/results/raw/<timestamp>_<model>.json`. Add `CLUSTER_ANTWORT_VERSION` env var (defaults to git commit hash via `git rev-parse --short HEAD`).
- [ ] T021 [US5] Create `test/cluster/run.sh`: shell orchestrator that (1) checks cluster reachability via curl to `CLUSTER_ANTWORT_URL/healthz`, (2) prompts for model name if `CLUSTER_MODEL` not set, (3) runs `go test -tags cluster ./test/cluster/ -v -timeout 300s`, (4) calls `report.sh` to generate markdown from the JSON output.
- [ ] T022 [US5] Create `test/cluster/report.sh`: reads the latest JSON from `test/cluster/results/raw/`, generates a timestamped markdown report in `test/cluster/results/` with model name, cluster details, Antwort version, per-category score table, latency percentiles table, and failure details section. Updates `latest.md` symlink and writes `latest.json`.

**Checkpoint**: Complete reporting pipeline works. `run.sh` produces publishable results.

---

## Phase 8: User Story 6 - Feature Coverage Validation (Priority: P3)

**Goal**: Validate background mode, RAG, auth, and conversation features against real cluster.

**Independent Test**: `go test -tags cluster ./test/cluster/ -run TestBackground -v` (or TestRAG, TestAuth, TestConversations)

### Implementation for User Story 6

- [ ] T023 [P] [US6] Write `test/cluster/features_test.go` with `TestBackgroundSubmitAndPoll`: submit a request with `background: true`, verify 202 response with queued status, poll until completed or timeout (30s), verify final response has valid output. Requires Antwort deployed with PostgreSQL and background mode.
- [ ] T024 [P] [US6] Add `TestRAGFileSearch` to `test/cluster/features_test.go`: upload a file via Files API, wait for processing, send a query with `file_search` tool, verify response includes citations. Requires Antwort deployed with vector store. Skip if Files API returns 404.
- [ ] T025 [P] [US6] Add `TestAuthAccepted` and `TestAuthRejected` to `test/cluster/features_test.go`: test with valid API key (expect 200) and invalid key (expect 401). Skip if auth is not configured (probe with empty key, skip if 200).
- [ ] T026 [P] [US6] Add `TestConversationChaining` to `test/cluster/features_test.go`: create a response, then create a second response with `previous_response_id` set to the first. Verify the second response references the first and produces contextually relevant output. Skip if storage is not configured.

**Checkpoint**: Feature-specific tests validate advanced capabilities when infrastructure is available.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, CI integration hints, and final cleanup

- [ ] T027 [P] Create `docs/modules/operations/pages/validation.adoc` documenting the validation harness: purpose, how to run, environment variables, BFCL methodology, interpreting results. Update `docs/modules/operations/nav.adoc` to include the new page.
- [ ] T028 [P] Update `README.md` spec table to include spec 045. Add a "Validation" section under Platform Vision describing the real-cluster validation capability.
- [ ] T029 Verify full validation pipeline by running `test/cluster/run.sh` against a live cluster (manual verification, not automated). Confirm: tests run, results JSON is written, markdown report is generated, `latest.md` symlink works.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies, start immediately
- **Foundational (Phase 2)**: Depends on T001 (TestMain, env vars). Can run in parallel with US1 and US2.
- **US1 Basic Inference (Phase 3)**: Depends on Phase 1 (T001 for client helpers)
- **US2 Multi-Provider (Phase 4)**: Depends on Phase 1 (T001 for client helpers)
- **US3 Tool Calling (Phase 5)**: Depends on Phase 1 (T001 for client helpers)
- **US4 BFCL (Phase 6)**: Depends on Phase 2 (T005-T007 for loader, scorer, test data)
- **US5 Results (Phase 7)**: Depends on T002 (ResultCollector) + at least one test phase complete
- **US6 Features (Phase 8)**: Depends on Phase 1 (T001 for client helpers)
- **Polish (Phase 9)**: Depends on US5 (reporting pipeline complete)

### User Story Dependencies

- **US1 (P1)**: After Setup. No story dependencies.
- **US2 (P1)**: After Setup. No story dependencies. Can parallel with US1.
- **US3 (P2)**: After Setup. No story dependencies. Can parallel with US1/US2.
- **US4 (P2)**: After Foundational. Depends on BFCL loader and scorer.
- **US5 (P2)**: After Setup (T002). Integrates with all test phases.
- **US6 (P3)**: After Setup. No story dependencies. All tests skip gracefully if features unavailable.

### Parallel Opportunities

- T002, T003, T004 all parallel (different files)
- T005, T006 parallel (different files)
- US1, US2, US3 can proceed in parallel after Setup (different test files)
- T023, T024, T025, T026 all parallel (different test functions, same file but independent)
- T027, T028 parallel (different files)

---

## Implementation Strategy

### MVP First (US1 + US2 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 3: US1 - Basic inference tests (T008-T010)
3. Complete Phase 4: US2 - Multi-provider comparison (T011-T013)
4. **STOP and VALIDATE**: Basic inference works, provider paths compared
5. This is a usable validation harness

### Incremental Delivery

1. Setup + US1 -> Basic inference validated
2. US2 -> Provider comparison working
3. Foundational (T005-T007) -> BFCL infrastructure ready
4. US3 -> Tool calling validated
5. US4 -> BFCL benchmark running
6. US5 -> Reports and documentation
7. US6 -> Feature coverage
8. Polish -> Docs site integration

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Cluster tests use build tag `//go:build cluster` to avoid running with `go test ./...`
- All tests depend on cluster being reachable via `CLUSTER_ANTWORT_URL`
- Tests skip gracefully when required infrastructure is not available
- BFCL test data is committed as a fixed subset for reproducibility
- 29 total tasks across 9 phases

## Beads Task Management

This project uses beads (`bd`) for persistent task tracking across sessions:
- Run `/sdd:beads-task-sync` to create bd issues from this file
- `bd ready --json` returns unblocked tasks (dependencies resolved)
- `bd close <id>` marks a task complete (use `-r "reason"` for close reason, NOT `--comment`)
- `bd comments add <id> "text"` adds a detailed comment to an issue
- `bd sync` persists state to git
- `bd create "DISCOVERED: [short title]" --labels discovered` tracks new work
  - Keep titles crisp (under 80 chars); add details via `bd comments add <id> "details"`
- Run `/sdd:beads-task-sync --reverse` to update checkboxes from bd state
- **Always use `jq` to parse bd JSON output, NEVER inline Python one-liners**
