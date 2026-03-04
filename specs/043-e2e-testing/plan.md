# Implementation Plan: E2E Testing with LLM Recording/Replay

**Branch**: `043-e2e-testing` | **Date**: 2026-03-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/043-e2e-testing/spec.md`

## Summary

Add comprehensive E2E tests that exercise the full deployed antwort stack in a kind cluster using pre-recorded LLM responses. The existing mock-backend is evolved with a replay mode that matches incoming requests to stored JSON recordings via SHA256 hashing. E2E tests use the Go OpenAI SDK (`openai-go`) as the client, validating SDK compatibility while covering core API, authentication, agentic loop, and audit logging. A conversion script enables reusing existing LLM recordings from the llama-stack project.

## Technical Context

**Language/Version**: Go 1.22+ (consistent with all existing specs)
**Primary Dependencies**: Go standard library + `github.com/openai/openai-go` (test dependency only, not in core packages)
**Storage**: N/A (recordings are static JSON files, not a database)
**Testing**: Go standard `testing` package with openai-go SDK as client
**Target Platform**: GitHub Actions free-tier runners, kind cluster
**Project Type**: Test infrastructure (not user-facing service code)
**Performance Goals**: Full E2E run under 10 minutes including cluster setup
**Constraints**: Must run on GitHub Actions free-tier (2 cores, 7GB RAM, no GPU). Must not break existing CI jobs.
**Scale/Scope**: ~15 test scenarios, ~15 recording files, 3 new test files, 1 evolved binary

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | N/A | Test infrastructure, no new interfaces in core |
| II. Zero External Dependencies | PASS | `openai-go` is test-only, not in core packages. Recording/replay logic in `cmd/mock-backend` uses stdlib only. |
| III. Nil-Safe Composition | N/A | Test infrastructure |
| IV. Typed Error Domain | N/A | Test infrastructure |
| V. Validate Early | PASS | Replay backend validates recording files on load, returns diagnostic errors on missing recordings |
| IX. Kubernetes-Native | PASS | E2E tests run in kind, exercising real K8s deployment |
| Testing Standards | PASS | This IS the testing feature. Fulfills constitution v1.7.0 integration test mandate. |
| Documentation | PASS | No user-facing API changes. README already updated. Internal docs via recordings/README.md. |

No violations.

## Project Structure

### Documentation (this feature)

```text
specs/043-e2e-testing/
├── spec.md
├── plan.md              # This file
├── research.md
├── data-model.md
├── checklists/
│   └── requirements.md
└── tasks.md             # Via /speckit.tasks
```

### Source Code (repository root)

```text
cmd/mock-backend/
└── main.go              # Evolve with replay mode (--recordings-dir, --mode flags)

test/e2e/
├── e2e_test.go          # TestMain, helpers, openai-go client setup
├── responses_test.go    # Core API tests (create, stream, get, list, delete)
├── auth_test.go         # API key auth, ownership isolation
├── agentic_test.go      # Tool calling with replay
├── audit_test.go        # Audit event verification
└── recordings/
    ├── README.md
    ├── chat-basic.json           # Non-streaming chat completion
    ├── chat-streaming.json       # Streaming chat completion
    ├── chat-tool-call.json       # Turn 1: LLM requests tool call
    ├── chat-tool-result.json     # Turn 2: LLM final response after tool
    ├── chat-streaming-tool.json  # Streaming tool call turn 1
    ├── chat-streaming-result.json # Streaming tool result turn 2
    ├── chat-reasoning.json       # Response with reasoning_content
    ├── chat-structured.json      # Response with response_format
    └── responses-api-basic.json  # Responses API protocol recording

scripts/
└── convert-llamastack-recordings.go  # Conversion from llama-stack format

quickstarts/01-minimal/e2e/
├── kustomization.yaml   # E2E overlay (references ../ci)
├── auth-config.yaml     # API key auth configuration
├── audit-config.yaml    # Audit logging configuration
└── recordings-cm.yaml   # ConfigMap with recording files (or init container)

