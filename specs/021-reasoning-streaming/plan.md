# Implementation Plan: Reasoning Streaming Events

**Branch**: `021-reasoning-streaming` | **Date**: 2026-02-24 | **Spec**: [spec.md](spec.md)

## Summary

Map existing `ProviderEventReasoningDelta`/`Done` events to OpenResponses SSE event types (`response.reasoning.delta`, `response.reasoning.done`). Add reasoning output items to both streaming and non-streaming responses. The provider layer already parses `reasoning_content` from Chat Completions chunks; this spec adds the engine-level event mapping and output item construction.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Go standard library only
**Storage**: N/A (no new persistence)
**Testing**: Go `testing` package, integration tests via httptest with mock backend producing `reasoning_content`
**Target Platform**: Kubernetes
**Project Type**: Single Go project with layered architecture

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Interface-First | PASS | No new interfaces, extends existing event types |
| II. Zero External Dependencies | PASS | No new deps |
| III. Nil-Safe Composition | PASS | No reasoning = no events (nil-safe) |
| VI. Protocol-Agnostic Provider | PASS | Provider already produces reasoning events |
| VII. Streaming First-Class | PASS | Core purpose of this spec |

No violations.

## Project Structure

```text
pkg/api/
├── events.go              # Add reasoning event type constants + MarshalJSON cases

pkg/engine/
├── events.go              # Map ProviderEventReasoningDelta/Done to SSE events
├── engine.go              # Handle reasoning items in streaming lifecycle
└── loop.go                # Handle reasoning in agentic loop streaming

pkg/provider/openaicompat/
├── response.go            # Ensure reasoning items created in non-streaming path

api/
└── openapi.yaml           # Add reasoning event types to StreamEventType enum

test/integration/
└── streaming_test.go      # Add reasoning streaming tests with mock backend
```
