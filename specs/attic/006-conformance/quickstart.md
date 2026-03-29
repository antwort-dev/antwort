# Quickstart: OpenResponses Conformance Testing

**Feature**: 006-conformance
**Date**: 2026-02-18

## Run Conformance Tests (One Command)

```bash
# Run with core profile (5/6 tests: basic, streaming, system prompt, tools, multi-turn)
make conformance PROFILE=core

# Run with extended profile (all 6 tests, including image input)
make conformance PROFILE=extended

# Default profile (core)
make conformance
```

## Manual Test Drive

```bash
# Terminal 1: Start the mock Chat Completions backend
go run ./cmd/mock-backend

# Terminal 2: Start antwort pointing at the mock
ANTWORT_BACKEND_URL=http://localhost:9090 \
ANTWORT_MODEL=mock-model \
ANTWORT_PORT=8080 \
go run ./cmd/server

# Terminal 3: Send a test request
curl -X POST http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mock-model",
    "input": [{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "Hello"}]}]
  }'
```

## Run Official Compliance Suite Manually

```bash
# Build the compliance runner container
podman build -t antwort-compliance -f conformance/Containerfile .

# Run against a running antwort instance
podman run --rm --network=host \
  -e BASE_URL=http://localhost:8080/v1 \
  -e MODEL=mock-model \
  antwort-compliance
```

## Understanding Results

```json
{
  "profile": "core",
  "score": "5/5",
  "tests": [
    {"name": "basic-response", "status": "passed", "duration_ms": 120},
    {"name": "streaming-response", "status": "passed", "duration_ms": 340},
    {"name": "system-prompt", "status": "passed", "duration_ms": 95},
    {"name": "tool-calling", "status": "passed", "duration_ms": 150},
    {"name": "image-input", "status": "skipped", "reason": "not in core profile"},
    {"name": "multi-turn", "status": "passed", "duration_ms": 110}
  ]
}
```
