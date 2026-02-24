# Brainstorm 18: Refusal, Error, and Incomplete Streaming Events

## Problem

When a model refuses to answer (content policy, safety filter) or the stream encounters an error, the upstream spec defines specific event types. Antwort currently handles these as generic `response.failed` events without the finer-grained refusal streaming that lets clients show "the model declined to answer" progressively.

## Missing SSE Events (4)

- `response.refusal.delta` - incremental refusal text
- `response.refusal.done` - refusal text complete
- `response.incomplete` - response ended incomplete (distinct from failed)
- `error` - stream-level error event

## Current State

- `pkg/engine/engine.go` emits `response.failed` for errors and `response.cancelled` for cancellations
- The engine maps provider `finish_reason: "content_filter"` to `ResponseStatusFailed` but doesn't produce refusal content
- `response.incomplete` is a separate status from `response.failed` (the model stopped due to max tokens, not an error), but currently both map to `response.completed` or `response.failed`
- There's no `error` event type (distinct from `response.failed`, which wraps the error in a response object)

## What's Needed

1. Add 4 new `StreamEventType` constants
2. Detect refusal content from the provider (some backends return a `refusal` field in the Chat Completions response)
3. Distinguish `response.incomplete` (max tokens reached) from `response.failed` (actual error) in the streaming terminal event
4. Add `error` event for stream-level errors that don't produce a response object

## Complexity

Low. Mostly event type additions and conditional logic in the existing terminal event emission.

## Questions

- Do any Chat Completions backends currently return a `refusal` field? OpenAI does for GPT-4o and later, but vLLM may not support this yet.
- Should refusal be streamed token-by-token (matching the delta/done pattern) or emitted as a single event?
