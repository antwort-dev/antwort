# SSE Wire Format Contract

**Feature**: 002-transport-layer
**Date**: 2026-02-17
**Reference**: OpenResponses Streaming Protocol, OpenAI Responses API

## Overview

This document defines the exact Server-Sent Events (SSE) wire format used by antwort for streaming responses. The format is compatible with the OpenAI Responses API and follows the W3C SSE specification.

## Content-Type and Headers

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

## Event Format

Each event consists of:

```
event: {StreamEventType}\n
data: {JSON-serialized StreamEvent}\n
\n
```

Where:
- `{StreamEventType}` is one of the 14 event type constants from `pkg/api` (e.g., `response.created`, `response.output_text.delta`)
- `{JSON-serialized StreamEvent}` is the complete `api.StreamEvent` serialized as a single JSON line

## Terminal Sentinel

After the final event (one of `response.completed`, `response.failed`, or `response.cancelled`), the server sends:

```
data: [DONE]\n
\n
```

This follows the OpenAI convention. The `[DONE]` sentinel does NOT have an `event:` line.

## Complete Stream Example

```
event: response.created
data: {"type":"response.created","sequence_number":0,"response":{"id":"resp_abc123","object":"response","status":"in_progress","output":[],"model":"meta-llama/Llama-3-8B","created_at":1700000000}}

event: response.output_item.added
data: {"type":"response.output_item.added","sequence_number":1,"item":{"id":"item_def456","type":"message","status":"in_progress","message":{"role":"assistant"}},"output_index":0}

event: response.content_part.added
data: {"type":"response.content_part.added","sequence_number":2,"part":{"type":"output_text","text":""},"item_id":"item_def456","output_index":0,"content_index":0}

event: response.output_text.delta
data: {"type":"response.output_text.delta","sequence_number":3,"delta":"Hello","item_id":"item_def456","output_index":0,"content_index":0}

event: response.output_text.delta
data: {"type":"response.output_text.delta","sequence_number":4,"delta":" world!","item_id":"item_def456","output_index":0,"content_index":0}

event: response.output_text.done
data: {"type":"response.output_text.done","sequence_number":5,"delta":"Hello world!","item_id":"item_def456","output_index":0,"content_index":0}

event: response.content_part.done
data: {"type":"response.content_part.done","sequence_number":6,"item_id":"item_def456","output_index":0,"content_index":0}

event: response.output_item.done
data: {"type":"response.output_item.done","sequence_number":7,"item_id":"item_def456","output_index":0}

event: response.completed
data: {"type":"response.completed","sequence_number":8,"response":{"id":"resp_abc123","object":"response","status":"completed","output":[{"id":"item_def456","type":"message","status":"completed","message":{"role":"assistant","output":[{"type":"output_text","text":"Hello world!"}]}}],"model":"meta-llama/Llama-3-8B","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15},"created_at":1700000000}}

data: [DONE]

```

## Error During Stream

If an error occurs after streaming has begun, the server emits a `response.failed` event:

```
event: response.failed
data: {"type":"response.failed","sequence_number":5,"response":{"id":"resp_abc123","object":"response","status":"failed","output":[],"model":"meta-llama/Llama-3-8B","error":{"type":"server_error","message":"internal failure"},"created_at":1700000000}}

data: [DONE]

```

## Error Before Stream

If an error occurs before any events have been sent (e.g., during request validation), the server returns a standard JSON error response (NOT SSE format):

```
HTTP/1.1 400 Bad Request
Content-Type: application/json

{"error":{"type":"invalid_request","param":"model","message":"model is required"}}
```

## HTTP Error Code Mapping

| Error Type | HTTP Status |
|------------|-------------|
| invalid_request | 400 |
| not_found | 404 |
| too_many_requests | 429 |
| server_error | 500 |
| model_error | 500 |
| (body too large) | 413 |
| (unsupported content type) | 415 |
| (method not allowed) | 405 |
