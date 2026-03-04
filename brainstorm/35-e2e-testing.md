# Brainstorm 35: E2E Testing with LLM Recording/Replay

**Date**: 2026-03-04
**Participants**: Roland Huss
**Goal**: Design comprehensive E2E tests for all antwort features, running in kind on GitHub Actions, using pre-recorded LLM responses instead of hardcoded mock responses.

## Motivation

The current test pyramid has gaps:
- **Unit tests** (pkg/): Good coverage, test individual packages in isolation
- **Integration tests** (test/integration/): In-process httptest servers, growing coverage (23 of 34 code specs)
- **E2E tests**: Minimal. The `kubernetes` CI job deploys to kind but only runs healthz + basic Python SDK tests
- **Conformance tests**: OpenResponses compliance suite runs against mock backend

Missing: tests that exercise the full deployed stack (real K8s networking, real config, real auth middleware chain) with realistic LLM responses (tool calls, streaming, multi-turn conversations).

## Reference: Llama-Stack Recording/Replay

Llama Stack has a mature recording/replay system (`src/llama_stack/testing/api_recorder.py`) with 1,738 JSON recordings. Key properties:

- **Intercept point**: Monkey-patches the outbound HTTP client calls from Llama Stack to downstream LLM backends (Ollama, vLLM)
- **Recording format**: JSON files with request (method, URL, body, endpoint) and response (body, is_streaming). Streaming responses stored as arrays of chunks.
- **Matching**: SHA256 hash of normalized request body. Deterministic for same inputs.
- **Modes**: `replay` (default), `record`, `record-if-missing`, `live`
- **Coverage**: Chat Completions, streaming, tool calls, embeddings, multi-turn conversations, web search tool invocations

Antwort's architecture is analogous: antwort is a server that forwards inference to downstream LLM backends. The recording point is the same boundary (server-to-backend HTTP calls).

## Design Decisions

### D1: Intercept at HTTP transport level

Record/replay the HTTP calls between antwort's providers (vLLM, LiteLLM) and the LLM backend. This is implemented as a "replay backend" that antwort connects to instead of a real LLM.

The replay backend is a Go HTTP server that:
1. Receives Chat Completions or Responses API requests from antwort
2. Hashes the request body (SHA256 of normalized JSON)
3. Looks up the hash in a recordings directory
4. Returns the recorded response (including streaming SSE if applicable)
5. Returns 500 with diagnostic info if no recording matches

### D2: Evolve existing mock-backend

Add recording/replay mode to `cmd/mock-backend`. When started with `--recordings-dir /path/to/recordings`, it operates in replay mode. Without the flag, it uses the current hardcoded responses (backward compatible).

New flags:
- `--recordings-dir DIR`: Path to recordings directory. Enables replay mode.
- `--mode replay|record|record-if-missing`: Control recording behavior (default: replay)
- `--record-target URL`: Backend URL to forward to when recording (required for record modes)

### D3: Request-hash matching (like llama-stack)

Each recording is a JSON file named `{sha256_hash}.json`. The hash is computed from:
- HTTP method
- URL path (without host)
- Normalized request body (sorted keys, stripped whitespace)

Multi-turn agentic scenarios work naturally: each turn's request body includes the conversation history, producing a unique hash.

### D4: Recording format

```json
{
  "request": {
    "method": "POST",
    "path": "/v1/chat/completions",
    "body": {
      "model": "llama3.2:3b-instruct-fp16",
      "messages": [...],
      "stream": false,
      "tools": [...]
    }
  },
  "response": {
    "status": 200,
    "headers": {"Content-Type": "application/json"},
    "body": {...}
  },
  "streaming": false,
  "metadata": {
    "recorded_at": "2026-03-04T15:00:00Z",
    "source": "llama-stack",
    "test_id": "TestE2ECreateResponse"
  }
}
```

For streaming responses:
```json
{
  "request": {...},
  "response": {
    "status": 200,
    "headers": {"Content-Type": "text/event-stream"}
  },
  "streaming": true,
  "chunks": [
    "data: {\"id\":\"chatcmpl-1\",...}\n\n",
    "data: {\"id\":\"chatcmpl-1\",...}\n\n",
    "data: [DONE]\n\n"
  ],
  "metadata": {...}
}
```

