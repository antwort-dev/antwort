# Implementation Plan: Terminal Streaming Events

**Branch**: `022-terminal-events` | **Date**: 2026-02-24 | **Spec**: [spec.md](spec.md)

## Summary

Add `response.incomplete` terminal event for max-tokens truncation (distinct from `response.completed`), `error` stream event for pre-response errors, and `response.refusal.delta/done` for content policy refusals. Main change: detect `finish_reason: "length"` and emit `response.incomplete` instead of `response.completed`.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Go standard library only
**Testing**: Go `testing` package, integration tests via httptest
**Project Type**: Single Go project with layered architecture

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Interface-First | PASS | No new interfaces |
| II. Zero External Dependencies | PASS | No new deps |
| III. Nil-Safe | PASS | Missing fields = default behavior |
| IV. Typed Error Domain | PASS | Error event uses existing APIError type |
| VII. Streaming First-Class | PASS | Core purpose |

No violations.

## Project Structure

```text
pkg/api/
├── events.go              # Add 4 new event type constants + MarshalJSON
├── types.go               # Ensure IncompleteDetails populated correctly

pkg/engine/
├── engine.go              # Use response.incomplete for finish_reason=length
├── events.go              # Add refusal event mapping (future use)

pkg/provider/openaicompat/
├── response.go            # Map finish_reason "length" to incomplete status

api/
└── openapi.yaml           # Add new event types

test/integration/
├── streaming_test.go      # Test incomplete detection
├── helpers_test.go        # Mock backend with finish_reason=length
```
