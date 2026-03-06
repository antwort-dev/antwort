# Brainstorm 40: Real-Cluster Validation Harness

**Date**: 2026-03-06
**Participants**: Roland Huss
**Goal**: Design a comprehensive test harness that validates Antwort against a real ROSA HCP cluster with production dependencies, publishes results as documentation, and integrates industry-standard benchmarks.

## Motivation

The current test pyramid covers unit tests, in-process integration tests, and CI E2E tests with mock backends. What's missing is validation against a real inference backend on a real Kubernetes cluster. This matters because:

- Mock backends produce deterministic responses but don't test real LLM behavior (hallucinations, variable tool call formats, streaming edge cases)
- Kind clusters in CI don't have GPUs, so no real inference happens
- Users want proof that Antwort works with specific models before adopting it
- Tool calling accuracy varies significantly across models (BFCL shows 30-90% range)

The validation harness fills this gap: run Antwort against a ROSA HCP cluster with vLLM serving real models, capture results, and publish them as verifiable documentation.

## Architecture

### Components

```
Developer laptop
  │
  ├── test/cluster/run.sh          # Orchestrator: cluster check, model select, run tests
  ├── test/cluster/cluster_test.go  # Go test suite (build tag: cluster)
  │
  └── ROSA HCP cluster (AAET profile)
        ├── RHOAI (operator)
        ├── vLLM InferenceService (GPU, selected model)
        ├── Antwort gateway (quickstart deployment)
        ├── PostgreSQL (CrunchyData operator)
        └── Qdrant (vector store)
```

### Workflow

1. Developer runs `test/cluster/run.sh`
2. Script checks cluster status via cc-rosa plugin (`/rosa:status`)
3. Prompts for model selection (from deployed models or install new one)
4. Deploys Antwort quickstart if not running
5. Runs Go test suite against the cluster endpoint
6. Captures results as timestamped markdown
7. Generates summary for documentation site

## Design Decisions

### D1: Go Test Suite with `cluster` Build Tag

**Decision**: Go test files with `//go:build cluster` tag in `test/cluster/`.

**Rationale**: Go tests are extensible (add a new test function for each new feature), integrate with the existing test infrastructure, produce structured output, and can measure latency/timing. Shell scripts are brittle for assertion logic and hard to extend.

The shell wrapper (`run.sh`) handles cluster detection, model selection, and report generation. The Go tests handle actual validation.

**Structure**:
```
test/cluster/
├── run.sh                    # Orchestrator script
├── cluster_test.go           # Test setup, helpers, model selection
├── basic_test.go             # Basic inference (non-streaming, streaming)
├── tools_test.go             # Tool calling (MCP, web search, file search)
├── background_test.go        # Background mode (async requests)
├── auth_test.go              # Authentication and authorization
├── rag_test.go               # RAG pipeline (upload, search, cite)
├── conversations_test.go     # Conversation chaining
├── bfcl_test.go              # BFCL benchmark subset
└── results/                  # Generated results (gitignored except latest)
    ├── latest.md             # Symlink to most recent run
    ├── 2026-03-06_llama-3.1-8b.md
    ├── 2026-03-06_qwen-2.5-72b.md
    └── ...
```

### D2: cc-rosa Instills for All Components

**Decision**: Create Antwort-specific instills in the project repo at `.claude/instills/rosa/` (cc-rosa discovers these automatically as "external instills").

**Instills to create**:

| ID | Name | Requires | Description |
|----|------|----------|-------------|
| `antwort-minimal` | Antwort (Minimal) | model | Deploy quickstart 01-minimal against vLLM |
| `antwort-production` | Antwort (Production) | model, postgres | Deploy quickstart 02 with PostgreSQL |
| `antwort-rag` | Antwort (RAG) | model, postgres, qdrant | Deploy quickstart 08 with full RAG stack |
| `antwort-background` | Antwort (Background) | model, postgres | Deploy quickstart 09 with gateway+worker |
| `postgres` | PostgreSQL (CrunchyData) | rhoai | CrunchyData PostgreSQL for Antwort storage |
| `qdrant` | Qdrant | - | Qdrant vector database for file_search |

Each instill follows the cc-rosa pattern: INSTILL.md (metadata), install.md (procedure), uninstall.md (cleanup), verify.md (smoke test).

**External instill location**: `.claude/instills/rosa/` in the Antwort repo. The cc-rosa plugin auto-discovers these per-project instills.

### D3: Model Selection

**Decision**: The test harness prompts for model selection at runtime. The cc-rosa `model` instill already supports deploying different models.

**Supported models for validation**:
- `meta-llama/Llama-3.1-8B-Instruct` (small, fast, baseline)
- `meta-llama/Llama-3.1-70B-Instruct` (large, production-grade)
- `Qwen/Qwen2.5-7B-Instruct` (current default in quickstarts)
- `Qwen/Qwen2.5-72B-Instruct` (large, strong tool calling)

The test results are tagged with the model name and version, so results from different models are comparable.

### D4: BFCL Integration

**Decision**: Include a subset of BFCL tests adapted for the Responses API format.

BFCL has 4,441+ test cases across categories. Running all of them is impractical for a validation run. Instead, select a representative subset:

| Category | Cases | Why |
|----------|-------|-----|
| Simple Function | 50 | Baseline tool calling |
| Multiple Function | 50 | Function selection accuracy |
| Parallel Function | 30 | Parallel tool invocation |
| Irrelevance Detection | 30 | Knowing when NOT to call tools |
| Multi-Turn Base | 20 | Conversation-aware tool calling |

