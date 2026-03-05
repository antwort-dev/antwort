# Brainstorm 39: Async Responses (Background Mode)

**Date**: 2026-03-05
**Participants**: Roland Huss
**Goal**: Design the `background: true` async response mode for long-running agent requests.

## Motivation

Autonomous agents (OpenClaw, LangGraph, CrewAI, custom) often need to fire off inference requests that take minutes to complete (complex reasoning, multi-turn agentic loops with tool calls, large code generation). The current synchronous request/response model blocks the agent's control loop until the response is ready.

The OpenAI Responses API includes a `background` field for exactly this purpose. When `background: true`, the server:
1. Accepts the request immediately
2. Returns a response with status `queued` or `in_progress`
3. Processes the request asynchronously
4. The client polls for completion or receives a webhook callback

This is the #1 feature gap for Antwort's "inference gateway for agentic AI" positioning.

## Current State

The `background` field exists on the `Response` type (as a passthrough/echo field) but NOT on the `CreateResponseRequest` type. No async processing logic exists. All requests are handled synchronously.

```go
// Response type has it:
Background bool `json:"background"`

// CreateResponseRequest does NOT have it yet
```

## Design Questions

### Q1: Request Flow

When `background: true`:

1. **Accept**: Server receives POST /v1/responses with `background: true`
2. **Validate**: Normal request validation (model, input, tools, etc.)
3. **Queue**: Server creates a Response object with status `queued`, persists it to storage, and returns immediately (HTTP 200 with the queued response)
4. **Process**: A background worker picks up the request and processes it (inference, agentic loop, tool execution)
5. **Update**: As processing progresses, status changes from `queued` to `in_progress` to `completed` (or `failed`/`cancelled`)
6. **Retrieve**: Client polls GET /v1/responses/{id} to check status and get the result

### Q2: Worker Architecture

How should background requests be processed?

**Option A: Goroutine pool** (simplest)
- Background requests are processed by goroutines from a bounded pool
- Pool size configurable (e.g., `engine.max_background_workers: 10`)
- Queue is in-memory (requests lost on restart)
- Simple, no new dependencies

**Option B: Work queue with persistence** (more robust)
- Background requests are persisted to storage with status `queued`
- A worker goroutine pool processes queued responses
- Survives restarts (reprocesses queued items on startup)
- Requires storage backend (already have PostgreSQL)

**Option C: External job queue** (most scalable)
- Use an external message broker (Redis, NATS, etc.)
- Decoupled workers can scale independently
- Adds infrastructure dependency (conflicts with constitution Principle II)

**Recommendation**: Start with **Option A** (goroutine pool) for MVP, with the queue persisted to storage so responses survive restart. This keeps it simple and stdlib-only while being practical.

### Q3: Polling vs Webhooks vs SSE

How does the client get the result?

**Polling**: Client calls GET /v1/responses/{id} periodically until status is `completed`. Simple, reliable, already works.

**Webhooks**: Server POSTs the completed response to a client-specified URL. Requires new infrastructure (outbound HTTP, retry logic, webhook registration).

**SSE stream**: Client opens a streaming connection that receives events as the background request progresses. Could reuse existing SSE infrastructure.

**Recommendation**: Start with **polling only** for MVP. The GET /v1/responses/{id} endpoint already exists. No new infrastructure needed. Add webhooks and SSE as future enhancements.

### Q4: Cancellation

Can a client cancel a background request?

**Yes**: DELETE /v1/responses/{id} while status is `queued` or `in_progress` should cancel the background processing.

The existing DELETE handler already exists. It needs to be enhanced to cancel in-flight background requests (using context cancellation).

### Q5: Rate Limiting

Background requests consume resources over a longer period. Should they be rate-limited differently?

**Recommendation**: Background requests count against the same rate limits as synchronous requests. The goroutine pool size provides a natural backpressure mechanism. If the pool is full, new background requests can be rejected with a `429 Too Many Requests` response.

### Q6: Stateless Mode

What about `store: false` with `background: true`?

**Recommendation**: `background: true` requires `store: true`. If `store: false` is set with `background: true`, return a validation error. There's no way to retrieve a background response without storage.

### Q7: Streaming + Background

What about `stream: true` with `background: true`?

**Recommendation**: `background: true` is mutually exclusive with `stream: true`. If both are set, return a validation error. Streaming is inherently synchronous (the client must be connected to receive events). Background mode is for fire-and-forget.

However, a future enhancement could allow SSE streaming to a background response (client connects later and receives events from that point forward).

## Proposed Implementation

### Phase 1: MVP (spec candidate)

1. Add `Background bool` field to `CreateResponseRequest`
2. Validate: `background: true` requires `store: true`, mutually exclusive with `stream: true`
3. On `background: true`, create response with status `queued`, persist to storage, return immediately
4. Background worker goroutine pool picks up queued responses and processes them
5. Worker updates response status as it progresses (queued -> in_progress -> completed/failed)
6. Client polls GET /v1/responses/{id} to check status
7. DELETE /v1/responses/{id} cancels in-flight background requests
8. Config: `engine.max_background_workers` (default: 10)

### Phase 2: Enhancements (future)

- Webhook callbacks on completion
- SSE streaming to background responses (connect-later pattern)
- Priority queues (high-priority background requests)
- Persistent queue (reprocess on restart)
- Background request metrics (queue depth, wait time, processing time)

## Scope

### In Scope (Phase 1)
- `background` field on CreateResponseRequest
- Goroutine pool for background processing
- Status transitions (queued -> in_progress -> completed/failed/cancelled)
- Polling via GET /v1/responses/{id}
- Cancellation via DELETE /v1/responses/{id}
- Validation (store required, stream mutually exclusive)
- Configuration (pool size)

### Out of Scope
- Webhook callbacks
- SSE streaming to background responses
- Persistent queue (restart survival)
- External job queue (Redis, NATS)
- Priority queues

## Dependencies

- **Spec 005 (Storage)**: Background responses must be stored
- **Spec 042 (Audit Logging)**: Background request lifecycle should be audited

## Resolved Questions

1. **TTL**: Yes. Background responses auto-expire with a configurable TTL (default 24h). Expired responses cleaned up periodically. Prevents orphaned background responses from accumulating.

2. **Listing**: Add `?status=queued` and `?background=true` query parameters to the existing GET /v1/responses list endpoint. Reuses existing infrastructure, no new endpoint needed.

3. **Agentic loop**: Yes, full agentic loop support. Background mode is most valuable for complex agentic tasks (multi-turn tool calls, code execution) that take minutes. Without this, background mode has limited utility.

4. **Shutdown**: Graceful drain. On shutdown signal, stop accepting new background requests. Wait for in-flight requests to complete (with configurable timeout). Mark remaining as `failed` with reason. Aligns with constitution (graceful shutdown pattern).
