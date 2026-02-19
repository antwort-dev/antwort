# Implementation Plan: LiteLLM Provider Adapter

**Branch**: `008-provider-litellm` | **Date**: 2026-02-19 | **Spec**: [spec.md](spec.md)

## Summary

Extract shared OpenAI Chat Completions translation logic from vLLM adapter into a reusable base package. Build LiteLLM adapter as a thin layer on top with model mapping and extension support. Refactor vLLM to use the shared base. Add provider selection to the server binary.

## Technical Context

**Language/Version**: Go 1.22+ (consistent with Specs 001-007)
**Primary Dependencies**: None new. Shared base uses existing types from pkg/api and pkg/provider.
**Testing**: `go test` with mock HTTP servers (httptest). Existing vLLM tests validate refactoring.
**Constraints**: Zero regressions on vLLM tests. No new external dependencies.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Interface-First | PASS | Reuses Provider interface |
| II. Zero Dependencies | PASS | No new external deps |
| III. Nil-Safe | PASS | Model mapping nil = passthrough |
| VI. Protocol-Agnostic | PASS | Shared base handles protocol |

## Project Structure

```text
pkg/
├── provider/
│   ├── openaicompat/
│   │   ├── client.go             # Shared HTTP client (Complete, Stream, ListModels)
│   │   ├── translate.go          # Request/response translation (extracted from vLLM)
│   │   ├── stream.go             # SSE parser (extracted from vLLM)
│   │   ├── response.go           # Response translation (extracted from vLLM)
│   │   ├── errors.go             # Error mapping (extracted from vLLM)
│   │   ├── types.go              # Chat Completions types (extracted from vLLM)
│   │   └── client_test.go        # Shared base tests
│   │
│   ├── vllm/
│   │   ├── vllm.go               # REFACTORED: embed openaicompat.Client
│   │   ├── config.go             # Unchanged
│   │   └── vllm_test.go          # MUST PASS unchanged
│   │
│   └── litellm/
│       ├── litellm.go            # LiteLLM adapter (thin wrapper)
│       ├── config.go             # LiteLLM config + model mapping
│       ├── extensions.go         # LiteLLM-specific extension handling
│       └── litellm_test.go       # LiteLLM tests

cmd/
└── server/
    └── main.go                   # MODIFIED: ANTWORT_PROVIDER env var
```
