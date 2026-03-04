# Research: E2E Testing with LLM Recording/Replay

**Feature**: 043-e2e-testing
**Date**: 2026-03-04

## Decision 1: Replay Backend Architecture

**Decision**: Evolve `cmd/mock-backend` with a `--recordings-dir` flag. When set, the backend matches incoming requests against stored JSON recordings using SHA256 hash of normalized request body. When not set, existing deterministic mock responses are used (backward compatible).

**Rationale**: The current mock-backend is a simple Go HTTP server using `http.ServeMux`. Adding replay mode requires:
1. A request normalizer (sort JSON keys, strip volatile fields like timestamps)
2. A file-based recording store (directory of `{hash}.json` files)
3. An SSE streaming replayer (reads chunks from recording, writes them with proper SSE formatting)

This is a natural extension of the existing binary, not a separate component.

**Alternatives considered**:
- **Separate binary**: Cleaner separation but duplicates mock-backend infrastructure (health endpoint, server setup, container build).
- **VCR-style library**: Go has `gopkg.in/dnaeon/go-vcr.v3` but it intercepts at `http.RoundTripper` level inside the client, not at the server level. We need a server-side replay.
- **WireMock/Mountebank**: External tools, non-Go, add CI complexity.

## Decision 2: Recording Format

**Decision**: Simple JSON format with `request`, `response`, `streaming`, `chunks`, and `metadata` fields. Request hashing uses SHA256 of `method + path + normalized body`.

```json
{
  "request": {
    "method": "POST",
    "path": "/v1/chat/completions",
    "body": { ... }
  },
  "response": {
    "status": 200,
    "headers": { "Content-Type": "application/json" },
    "body": { ... }
  },
  "streaming": false,
  "metadata": {
    "recorded_at": "2026-03-04T15:00:00Z",
    "test_id": "TestE2ECreateResponse"
  }
}
```

For streaming:
```json
{
  "request": { ... },
  "response": {
    "status": 200,
    "headers": { "Content-Type": "text/event-stream" }
  },
  "streaming": true,
  "chunks": [
    "data: {\"id\":\"chatcmpl-1\",...}\n\n",
    "data: [DONE]\n\n"
  ],
  "metadata": { ... }
}
```

**Rationale**: Simpler than llama-stack's format (no `__type__`/`__data__` wrappers, no Pydantic). Pure HTTP-level recording that any language can consume. Conversion from llama-stack format is straightforward: extract `__data__` from response body, reconstruct SSE chunks from streaming arrays.

**Alternatives considered**:
- **Llama-stack format as-is**: Has Python-specific `__type__` annotations. Not Go-idiomatic.
- **HAR format**: Standard but verbose and complex for this use case.
- **Protocol buffers**: Binary format, harder to debug and version-control.

## Decision 3: Request Normalization for Hashing

**Decision**: Normalize requests by:
1. Sorting JSON object keys recursively
2. Removing volatile fields: `stream_options` (varies per test), timestamps
3. Normalizing whitespace (compact JSON)
4. Hashing: SHA256 of `"POST\n/v1/chat/completions\n{compact sorted body}"`

**Rationale**: Deterministic hashing is critical for reliable replay. Llama-stack uses the same approach (SHA256 of normalized request). Sorting keys handles different JSON serialization orders between clients.

**Alternatives considered**:
- **Full URL hashing**: Would break when backend URL changes.
- **Body-only hashing**: Wouldn't distinguish GET vs POST to same path.
- **Sequence-based matching**: Fragile, breaks if test order changes.

## Decision 4: Go OpenAI SDK for E2E Tests

**Decision**: Use `github.com/openai/openai-go` as the test client. Tests live in `test/e2e/` with standard Go test structure.

**Rationale**:
- Validates SDK compatibility (if Go SDK works, Python/TypeScript will too)
- Single language (Go) for the entire test infrastructure
- Compiles to a single binary for deployment as K8s Job
- The SDK supports both Chat Completions and Responses API

**Alternatives considered**:
- **Python openai SDK**: Already exists in test/sdk/python/ but adds Python runtime dependency to E2E.
- **Raw HTTP calls**: Doesn't validate SDK compatibility.
- **Multiple SDKs**: More comprehensive but higher maintenance burden.

## Decision 5: CI Pipeline Extension

**Decision**: Extend the existing `kubernetes` CI job rather than creating a new job. The enhanced job:
1. Builds antwort + mock-backend images (already done)
2. Loads into kind (already done)
3. Deploys with a new kustomize overlay (`quickstarts/01-minimal/e2e/`) that configures auth, audit, and replay
4. Builds and runs the E2E test binary as a K8s Job
5. Collects results

**Rationale**: Reuses existing kind cluster setup infrastructure. The kubernetes job already does most of what's needed. Adding E2E tests is an incremental enhancement.

**Alternatives considered**:
- **New CI job**: Duplicates cluster setup code. More parallelism but more infrastructure.
- **Local-only E2E**: Misses the K8s deployment verification, which is the whole point.

## Decision 6: Llama-Stack Recording Conversion

**Decision**: Build a Go script (`scripts/convert-llamastack-recordings.go`) that:
1. Reads llama-stack JSON recordings
2. Strips `__type__`/`__data__` wrappers from response body
3. Reconstructs SSE chunks from streaming response arrays (wrapping each chunk in `data: {json}\n\n`)
4. Outputs antwort recording format
5. Filters for Chat Completions recordings only (skip Ollama-native format)

**Rationale**: Llama-stack has extensive Chat Completions recordings (inference, streaming, tool calls) that exercise realistic LLM behavior. Converting them gives us instant baseline coverage without needing GPU access.

**Alternatives considered**:
- **Record everything fresh**: Needs GPU access for initial recording. Slower bootstrap.
- **Use llama-stack format directly**: Adds Go deserialization complexity for Python-specific annotations.

## Decision 7: E2E Kustomize Overlay

**Decision**: Create `quickstarts/01-minimal/e2e/` kustomize overlay that:
- References `../ci` as base (inherits mock-backend)
- Patches antwort config for:
  - Auth: API key auth with test keys (alice, bob)
  - Audit: Enabled, JSON format, file output to `/tmp/audit.log`
  - Mock-backend: Points at recordings directory (mounted via ConfigMap or emptyDir with init container)

**Rationale**: Kustomize overlays are the established deployment pattern. The e2e overlay stacks on top of the ci overlay, adding auth and audit configuration.
