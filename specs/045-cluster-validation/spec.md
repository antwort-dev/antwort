# Feature Specification: Real-Cluster Validation Harness

**Feature Branch**: `045-cluster-validation`
**Created**: 2026-03-08
**Status**: Draft
**Input**: Brainstorm 40: Real-cluster validation harness with BFCL benchmarks against ROSA HCP clusters

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Basic Inference Validation (Priority: P1)

A developer wants to verify that Antwort correctly proxies inference requests to a real LLM backend on a ROSA HCP cluster. They run a validation suite that sends non-streaming and streaming requests through Antwort and confirms the responses are well-formed, complete, and match expected structure.

**Why this priority**: Without basic inference working, nothing else matters. This is the foundation all other validation builds on.

**Independent Test**: Deploy Antwort with a vLLM backend on a ROSA HCP cluster, run `go test -tags cluster ./test/cluster/ -run TestBasic`, verify responses contain valid text output with correct structure.

**Acceptance Scenarios**:

1. **Given** Antwort is deployed on a ROSA HCP cluster with a vLLM backend serving a model, **When** a developer runs the basic inference tests, **Then** non-streaming requests return a complete response with valid ID, model name, output text, and usage statistics.
2. **Given** Antwort is deployed with streaming enabled, **When** a developer runs the streaming inference tests, **Then** the test receives the complete SSE event lifecycle (response.created, content deltas, response.completed) and the assembled text is coherent.
3. **Given** no cluster is reachable, **When** a developer runs the tests, **Then** the test suite skips with a clear message explaining the cluster is not available.

---

### User Story 2 - Multi-Provider Comparison (Priority: P1)

A developer wants to compare Antwort's behavior across different provider paths to verify translation fidelity and measure gateway overhead. They run the same test scenarios through three paths: Antwort with Chat Completions translation, Antwort with Responses API passthrough, and directly against vLLM as a baseline.

**Why this priority**: Comparing provider paths reveals translation bugs and performance regressions that mock-based tests cannot catch. This is equally critical to basic inference for validating the gateway's correctness.

**Independent Test**: Run `go test -tags cluster ./test/cluster/ -run TestMultiProvider`, which executes each test case through all three provider paths and reports per-path results.

**Acceptance Scenarios**:

1. **Given** Antwort is configured with a Chat Completions provider (`vllm` type), **When** the same prompt is sent through Antwort and directly to vLLM, **Then** both responses contain structurally equivalent output (valid text, matching model name, comparable token counts).
2. **Given** Antwort is configured with a Responses API provider (`responses` type), **When** the same prompt is sent through Antwort and directly to vLLM's Responses API, **Then** both responses are structurally equivalent.
3. **Given** all three paths are tested, **When** the results are generated, **Then** the report includes per-path latency (TTFT, total time) so developers can quantify gateway overhead.

---

### User Story 3 - Tool Calling Validation (Priority: P2)

A developer wants to verify that Antwort's agentic loop correctly handles tool calling with a real LLM that may produce varied tool call formats. They run tests that provide tools and prompts designed to trigger tool invocations, then verify the complete tool call lifecycle.

**Why this priority**: Tool calling accuracy varies significantly across models (30-90% in BFCL benchmarks). Real-model validation catches format mismatches and edge cases that deterministic mocks hide.

**Independent Test**: Run `go test -tags cluster ./test/cluster/ -run TestTools`, verify tool calls are correctly parsed, executed, and returned to the model.

**Acceptance Scenarios**:

1. **Given** a request with a simple function tool (e.g., get_weather), **When** the model decides to call the tool, **Then** Antwort correctly parses the tool call, returns a tool result, and the model produces a final text response incorporating the tool output.
2. **Given** a request with multiple tools where the model should not call any, **When** the model responds with text only, **Then** no tool calls are generated and the response completes normally.

---

### User Story 4 - BFCL Benchmark Subset (Priority: P2)

A developer wants to run a standardized benchmark to measure tool calling accuracy for a specific model. They run a fixed subset of the Berkeley Function Calling Leaderboard (BFCL) test cases adapted for the Responses API format and get a scored report.

**Why this priority**: BFCL provides an industry-standard, reproducible measure of tool calling quality. The fixed subset enables comparable results across models and Antwort versions.

**Independent Test**: Run `go test -tags cluster ./test/cluster/ -run TestBFCL`, which executes ~180 fixed test cases and produces accuracy scores by category.

**Acceptance Scenarios**:

