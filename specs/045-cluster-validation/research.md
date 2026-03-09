# Research: Real-Cluster Validation Harness

**Date**: 2026-03-09
**Feature**: 045-cluster-validation

## R1: BFCL Dataset Format and Evaluation

### Decision: Use BFCL v4 JSONL files with AST evaluation

### Dataset Location
- HuggingFace: `gorilla-llm/Berkeley-Function-Calling-Leaderboard`
- GitHub source: `ShishirPatil/gorilla/berkeley-function-call-leaderboard/bfcl_eval/data/`
- Do NOT use HuggingFace `load_dataset()` (schema mismatches). Load JSONL files directly.

### Test Case Format (JSONL)
```json
{
  "id": "simple_python_0",
  "question": [[{"role": "user", "content": "Find the area..."}]],
  "function": [{
    "name": "calculate_triangle_area",
    "parameters": {
      "type": "dict",
      "properties": {"base": {"type": "integer"}, "height": {"type": "integer"}},
      "required": ["base", "height"]
    }
  }]
}
```

Ground truth (separate files in `possible_answer/`):
```json
{
  "id": "simple_python_0",
  "ground_truth": [{"calculate_triangle_area": {"base": [10], "height": [5]}}]
}
```

Each argument value is a list of acceptable values. Empty string means optional.

### Gorilla-to-OpenAPI Type Mapping
- `"dict"` -> `"object"`, `"float"` -> `"number"`, `"tuple"` -> `"array"`, `"any"` -> `"string"`
- Dots in function names replaced with underscores (e.g., `math.factorial` -> `math_factorial`)
- `parameters.type` always set to `"object"`

### AST Evaluation Logic
1. **simple**: Exact 1 function call, name match, required params present, values in acceptable list
2. **parallel**: Multiple calls, order-independent matching via `simple` checker per call
3. **multiple**: 1 call from multiple function options, uses `simple` checker
4. **irrelevance**: Model should NOT produce any function calls

String comparison: case-insensitive, punctuation/spaces stripped (`standardize_string()`).

### Category Counts (for fixed subset selection)

| Category | File Prefix | Cases |
|----------|-------------|-------|
| simple_python | BFCL_v4_simple_python | 399 |
| simple_java | BFCL_v4_simple_java | 99 |
| simple_javascript | BFCL_v4_simple_javascript | 49 |
| multiple | BFCL_v4_multiple | 199 |
| parallel | BFCL_v4_parallel | 199 |
| parallel_multiple | BFCL_v4_parallel_multiple | 199 |
| irrelevance | BFCL_v4_irrelevance | 239 |
| **Total (non-live single-turn)** | | **~1,384** |

Live, multi-turn, and agentic categories add ~3,300 more cases.

### Fixed Subset (~180 cases)
Selected for reproducibility and category coverage:

| Category | Cases | Selection |
|----------|-------|-----------|
| simple_python | 50 | First 50 by ID |
| multiple | 50 | First 50 by ID |
| parallel | 30 | First 30 by ID |
| parallel_multiple | 20 | First 20 by ID |
| irrelevance | 30 | First 30 by ID |
| **Total** | **180** | |

### Rationale
- Non-live categories only (deterministic, no external API calls)
- Python-only for simple (largest category, most representative)
- Covers all core evaluation patterns (AST simple, parallel, multiple, irrelevance)
- ~180 cases runnable in ~10 minutes with fast backend
- Full suite available via `--bfcl-all` flag

### Alternatives Considered
- Random sampling: rejected (not reproducible across runs)
- Live categories: rejected (require external API mocking)
- Multi-turn: deferred to Phase 2 (requires conversation state management)

---

## R2: Multi-Provider Test Architecture

### Decision: Single Antwort deployment + direct HTTP clients for comparison

### Three Provider Paths

1. **Antwort + Chat Completions** (`vllm` provider): Tests send requests to Antwort's `/v1/responses` endpoint. Antwort translates to Chat Completions and forwards to vLLM's `/v1/chat/completions`.

2. **Antwort + Responses API** (`vllm-responses` provider): Tests send requests to Antwort's `/v1/responses` endpoint. Antwort forwards using native Responses API to vLLM's `/v1/responses`.

