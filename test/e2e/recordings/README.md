# E2E Test Recordings

## Format

Each recording is a JSON file with the following structure:

### Non-streaming

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
    "recorded_at": "2026-03-04T00:00:00Z",
    "test_id": "TestName"
  }
}
```

### Streaming

```json
{
  "request": {
    "method": "POST",
    "path": "/v1/chat/completions",
    "body": { ... }
  },
  "response": {
    "status": 200,
    "headers": { "Content-Type": "text/event-stream" }
  },
  "streaming": true,
  "chunks": [
    "data: {\"id\":\"...\", ...}\n\n",
    "data: [DONE]\n\n"
  ],
  "metadata": {
    "recorded_at": "2026-03-04T00:00:00Z",
    "test_id": "TestName"
  }
}
```

## File naming

Files are named `{sha256_hash}.json` where `hash = SHA256(method + "\n" + path + "\n" + normalized_body)`.

Human-readable names (like `chat-basic.json`) are also supported for hand-crafted recordings.

## Creating recordings

1. **Handcraft**: Copy response format from `cmd/mock-backend` output
2. **Record**: Run mock-backend with `--mode record --record-target <url> --recordings-dir <dir>`
3. **Convert**: Run `scripts/convert-llamastack-recordings.go`
