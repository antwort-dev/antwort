# Brainstorm 16: Reasoning Streaming Events

## Problem

Models like DeepSeek R1, Qwen QwQ, and OpenAI o-series produce reasoning tokens (chain-of-thought) before generating the final answer. Antwort already handles reasoning in the non-streaming path (creating `reasoning` items from `reasoning_content` in the Chat Completions response), but the streaming path doesn't emit reasoning-specific SSE events. Clients can't see the model "thinking" in real-time.

## Missing SSE Events (6)

- `response.reasoning.delta` - incremental reasoning token
- `response.reasoning.done` - reasoning complete
- `response.reasoning_summary.delta` - incremental reasoning summary
- `response.reasoning_summary.done` - reasoning summary complete
- `response.reasoning_summary_part.added` - new summary part started
- `response.reasoning_summary_part.done` - summary part finished

## Current State

- `pkg/provider/openaicompat/stream.go` already parses `reasoning_content` from streaming chunks
- `pkg/api/events.go` has `ProviderEventReasoningDelta` and `ProviderEventReasoningDone` event types
- `pkg/engine/events.go` maps provider events to stream events, but doesn't emit the reasoning-specific SSE event types
- The engine currently creates a `reasoning` Item but doesn't stream the reasoning tokens to the client

## What's Needed

1. Add 6 new `StreamEventType` constants to `pkg/api/events.go`
2. Add serialization cases in `StreamEvent.MarshalJSON`
3. Map `ProviderEventReasoningDelta` / `ProviderEventReasoningDone` to the new SSE events in `pkg/engine/events.go`
4. Handle reasoning summary (may need provider support for summary extraction)
5. Integration test with a mock backend that produces `reasoning_content` in streaming chunks

## Complexity

Low. The provider-level plumbing already exists. This is primarily event mapping and serialization in the engine.

## Dependencies

- Spec 003 (Core Engine) - streaming infrastructure
- A reasoning-capable model for end-to-end testing (DeepSeek R1, Qwen QwQ)

## Questions

- Should reasoning events be emitted by default, or only when the client opts in via `include: ["reasoning.encrypted_content"]`?
- How do reasoning summaries work? Some models produce a summary after reasoning; the upstream spec has separate summary events. Do any open-source models currently produce summaries?