1. **Given** the BFCL test data is available in the repository, **When** a developer runs the BFCL tests against a deployed model, **Then** each test case is evaluated and scored (pass/fail based on AST matching of function name and arguments).
2. **Given** the test run completes, **When** results are generated, **Then** the report includes per-category scores (Simple Function, Multiple Function, Parallel Function, Irrelevance Detection) and an overall accuracy percentage.
3. **Given** a developer runs with `--bfcl-all`, **When** the full suite executes, **Then** all ~4,441 BFCL cases are run (longer execution time accepted).

---

### User Story 5 - Validation Results as Documentation (Priority: P2)

A developer or evaluator wants to see published validation results proving Antwort works with specific models on real infrastructure. Results are generated as timestamped markdown files that can be committed to the repository and published to the documentation site.

**Why this priority**: Published results build trust and serve as regression baselines. Without this, validation is ephemeral.

**Independent Test**: After any test run, verify a markdown report and JSON summary are generated in `test/cluster/results/` with correct structure, timestamps, and scores.

**Acceptance Scenarios**:

1. **Given** a validation run completes, **When** the results are generated, **Then** a timestamped markdown file is created with model name, cluster details, Antwort version, per-category pass/fail counts, latency percentiles, and failure details.
2. **Given** a validation run completes, **When** the JSON summary is generated, **Then** it contains machine-readable scores suitable for the documentation site landing page card.
3. **Given** previous results exist, **When** a new run completes, **Then** the `latest.md` symlink is updated to point to the newest result without overwriting historical results.

---

### User Story 6 - Feature Coverage Validation (Priority: P3)

A developer wants to validate Antwort features beyond basic inference: background mode, RAG pipeline, conversation chaining, and authentication. They run feature-specific test suites against a fully-configured cluster deployment.

**Why this priority**: These features depend on additional infrastructure (PostgreSQL, vector store, auth config) and are tested last because they build on the foundation validated by US1-US3.

**Independent Test**: Run `go test -tags cluster ./test/cluster/ -run TestBackground` (or TestRAG, TestConversations, TestAuth) with the appropriate quickstart deployed.

**Acceptance Scenarios**:

1. **Given** Antwort is deployed with PostgreSQL and background mode enabled, **When** a background request is submitted and polled, **Then** the response transitions from queued to completed with valid output.
2. **Given** Antwort is deployed with a vector store and files uploaded, **When** a file_search query is sent, **Then** the response includes citations referencing the uploaded content.
3. **Given** Antwort is deployed with auth enabled, **When** requests are sent with valid and invalid credentials, **Then** authenticated requests succeed and unauthenticated requests are rejected.

---

### User Story 7 - Declarative Cluster Setup via Recipe (Priority: P2)

A developer wants to deploy the complete Antwort validation stack on a ROSA HCP cluster with a single command. They use a declarative recipe file that specifies the cluster configuration, model selection, and Antwort deployment pattern. The cc-rosa plugin handles dependency resolution, component installation, and idempotent reconciliation.

**Why this priority**: Without a repeatable deployment method, each validation run requires manual cluster setup. A recipe makes validation accessible to any team member with cluster access.

**Independent Test**: Run `/rosa:setup .claude/cc-rosa/recipe.yaml` on a fresh cluster, verify all components are deployed and Antwort is reachable.

**Acceptance Scenarios**:

1. **Given** a developer has AWS/ROSA credentials and a cluster exists, **When** they run `/rosa:setup` with the validation recipe, **Then** RHOAI, a model, and Antwort are deployed in dependency order without interactive prompts (configuration from recipe parameters or environment variables).
2. **Given** some components are already deployed (e.g., RHOAI installed, model serving), **When** the recipe is re-run, **Then** already-completed steps are skipped and only missing components are installed (idempotent reconciliation).
3. **Given** a developer wants a different deployment pattern (e.g., RAG with vector store), **When** they modify the recipe to include additional instills (postgres, qdrant), **Then** the additional infrastructure is deployed alongside the base stack.

---

### Edge Cases