Total: ~180 cases, runnable in ~10 minutes with a fast backend.

The BFCL test data is downloaded from the [Hugging Face dataset](https://huggingface.co/datasets/gorilla-llm/Berkeley-Function-Calling-Leaderboard) and converted to Responses API format (input items + tools). Results are scored using BFCL's AST evaluation method.

### D5: Result Format

**Decision**: Each test run produces a timestamped markdown file.

```markdown
# Validation Results: Llama 3.1 8B Instruct

**Date**: 2026-03-06T14:30:00Z
**Cluster**: antwort-dev (ROSA HCP, us-east-2)
**Model**: meta-llama/Llama-3.1-8B-Instruct
**Antwort Version**: v0.44.0 (commit abc123)
**Quickstart**: 09-background (gateway + worker + PostgreSQL)

## Summary

| Category | Passed | Total | Score |
|----------|--------|-------|-------|
| Basic Inference | 8 | 8 | 100% |
| Streaming | 5 | 5 | 100% |
| Tool Calling | 12 | 15 | 80% |
| Background Mode | 6 | 6 | 100% |
| RAG Pipeline | 4 | 5 | 80% |
| Conversations | 3 | 3 | 100% |
| BFCL Simple | 42 | 50 | 84% |
| BFCL Multiple | 38 | 50 | 76% |
| **Overall** | **118** | **142** | **83%** |

## Latency

| Operation | P50 | P95 | P99 |
|-----------|-----|-----|-----|
| Non-streaming TTFT | 120ms | 340ms | 520ms |
| Streaming TTFT | 85ms | 210ms | 380ms |
| Background queue-to-complete | 2.1s | 4.8s | 7.2s |

## Failures

### Tool Calling: parallel_multiple_3
- Expected: `get_weather("SF")` + `get_time("PST")`
- Got: `get_weather("San Francisco, CA")` (location format mismatch)

## Environment
- GPU: NVIDIA A10G (24GB)
- vLLM version: 0.8.x
- RHOAI version: 2.17
- Kubernetes: OpenShift 4.17
```

### D6: Documentation Site Integration

**Decision**: Three publication points.

1. **Landing page card**: "Validated on Real Infrastructure" with latest overall score and link
2. **Antora docs page**: `docs/modules/operations/pages/validation.adoc` with methodology, how to run, and latest results table
3. **GitHub results directory**: `test/cluster/results/` with all historical run files, gitignored except for published runs that are committed

The landing page card pulls data from a JSON file (`test/cluster/results/latest.json`) generated alongside the markdown report.

### D7: Making cc-rosa More Scriptable

**Ideas for the cc-rosa plugin** to support automated test harness workflows:

1. **`/rosa:status --json`**: Output cluster status as JSON (cluster name, ready state, installed components with versions) so the test script can parse it programmatically
2. **`/rosa:install --non-interactive <instill-id> [--param key=value]`**: Skip AskUserQuestion prompts, use provided params or defaults. Enables scripted instill chains.
3. **Instill composition**: A meta-instill that declares "install these instills in order" (e.g., `antwort-rag` depends on `rhoai`, `model`, `postgres`, `qdrant`, `antwort-rag`). The plugin resolves and installs the full chain.
4. **`/rosa:verify <instill-id> --json`**: Structured verification output that the test script can assert against
5. **Instill state file**: Track what's installed in a local state file (`.claude/rosa-state.json`) so the test script knows what's already deployed without querying the cluster

## Scope

### Phase 1: Foundation
- Create `test/cluster/` directory with Go test suite structure
- Shell orchestrator (`run.sh`) with cluster detection and model selection
- Basic inference tests (non-streaming, streaming)
- Result format and report generation
- First instill: `antwort-minimal`

### Phase 2: Feature Coverage
- Tool calling tests (MCP, web search, file search)
- Background mode tests
- RAG pipeline tests
- Conversation chaining tests
- Instills: `postgres`, `qdrant`, `antwort-production`, `antwort-rag`, `antwort-background`

### Phase 3: Benchmarks and Publication
- BFCL subset integration
- Landing page card
- Antora validation page
- Historical results tracking

### Out of Scope
- Automated cluster creation/teardown (too expensive, manual via cc-rosa)
- Running in CI (no GPU runners available)
- Performance benchmarking (focus is correctness, not throughput)

## Dependencies

- cc-rosa plugin with AAET profile (cluster creation)
- ROSA HCP cluster with GPU machinepool (A10G or larger)
- RHOAI installed with vLLM ServingRuntime
- Network access to cluster API and routes

## Resolved Questions

1. **Multi-provider testing**: Yes, test three paths: (a) Antwort via Chat Completions translation (`vllm` provider), (b) Antwort via vLLM Responses API passthrough (`vllm-responses` provider), (c) vLLM Responses API directly (baseline, no gateway). The baseline comparison shows gateway overhead and translation fidelity. Future: add LiteLLM and other runtimes.

2. **BFCL scope**: BFCL has ~4,441 test cases total. Default run uses a fixed subset of ~180 cases (reproducible, comparable across runs). CLI flags: `--bfcl-all` for the full suite, `--bfcl-random N` for random sampling of N cases, `--bfcl-category <name>` for specific categories. The fixed subset is committed to the repo for reproducibility.

3. **Result artifacts**: Each run produces two artifacts: (a) formatted markdown summary (committed to `test/cluster/results/`), (b) raw test output as a companion file (`test/cluster/results/raw/`). Only the markdown summaries are published to the docs site. Raw output is kept in the repo for debugging and reference.
