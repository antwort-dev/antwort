# Spec 02: Transport Layer

**Branch**: `spec/02-transport`
**Dependencies**: Spec 01 (Core Protocol)
**Package**: `github.com/rhuss/antwort/pkg/transport`

## Purpose

Define the transport interface and implement adapters for HTTP/SSE and gRPC. The transport layer is responsible for accepting client connections, deserializing requests, and delivering responses (including streaming).

## Scope

### In Scope
- Transport interface definition (protocol-agnostic)
- HTTP/SSE adapter with full OpenResponses streaming event protocol
- gRPC adapter with bidirectional streaming
- Streaming event types (delta events + state machine events)
- Connection lifecycle (keepalive, backpressure, cancellation)
- Content negotiation (JSON vs streaming)

### Out of Scope
- Authentication (see Spec 06, called via middleware hook)
- Business logic / orchestration (handled by Core Engine)
- Provider communication (see Spec 03)

## Transport Interface

The core abstraction decouples protocol handling from business logic:

```go
// Handler processes a single OpenResponses request.
// Implementations are protocol-agnostic.
type Handler interface {
    // HandleCreateResponse processes a create response request.
    // For streaming requests, events are sent via the ResponseWriter.
    HandleCreateResponse(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error

    // HandleGetResponse retrieves a stored response by ID.
    HandleGetResponse(ctx context.Context, id string) (*api.Response, error)

    // HandleDeleteResponse deletes a stored response by ID.
    HandleDeleteResponse(ctx context.Context, id string) error
}

// ResponseWriter abstracts streaming output across protocols.
type ResponseWriter interface {
    // WriteEvent sends a single streaming event.
    // The transport adapter serializes it for the specific protocol.
    WriteEvent(ctx context.Context, event StreamEvent) error

    // WriteResponse sends a complete non-streaming response.
    WriteResponse(ctx context.Context, resp *api.Response) error

    // Flush ensures buffered data is sent to the client.
    Flush() error
}

// Middleware wraps a Handler to add cross-cutting concerns.
type Middleware func(Handler) Handler
```

## Streaming Events

Per the OpenResponses spec, streaming uses SSE with typed events:

```go
// StreamEventType identifies the event kind.
type StreamEventType string

const (
    // Delta events (incremental changes)
    EventResponseCreated       StreamEventType = "response.created"
    EventResponseInProgress    StreamEventType = "response.in_progress"
    EventOutputItemAdded       StreamEventType = "response.output_item.added"
    EventContentPartAdded      StreamEventType = "response.content_part.added"
    EventOutputTextDelta       StreamEventType = "response.output_text.delta"
    EventOutputTextDone        StreamEventType = "response.output_text.done"
    EventContentPartDone       StreamEventType = "response.content_part.done"
    EventOutputItemDone        StreamEventType = "response.output_item.done"

    // Function call events
    EventFunctionCallArgsDelta StreamEventType = "response.function_call_arguments.delta"
    EventFunctionCallArgsDone  StreamEventType = "response.function_call_arguments.done"

    // State machine events
    EventResponseCompleted     StreamEventType = "response.completed"
    EventResponseFailed        StreamEventType = "response.failed"
    EventResponseCancelled     StreamEventType = "response.cancelled"
)

// StreamEvent is a single event in the streaming protocol.
type StreamEvent struct {
    Type     StreamEventType `json:"type"`
    Response *api.Response   `json:"response,omitempty"`
    Item     *api.Item       `json:"item,omitempty"`
    Part     *api.ModelContent `json:"part,omitempty"`
    Delta    string          `json:"delta,omitempty"`

    // For provider extension events
    ExtensionType string          `json:"extension_type,omitempty"`
    ExtensionData json.RawMessage `json:"extension_data,omitempty"`
}
```

## HTTP/SSE Adapter

```go
// HTTPAdapter serves the OpenResponses API over HTTP.
type HTTPAdapter struct {
    handler    Handler
    middleware []Middleware
    addr       string
}

// NewHTTPAdapter creates an HTTP adapter.
// Uses Go standard library net/http with http.Flusher for SSE.
func NewHTTPAdapter(handler Handler, opts ...HTTPOption) *HTTPAdapter

// Routes:
//   POST /v1/responses        -> HandleCreateResponse
//   GET  /v1/responses/{id}   -> HandleGetResponse
//   DELETE /v1/responses/{id} -> HandleDeleteResponse
```

SSE wire format per spec:

```
event: response.created
data: {"type":"response.created","response":{"id":"resp_...","status":"in_progress",...}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"Hello"}

event: response.completed
data: {"type":"response.completed","response":{...}}

data: [DONE]
```

## gRPC Adapter

```go
// GRPCAdapter serves the OpenResponses API over gRPC.
// Uses server-streaming RPC for streaming responses.
type GRPCAdapter struct {
    handler Handler
    addr    string
}
```

Proto definition (sketch):

```protobuf
service ResponsesService {
    rpc CreateResponse(CreateResponseRequest) returns (stream StreamEvent);
    rpc GetResponse(GetResponseRequest) returns (Response);
    rpc DeleteResponse(DeleteResponseRequest) returns (google.protobuf.Empty);
}
```

## Middleware Hooks

The transport layer provides hooks for cross-cutting concerns:

```go
// Built-in middleware slots (applied in order):
// 1. Recovery (panic -> 500)
// 2. RequestID (assign unique ID)
// 3. Logging (structured request/response logging)
// 4. Auth (pluggable, see Spec 06)
// 5. RateLimit (pluggable, see Spec 06)
// 6. Metrics (request count, latency, error rate)
```

## Extension Points

- **Custom streaming events**: Provider-prefixed event types flow through `StreamEvent.ExtensionType`
- **Custom transport adapters**: Implement the `Handler` interface for new protocols
- **Middleware chain**: Add custom middleware via `HTTPOption` or `GRPCOption`

Note: Envoy ext_proc was considered but removed from scope. Antwort operates as a standalone server, not as an Envoy filter.

## Open Questions

- Should the gRPC proto be auto-generated from the OpenResponses JSON Schema, or hand-written?
- How to handle SSE reconnection (Last-Event-ID header)?

## Deliverables

- [ ] `pkg/transport/handler.go` - Handler and ResponseWriter interfaces
- [ ] `pkg/transport/events.go` - StreamEvent types
- [ ] `pkg/transport/middleware.go` - Middleware chain
- [ ] `pkg/transport/http/adapter.go` - HTTP/SSE adapter
- [ ] `pkg/transport/http/sse.go` - SSE serialization
- [ ] `pkg/transport/grpc/adapter.go` - gRPC adapter
- [ ] `pkg/transport/grpc/proto/responses.proto` - Proto definitions
- [ ] Tests for each adapter
