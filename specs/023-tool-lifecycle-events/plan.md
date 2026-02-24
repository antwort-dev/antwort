# Implementation Plan: Tool Lifecycle SSE Events

**Branch**: `023-tool-lifecycle-events` | **Date**: 2026-02-24 | **Spec**: [spec.md](spec.md)

## Summary

Emit tool-specific lifecycle SSE events (in_progress, searching, completed, failed) around tool execution in the streaming agentic loop. The engine wraps each tool executor call with before/after events based on tool type (MCP, file_search, web_search). No ToolExecutor interface changes needed.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Go standard library only
**Testing**: Go `testing` package, integration tests via httptest
**Project Type**: Single Go project with layered architecture

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Interface-First | PASS | No interface changes; events emitted at engine level |
| II. Zero External Dependencies | PASS | No new deps |
| III. Nil-Safe | PASS | No tools = no events |
| VII. Streaming First-Class | PASS | Core purpose |

No violations.

## Project Structure

```text
pkg/api/
├── events.go              # Add 9 tool lifecycle event type constants + MarshalJSON

pkg/engine/
├── loop.go                # Emit lifecycle events around tool execution in streaming loop

api/
└── openapi.yaml           # Add new event types to StreamEventType enum

test/integration/
├── streaming_test.go      # Test tool lifecycle events with mock tool executor
├── helpers_test.go        # Add tool executor to test environment
```