### D5: Hybrid recording strategy

- **Reuse llama-stack recordings** for basic inference, streaming, tool calls. Convert from their format (strip `__type__`/`__data__` wrappers, extract raw HTTP data).
- **Record fresh** for antwort-specific scenarios: multi-user auth interactions, audit event generation, conversation chaining, agent profile resolution.
- **Conversion script**: `scripts/convert-llamastack-recordings.go` parses llama-stack JSON and outputs antwort recording format.

### D6: Go tests with openai-go SDK as client

E2E tests use `github.com/openai/openai-go` SDK to call antwort's API. This validates that the API is SDK-compatible while keeping everything in Go.

Tests live in `test/e2e/` (separate from `test/integration/` which uses in-process servers).

### D7: Replay backend as container in kind

The replay backend runs as a separate container (Pod) in the kind cluster, loaded with recording files. Antwort connects to it via K8s Service, exercising real networking.

Deployment:
```
kind cluster:
  replay-backend (Pod) <-- recordings mounted via ConfigMap or emptyDir
  antwort (Pod) --> connects to replay-backend as LLM provider
  e2e-tests (Job) --> calls antwort API via K8s Service
```

### D8: Test scenarios (Phase 1: Core API + Auth + Agentic loop)

Priority scenarios for the first implementation:

| Scenario | Specs Covered | Recording Needs |
|----------|--------------|-----------------|
| Create response (non-streaming) | 001, 002, 003 | 1 recording |
| Create response (streaming) | 007 (streaming) | 1 recording |
| Get/List/Delete response | 005, 028 | Reuses create recording |
| Multi-user auth (API key) | 007 | No LLM recording needed |
| Ownership isolation | 040 | Reuses create recording |
| Scope enforcement | 041 | No LLM recording needed |
| Agentic loop (tool call + response) | 004 | 2 recordings (tool call turn + final turn) |
| Streaming with tool calls | 023 | 2 streaming recordings |
| Audit event verification | 042 | Reuses other recordings |
| Structured output | 029 | 1 recording with response_format |
| Include filtering | 020 | Reuses create recording |
| Reasoning items | 021 | 1 recording with reasoning_content |

Estimated recordings needed: ~10-15 unique recordings for Phase 1.

### D9: CI pipeline changes

Extend the existing `kubernetes` job in `.github/workflows/ci.yml`:

1. Build antwort and replay-backend container images
2. Load into kind
3. Deploy antwort with replay-backend as LLM provider
4. Deploy auth configuration (API keys for multi-user tests)
5. Run E2E test binary as K8s Job
6. Collect results and logs

### D10: Local development support

Developers can run E2E tests locally:
```bash
# Start replay backend locally
go run ./cmd/mock-backend --recordings-dir test/e2e/recordings --port 9090

# Start antwort pointing at replay backend
ANTWORT_BACKEND_URL=http://localhost:9090 go run ./cmd/server

# Run E2E tests
go test ./test/e2e/ -v
```

## Scope

### In Scope (spec candidate)

- Replay mode in `cmd/mock-backend` (request-hash matching, streaming support)
- Recording format specification (JSON, compatible with llama-stack conversion)
- Conversion script for llama-stack recordings
- E2E test suite in `test/e2e/` using openai-go SDK
- K8s deployment manifests for E2E (kustomize overlay)
- CI pipeline updates for kind-based E2E
- Phase 1 test scenarios (Core API, Auth, Agentic loop, Audit)

### Out of Scope

- Record mode (recording against real LLM backends, future addition)
- MCP tool E2E tests (needs MCP test server in cluster, Phase 2)
- File upload E2E (needs file store backend, Phase 2)
- Vector store E2E (needs vector backend in cluster, Phase 2)
- Code interpreter E2E (needs agent-sandbox CRDs, Phase 3)
- Performance/load testing
- Multi-cluster testing

## Resolved Questions

1. **Recordings storage**: Commit to repo in `test/e2e/recordings/`. Small JSON files (~10-50KB each), version-controlled, easy to review.

2. **Protocol scope**: Support both Chat Completions AND Responses API protocol replay from the start. The replay backend handles both protocols based on the request path (`/v1/chat/completions` vs `/v1/responses`).
