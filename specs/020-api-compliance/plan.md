# Implementation Plan: OpenResponses API Compliance

**Branch**: `020-api-compliance` | **Date**: 2026-02-23 | **Spec**: [spec.md](spec.md)

## Summary

Add 11 missing OpenResponses request fields across 3 layers (API types, engine translation, provider passthrough), implement `parallel_tool_calls` sequential mode and `max_tool_calls` limit in the agentic loop, add `include` response filtering and `stream_options` usage control, and update the OpenAPI spec and integration tests.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Go standard library only (consistent with constitution)
**Storage**: N/A (no new persistence, fields echo through existing request/response flow)
**Testing**: Go `testing` package, table-driven tests, integration tests via httptest
**Target Platform**: Kubernetes (container-based deployment)
**Project Type**: Single Go project with layered architecture

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Interface-First | PASS | No new interfaces. Fields flow through existing `Provider` interface via `ProviderRequest.Extra` for provider-specific params |
| II. Zero External Dependencies | PASS | No new deps. Pure struct field additions |
| III. Nil-Safe Composition | PASS | All new fields use pointer types or omitempty. Missing = use default |
| IV. Typed Error Domain | PASS | No new error types. `max_tool_calls` violation uses existing error patterns |
| V. Validate Early | PASS | `max_tool_calls` validated in engine before loop starts |
| VI. Protocol-Agnostic Provider | PASS | Provider-forwarded fields go through `ProviderRequest` (model-agnostic), translated to Chat Completions in the openaicompat layer |
| VII. Streaming First-Class | PASS | `stream_options` integrates with existing SSE writer |
| VIII. Context Carries Data | N/A | No new cross-cutting concerns |

No violations. No complexity tracking needed.

## Project Structure

```text
pkg/api/
├── types.go                # Add fields to CreateResponseRequest + Response

pkg/engine/
├── translate.go            # Map new request fields to ProviderRequest
├── engine.go               # Build Response with new fields, enforce max_tool_calls
└── loop.go                 # Respect parallel_tool_calls=false (sequential dispatch)

pkg/provider/
├── types.go                # Add fields to ProviderRequest

pkg/provider/openaicompat/
├── types.go                # Add fields to ChatCompletionRequest
└── translate.go            # Map new ProviderRequest fields to Chat Completions

pkg/transport/http/
├── adapter.go              # Apply include filtering before response serialization

api/
└── openapi.yaml            # Add new request/response fields to spec

test/integration/
└── responses_test.go       # Add passthrough field round-trip tests
```
