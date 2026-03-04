# Data Model: E2E Testing with LLM Recording/Replay

**Feature**: 043-e2e-testing
**Date**: 2026-03-04

## Entities

### Recording

A JSON file containing a captured HTTP request/response pair from an LLM backend interaction.

**Fields**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `request.method` | string | yes | HTTP method (POST, GET) |
| `request.path` | string | yes | URL path (e.g., /v1/chat/completions) |
| `request.body` | object | yes | Parsed JSON request body |
| `response.status` | integer | yes | HTTP status code |
| `response.headers` | map | yes | Response headers (Content-Type, etc.) |
| `response.body` | object | conditional | Response body for non-streaming responses |
| `streaming` | boolean | yes | Whether this is a streaming response |
| `chunks` | array of strings | conditional | SSE chunks for streaming responses |
| `metadata.recorded_at` | timestamp | no | When the recording was captured |
| `metadata.source` | string | no | Origin of the recording (e.g., "llama-stack", "antwort") |
| `metadata.test_id` | string | no | Test function that generated this recording |

**File naming**: `{sha256_hash}.json` where hash is computed from `method + path + normalized body`.

**Storage**: `test/e2e/recordings/` directory, committed to the repository.

**Validation rules**:
- Either `response.body` (non-streaming) or `chunks` (streaming) must be present, not both
- `streaming: true` requires `chunks` array with at least one entry
- `streaming: false` requires `response.body` object
- `request.path` must start with `/v1/`

### Replay Backend

The mock-backend binary operating in replay mode. Not a persistent entity; a runtime mode of the existing binary.

**Configuration** (via CLI flags or environment variables):

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `recordings-dir` | string | "" | Path to recordings directory. Empty = use deterministic mock mode. |
| `mode` | string | "replay" | Operating mode: replay, record, record-if-missing |
| `record-target` | string | "" | Backend URL for record mode (required when mode != replay) |

**Behavior by mode**:
- **replay**: Match request hash to recording file, return stored response. Error if no match.
- **record**: Forward request to record-target, save request+response to recording file.
- **record-if-missing**: Check for existing recording first. If found, replay. If not, record.
- **deterministic** (no recordings-dir): Current mock behavior, backward compatible.

### E2E Test Suite

A collection of Go test functions in `test/e2e/` that exercise the deployed antwort stack.

**Configuration** (via environment variables, matching existing SDK test pattern):

| Variable | Default | Description |
|----------|---------|-------------|
| `ANTWORT_BASE_URL` | http://localhost:8080/v1 | Antwort API base URL |
| `ANTWORT_API_KEY` | test | Default API key for unauthenticated tests |
| `ANTWORT_ALICE_KEY` | alice-key | API key for user "alice" (multi-user tests) |
| `ANTWORT_BOB_KEY` | bob-key | API key for user "bob" (multi-user tests) |
| `ANTWORT_MODEL` | mock-model | Model to use in requests |
| `ANTWORT_AUDIT_FILE` | /tmp/audit.log | Path to audit log file (for audit verification tests) |

## Relationships

```text
Recording files --> loaded by --> Replay Backend
Replay Backend --> serves responses to --> Antwort (deployed in kind)
E2E Test Suite --> calls API of --> Antwort (via openai-go SDK)
E2E Test Suite --> reads --> Audit log file (for audit verification)
```

## Directory Structure

```text
test/e2e/
├── e2e_test.go              # Main test file with TestMain and helpers
├── responses_test.go        # Core API tests (US1)
├── auth_test.go             # Authentication + ownership tests (US2)
├── agentic_test.go          # Tool calling tests (US3)
├── audit_test.go            # Audit verification tests (US4)
└── recordings/
    ├── README.md            # Recording format documentation
    ├── chat-completion.json # Basic non-streaming response
    ├── chat-streaming.json  # Streaming response with deltas
    ├── tool-call-turn1.json # First turn: LLM requests tool call
    ├── tool-call-turn2.json # Second turn: LLM responds after tool result
    └── ...
```
