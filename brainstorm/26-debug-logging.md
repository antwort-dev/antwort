# Brainstorm 26: Category-Based Debug Logging

## Problem

Antwort uses `log/slog` with basic INFO/WARN/ERROR output. When debugging provider communication, tool execution, MCP calls, or streaming issues, there's no way to see the actual data flowing between components without reading the code.

## Design: Two Orthogonal Dimensions

### Categories (WHAT to debug)

Controlled via `ANTWORT_DEBUG` env var (comma-separated):

| Category | What it logs |
|---|---|
| `all` | Everything |
| `providers` | LLM backend HTTP requests/responses (bodies, headers, timing) |
| `engine` | Agentic loop decisions (turns, tool dispatch, status transitions) |
| `tools` | Tool execution (call arguments, results, timing) |
| `sandbox` | Sandbox server communication (code, stdout, timing) |
| `mcp` | MCP server communication (connect, tools/list, tools/call) |
| `auth` | Auth chain decisions (authenticator votes, identity) |
| `transport` | HTTP request/response handling (method, path, status, headers) |
| `streaming` | SSE event emission (event type, sequence number) |
| `config` | Configuration loading and hot-reload |

### Levels (HOW MUCH detail)

Controlled via `ANTWORT_LOG_LEVEL` env var:

| Level | Value | What shows |
|---|---|---|
| `ERROR` | slog.LevelError | Only errors |
| `WARN` | slog.LevelWarn | Warnings and above (default) |
| `INFO` | slog.LevelInfo | Startup, config, request summaries |
| `DEBUG` | slog.LevelDebug | Category debug output (when category enabled) |
| `TRACE` | slog.LevelDebug-4 | Full HTTP bodies, headers, raw data |

Category output is emitted at DEBUG level. Setting `ANTWORT_LOG_LEVEL=DEBUG` is required to see debug category output. TRACE adds untruncated bodies and headers.

### Configuration

```yaml
logging:
  level: DEBUG
  debug: providers,tools
  format: text  # or "json"
```

Or via environment:
```
ANTWORT_LOG_LEVEL=DEBUG
ANTWORT_DEBUG=providers,tools
```

### Implementation

A `pkg/debug` package with:
- `Enabled(category string) bool` - fast check (map lookup)
- `Log(category, msg string, args ...any)` - emits at DEBUG level if category enabled
- Zero-cost when disabled (no string formatting, no allocations)

### What "providers" Category Logs

```
DEBUG providers: POST http://vllm:8080/v1/chat/completions
DEBUG providers: request: model=/mnt/models messages=3 tools=1 stream=false
TRACE providers: request body: {"model":"/mnt/models","messages":[...]...}
DEBUG providers: response: 200 OK (1.234s) usage={in:100,out:50}
TRACE providers: response body: {"id":"chatcmpl-xxx","choices":[...]...}
```

At DEBUG: method, URL, summary fields, status, timing.
At TRACE: full request/response bodies (truncated at 10KB by default).

## Phasing

1. `pkg/debug` package with Enabled/Log
2. Configure slog handler with level from env/config
3. Instrument provider layer (providers category)
4. Instrument engine (engine category)
5. Instrument sandbox client (sandbox category)
6. Instrument MCP executor (mcp category)
7. Instrument SSE writer (streaming category)
8. Instrument auth middleware (auth category)
9. Instrument HTTP adapter (transport category)

Provider instrumentation (phase 3) is the highest value, the rest can follow incrementally.