3. **Direct vLLM baseline**: Tests send requests directly to vLLM's `/v1/responses` endpoint using the openai-go SDK, bypassing Antwort entirely.

### Implementation Approach
- Environment variables configure each path's endpoint URL:
  - `CLUSTER_ANTWORT_URL` (default Antwort endpoint, used by paths 1 and 2)
  - `CLUSTER_VLLM_URL` (direct vLLM endpoint, used by path 3)
- Tests create separate HTTP clients per path
- The Antwort deployment's provider type determines which translation path is exercised
- To compare both Antwort paths, run the suite twice with different Antwort configs (or deploy two instances)

### Rationale
- Simplest approach: one Antwort deployment at a time, tests just hit different URLs
- No need for complex multi-deployment orchestration in the test code
- `run.sh` can optionally run the suite twice (once per provider config)

### Alternatives Considered
- Two simultaneous Antwort deployments: rejected (complex, resource-heavy)
- In-process Antwort with swappable provider: rejected (doesn't test real K8s deployment)

---

## R3: vLLM Responses API Compatibility

### Decision: Support both Chat Completions and Responses API paths, document limitations

### Key Findings
- vLLM serves `/v1/responses` natively since v0.10.0 (July 2025)
- Multi-turn limitations: vLLM rejects typed input items from prior turns (Issue #33089)
- Tool calls returned as `function_call` output items with `name` and `arguments` fields
- Built-in tools (code_interpreter, file_search) are expanded to function definitions by Antwort before forwarding

### Antwort Provider Implementation
- `pkg/provider/responses/provider.go`: Direct HTTP to `/v1/responses` (no SDK, stdlib only per constitution)
- `pkg/provider/openaicompat/client.go`: Chat Completions translation with event synthesis
- Provider selection via `engine.provider` config: `vllm`, `vllm-responses`, or `litellm`

### Impact on Validation
- Basic inference: works on both paths
- Tool calling: works on both paths (function_call items)
- Multi-turn via previous_response_id: works (Antwort manages state, sends single-turn to backend)
- Streaming: both paths support SSE

---

## R4: Test Infrastructure Patterns

### Decision: Follow existing E2E patterns with cluster-specific extensions

### Environment Variables (from existing E2E tests)
Pattern: `envOr("ANTWORT_X", "default")` in TestMain

For cluster tests:
- `CLUSTER_ANTWORT_URL` - Antwort route URL (required, no default)
- `CLUSTER_VLLM_URL` - Direct vLLM URL (optional, enables baseline comparison)
- `CLUSTER_API_KEY` - API key for authenticated requests
- `CLUSTER_MODEL` - Model name (required)
- `CLUSTER_TIMEOUT` - Per-test timeout (default: 120s)

### Client Creation
- Use `openai-go` SDK for clean Responses API calls (already a test dependency)
- Raw `net/http` for direct vLLM baseline (consistent with provider pattern)
- Factory functions: `newAntwortClient()`, `newVLLMClient()`

### Test Organization
- `//go:build cluster` build tag
- `TestMain` with cluster reachability check (skip if unreachable)
- Separate files per test category matching E2E pattern
- Table-driven subtests for BFCL cases

### Result Collection
- `TestMain` collects results via a global `ResultCollector` (thread-safe)
- Each test registers pass/fail, latency, error details
- `TestMain` teardown generates markdown report and JSON summary
- `run.sh` orchestrates: check cluster, run tests, update symlink

---

## R5: Report Generation Architecture

### Decision: Go TestMain generates JSON, run.sh generates markdown

### Separation of Concerns
- **Go tests**: Collect structured results in `ResultCollector`, write `results.json` on teardown
- **run.sh**: Reads `results.json`, generates timestamped markdown report, updates `latest.md` symlink, optionally writes `latest.json` for docs site

### Rationale
- Go test output format is constrained (stdout interleaved with test framework output)
- Markdown generation is easier in shell (heredoc templates)
- JSON intermediate format enables future tooling (dashboards, trend analysis)

### Alternatives Considered
- Pure Go report generation in TestMain: rejected (markdown templating verbose in Go, shell is more natural)
- External post-processing tool: rejected (over-engineering for this scope)