.github/workflows/
└── ci.yml               # Extended kubernetes job with E2E step
```

**Structure Decision**: E2E tests in `test/e2e/` (separate from `test/integration/`). Recordings committed to repo as static JSON files. Mock-backend evolved in-place.

## Design Decisions

### D1: Replay Mode in Mock Backend

Add three flags to `cmd/mock-backend`:
- `--recordings-dir`: Path to directory containing recording JSON files. Empty = deterministic mock mode.
- `--mode`: `replay` (default), `record`, `record-if-missing`
- `--record-target`: Backend URL for record/record-if-missing modes

When `--recordings-dir` is set, the mock backend:
1. Loads all `*.json` files from the directory on startup
2. Builds an in-memory index: `map[string]*Recording` keyed by SHA256 hash
3. For each incoming request, computes the hash and looks up the recording
4. Returns the recorded response (streaming or non-streaming)
5. Returns 500 with diagnostic info if no recording matches

### D2: Request Hash Computation

```
hash = SHA256(method + "\n" + path + "\n" + normalizeJSON(body))
```

Normalization:
- Parse body as JSON
- Sort all object keys recursively
- Remove `stream_options` field (varies per request, doesn't affect response content)
- Re-serialize as compact JSON (no whitespace)

### D3: Streaming Replay

For streaming recordings, the mock backend:
1. Sets `Content-Type: text/event-stream` and related headers
2. Writes each chunk from the `chunks` array with a small delay (1ms) between chunks
3. Flushes after each chunk
4. Closes the connection after the last chunk

The delay prevents the client from receiving all chunks as a single read, which would mask streaming bugs.

### D4: E2E Test Architecture

```text
Kind Cluster:
  ┌─────────────────┐     ┌──────────────────┐
  │  antwort Pod     │────>│ replay-backend   │
  │  (auth, audit,   │     │ (loaded with     │
  │   engine, etc.)  │     │  recordings)     │
  └─────────────────┘     └──────────────────┘
         ↑
         │ HTTP (via K8s Service)
         │
  ┌─────────────────┐
  │  E2E test Job   │
  │  (openai-go     │
  │   SDK client)   │
  └─────────────────┘
```

Tests use environment variables for configuration (ANTWORT_BASE_URL, API keys, model).

### D5: E2E Kustomize Overlay

The `quickstarts/01-minimal/e2e/` overlay extends the `ci` overlay:
- Patches antwort ConfigMap with auth config (API keys for alice/bob)
- Patches antwort ConfigMap with audit config (enabled, JSON, file output)
- Mounts recordings into the replay-backend Pod (via ConfigMap generated from recording files)
- Adds `--recordings-dir /recordings` flag to the mock-backend command

### D6: CI Pipeline Enhancement

The `kubernetes` CI job gains new steps after the existing healthz check:
1. Build E2E test binary: `go test -c -o bin/e2e-tests ./test/e2e/`
2. Copy test binary into kind: `docker cp bin/e2e-tests ci-control-plane:/tmp/`
3. Run tests via kubectl exec or as a Job with appropriate env vars
4. Collect results and audit log for verification

Alternative: Run E2E tests from outside the cluster using `kubectl port-forward`. Simpler, no need to copy binary into the cluster. This is the approach used by the existing Python SDK tests.

### D7: Llama-Stack Recording Conversion

The conversion script reads llama-stack JSON, extracts:
- `request.body` as-is (already standard Chat Completions format)
- `request.method` and `request.endpoint` for path
- `response.body.__data__` (unwrap `__type__`/`__data__` wrapper)
- For streaming: reconstruct SSE chunks from the array of `__data__` objects

Filters:
- Only convert recordings with OpenAI Chat Completions format (skip Ollama-native)
- Skip recordings with empty or error responses
- Deduplicate by request hash