- What happens when the model returns malformed tool call arguments (invalid JSON)?
- How does the harness handle model timeouts or very slow inference (>60s)?
- What happens when the cluster is reachable but the model endpoint is not ready?
- How does the BFCL scorer handle cases where the model returns correct function calls with different argument formatting (e.g., "SF" vs "San Francisco")?
- What happens when a test run is interrupted mid-execution (partial results)?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The validation harness MUST be a Go test suite using `//go:build cluster` build tag in `test/cluster/`, runnable via `go test -tags cluster ./test/cluster/`.
- **FR-002**: The harness MUST detect cluster availability before running tests and skip gracefully with a descriptive message when no cluster is reachable.
- **FR-003**: The harness MUST support three provider paths for comparison: Antwort with Chat Completions translation, Antwort with Responses API passthrough, and direct vLLM baseline (no gateway).
- **FR-004**: The harness MUST accept the target model, cluster endpoint, and provider configuration via environment variables or test flags.
- **FR-005**: The harness MUST measure and report latency metrics: time-to-first-token (TTFT) for streaming, total request time for non-streaming, with P50/P95/P99 percentiles.
- **FR-006**: The harness MUST include a shell orchestrator script (`test/cluster/run.sh`) that handles cluster detection, model selection, and report generation.
- **FR-007**: The harness MUST produce timestamped markdown result files in `test/cluster/results/` with model name, cluster details, Antwort version, per-category scores, latency data, and failure details.
- **FR-008**: The harness MUST produce a machine-readable JSON summary (`latest.json`) alongside the markdown report for documentation site integration.
- **FR-009**: The harness MUST include a fixed subset of ~180 BFCL test cases committed to the repository for reproducible benchmark runs.
- **FR-010**: The BFCL scorer MUST evaluate results using AST matching of function names and arguments (consistent with the BFCL evaluation methodology).
- **FR-011**: The harness MUST support a `--bfcl-all` flag to run the full ~4,441 BFCL test case suite.
- **FR-012**: The harness MUST support model selection at runtime (via flag or environment variable) from any model deployed on the cluster.
- **FR-013**: Each test category (basic inference, streaming, tool calling, BFCL, background, RAG, auth, conversations) MUST be independently runnable via `-run TestCategory`.
- **FR-014**: The harness MUST maintain a `latest.md` symlink pointing to the most recent result file.
- **FR-015**: The harness MUST NOT create or tear down clusters. Cluster lifecycle is managed externally (via cc-rosa or manual setup).
- **FR-016**: The project MUST include cc-rosa instills in `.claude/cc-rosa/instills/` for deploying Antwort in different configurations (minimal, production, RAG, background).
- **FR-017**: The project MUST include a declarative cc-rosa recipe (`.claude/cc-rosa/recipe.yaml`) that deploys the complete validation stack (RHOAI, model, Antwort) via `/rosa:setup`.
- **FR-018**: The recipe MUST support idempotent reconciliation, so re-running it skips already-completed deployment steps.
- **FR-019**: Instill parameters (model name, namespace, storage backend) MUST be configurable via recipe parameters or environment variables, not hardcoded.

### Key Entities

- **ValidationRun**: A single execution of the test harness against a specific model and cluster. Identified by timestamp, model name, and Antwort version.
- **TestResult**: Per-test-case outcome (pass/fail, latency, error details) within a validation run.
- **BFCLCase**: A single BFCL benchmark test case with prompt, tools, expected function calls, and category.
- **ResultReport**: The generated markdown and JSON artifacts summarizing a validation run.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can validate Antwort against a real LLM backend on a ROSA HCP cluster in under 15 minutes for the standard suite (basic + tools + BFCL subset).
- **SC-002**: Validation results are comparable across runs: same model + same BFCL subset produces consistent scores (within 5% variance for deterministic test cases with temperature=0).
- **SC-003**: The multi-provider comparison reveals gateway overhead of less than 100ms P95 for non-streaming requests (exclusive of model inference time).
- **SC-004**: Published validation results are accessible on the documentation site and updated after each committed run.
- **SC-005**: A new developer can deploy the validation stack via `/rosa:setup .claude/cc-rosa/recipe.yaml` and run the harness via `test/cluster/run.sh` or `make cluster-test`, following the README without additional guidance.

## Assumptions

- The ROSA HCP cluster is created separately (via cc-rosa `/rosa:create` or manual setup). Component deployment (RHOAI, model, Antwort) is automated via the project recipe and instills.
- The harness runs from a developer's laptop with network access to the cluster's API and routes. It does not run in CI (no GPU runners available).
- vLLM is the primary inference backend. Other runtimes (LiteLLM, Ollama) may be added in future iterations but are not in scope.
- BFCL test data is downloaded from the Hugging Face dataset and converted to Responses API format. The fixed subset is committed to the repository; the full dataset is downloaded on demand.
- Temperature is set to 0 for all benchmark tests to maximize reproducibility.
- The harness uses the `openai-go` SDK (already a test dependency from spec 043) for direct vLLM baseline calls.
