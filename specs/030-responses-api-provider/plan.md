# Implementation Plan: Responses API Provider

**Branch**: `030-responses-api-provider` | **Date**: 2026-02-26 | **Spec**: [spec.md](spec.md)

## Summary

Add a provider adapter that forwards inference requests to backends using the Responses API wire format (`/v1/responses`) instead of Chat Completions. Antwort continues to own the agentic loop, state management, and tool execution. The new provider yields native SSE events and eliminates custom event synthesis logic. As part of this work, move the `expandBuiltinTools` logic from the engine to a shared provider utility used by both provider types.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Go standard library only for the new provider (consistent with existing providers). No new external dependencies.
**Storage**: N/A (provider does not manage state)
**Testing**: Go `testing` package. Real vLLM backend for integration tests. Mock HTTP server for unit tests (protocol-level testing per constitution).
**Target Platform**: Any backend supporting the Responses API (vLLM v0.10.0+, SGLang, Ollama)
**Project Type**: Single Go project with layered architecture

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Interface-First | PASS | Implements existing Provider interface, no changes |
| II. Zero External Dependencies | PASS | Go stdlib only, same as existing providers |
| III. Nil-Safe | PASS | Optional provider, not registered if not configured |
| V. Validate Early | PASS | Startup validation of backend Responses API support (FR-013) |
| VI. Protocol-Agnostic | **RESOLVES** | Moves built-in tool expansion from engine to provider layer |
| VII. Streaming First-Class | PASS | Native SSE event consumption |
| Testing: Real over fakes | PASS | Real vLLM for integration, mock HTTP for protocol tests |

## Project Structure

### Source Code

```text
pkg/provider/responses/
├── provider.go                  # ResponsesProvider implementing Provider interface (NEW)
├── translate.go                 # Translate ProviderRequest/Response to/from Responses API wire format (NEW)
├── translate_test.go            # Translation unit tests (NEW)
├── stream.go                    # SSE event parser and mapping to ProviderEvent (NEW)
├── stream_test.go               # Streaming unit tests (NEW)
└── types.go                     # Responses API wire format types (NEW)

pkg/provider/
├── tools.go                     # Shared built-in tool expansion utility (NEW, moved from engine)

pkg/engine/
├── engine.go                    # MODIFY: remove expandBuiltinTools, preserve tool types

cmd/server/
├── main.go                      # MODIFY: add "vllm-responses" provider case
```

## Design Decisions

### D1: Provider Package Location

New package `pkg/provider/responses/` alongside the existing `vllm/` and `litellm/` packages. Does not share the `openaicompat/` base since the wire format is different (Responses API, not Chat Completions).

### D2: Request Translation

`ProviderRequest` maps almost 1:1 to the Responses API request format. Key differences:
- `ProviderRequest.Messages` become the `input` field (array of items)
- Always sends `store: false` (antwort manages state)
- Built-in tool types expanded to function definitions via shared utility (same as Chat Completions)
- `ProviderRequest.Model`, `Stream`, `MaxTokens` map directly

### D3: SSE Event Mapping

The Responses API produces events like `response.output_text.delta` and `response.function_call_arguments.delta`. These map to `ProviderEvent` types (which were already modeled after the Responses API). The mapping is near-identity, much simpler than the Chat Completions delta chunk synthesis.

### D4: expandBuiltinTools Migration

Phase 1 moves `expandBuiltinTools` from `engine.go` to a shared utility in `pkg/provider/tools.go`. Both provider types use this utility to expand built-in tool types (`code_interpreter`, `file_search`, `web_search_preview`) to function definitions before forwarding to the backend. This prevents backends (particularly vLLM) from attempting their own built-in tool execution via MCP servers. The engine stops modifying tool types, restoring Constitution Principle VI compliance.

### D5: Backend Validation

At startup, the provider probes the backend's `/v1/responses` endpoint with a lightweight request to verify it's available. If the endpoint returns 404 or an error, startup fails with a clear message.
