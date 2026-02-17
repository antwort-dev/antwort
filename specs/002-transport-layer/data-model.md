# Data Model: Transport Layer

**Feature**: 002-transport-layer
**Date**: 2026-02-17

## Entities

### ResponseCreator (Interface)

The core handler contract for processing create-response requests. Used by both stateless and stateful deployments.

**Operations**:
| Operation | Input | Output | Description |
|-----------|-------|--------|-------------|
| CreateResponse | context, *api.CreateResponseRequest, ResponseWriter | error | Process a create-response request. Streaming responses use the ResponseWriter to emit events. Non-streaming responses use it to send the complete Response. |

**Constraints**:
- The context carries cancellation signals (client disconnect, timeout, explicit cancel)
- The handler MUST respect context cancellation (`ctx.Done()`)
- Errors returned are `*api.APIError` (mapped to HTTP status codes by the adapter)

### ResponseStore (Interface)

The handler contract for retrieving and deleting stored responses. Only used in stateful deployments.

**Operations**:
| Operation | Input | Output | Description |
|-----------|-------|--------|-------------|
| GetResponse | context, id (string) | *api.Response, error | Retrieve a stored response by ID |
| DeleteResponse | context, id (string) | error | Delete a stored response by ID |

**Constraints**:
- Returns `*api.APIError` with `not_found` type when ID doesn't exist
- Only available when the server is configured with persistence

### ResponseWriter (Interface)

The output abstraction provided by the transport layer to the handler. Handles SSE formatting internally.

**Operations**:
| Operation | Input | Output | Description |
|-----------|-------|--------|-------------|
| WriteEvent | context, api.StreamEvent | error | Send a single streaming event. Returns error if called after a terminal event. |
| WriteResponse | context, *api.Response | error | Send a complete non-streaming response. Mutually exclusive with WriteEvent. |
| Flush | (none) | error | Flush buffered data to the client |

**States**:
- `idle`: Initial state. Can transition to `streaming` or `completed`.
- `streaming`: After first WriteEvent call. Only WriteEvent and Flush are valid.
- `completed`: After WriteResponse, or after a terminal event (response.completed/failed/cancelled) is written via WriteEvent. No further writes allowed.

**Constraints**:
- WriteEvent and WriteResponse are mutually exclusive on a single writer instance
- WriteEvent after a terminal event returns an error
- WriteResponse after any WriteEvent returns an error
- Flush errors indicate client disconnection

### Middleware (Function Type)

Wraps ResponseCreator to add cross-cutting behavior.

**Signature**: `func(ResponseCreator) ResponseCreator`

**Built-in middleware** (applied in order, outermost first):

| Name | Order | Purpose |
|------|-------|---------|
| Recovery | 1 (outermost) | Catches panics, returns HTTP 500 |
| RequestID | 2 | Assigns/propagates X-Request-ID, adds to context |
| Logging | 3 | Emits structured log entry for each request |
| Custom... | 4+ (innermost) | User-provided middleware |

### InFlightRegistry

Tracks in-flight streaming responses for explicit cancellation.

**Attributes**:
| Attribute | Type | Description |
|-----------|------|-------------|
| entries | map[string]cancelFunc | Response ID to cancel function mapping |

**Operations**:
| Operation | Input | Output | Description |
|-----------|-------|--------|-------------|
| Register | responseID (string), cancelFunc | (none) | Register an in-flight response |
| Cancel | responseID (string) | bool | Cancel an in-flight response. Returns true if found. |
| Remove | responseID (string) | (none) | Remove entry after response completes |

**Constraints**:
- Must be safe for concurrent access (multiple streaming responses + DELETE requests)
- Entries are automatically removed when the response completes or is cancelled

### ServerConfig

Configuration for the HTTP server.

**Attributes**:
| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| Addr | string | ":8080" | Listen address (host:port) |
| MaxBodySize | int64 | 10485760 (10 MB) | Maximum request body size |
| ShutdownTimeout | time.Duration | 30s | Graceful shutdown deadline |

## Relationships

```
HTTPAdapter
  ├── uses ResponseCreator (required, for POST /v1/responses)
  ├── uses ResponseStore (optional, for GET/DELETE /v1/responses/{id})
  ├── creates ResponseWriter (one per streaming request)
  ├── owns InFlightRegistry (for cancellation)
  └── applies Middleware chain (wraps ResponseCreator)

ResponseCreator
  └── writes to ResponseWriter (provided by HTTPAdapter)

InFlightRegistry
  └── maps response ID -> cancel function
      (registered by HTTPAdapter, checked by DELETE handler)
```

## Reused Types from Spec 001 (pkg/api)

The transport layer does NOT define its own request/response/event types. It reuses:

- `api.CreateResponseRequest` - request body for POST
- `api.Response` - response body for GET and non-streaming POST
- `api.StreamEvent` - individual streaming event
- `api.StreamEventType` - event type constants (14 types)
- `api.APIError` - structured error type
- `api.ErrorResponse` - error wrapper for JSON responses
- `api.ErrorType*` constants - error type classification
