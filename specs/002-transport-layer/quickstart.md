# Quickstart: Transport Layer

## Prerequisites

- Go 1.22+
- Spec 001 (Core Protocol) implemented (`pkg/api/`)
- No external dependencies (stdlib only)

## Package Location

```
pkg/transport/
├── handler.go          # ResponseCreator, ResponseStore, ResponseWriter interfaces
├── middleware.go        # Middleware type, chain builder, built-in middleware
├── server.go            # Server struct, config, startup, graceful shutdown
├── inflight.go          # In-flight registry for cancellation
├── handler_test.go      # Interface contract tests (mock-based)
├── middleware_test.go   # Middleware chain and built-in middleware tests
├── server_test.go       # HTTP integration tests (httptest-based)
├── inflight_test.go     # Registry concurrency tests
└── http/
    ├── adapter.go       # HTTP adapter (routes, request parsing, response writing)
    ├── sse.go           # SSE ResponseWriter implementation
    ├── adapter_test.go  # HTTP adapter tests
    └── sse_test.go      # SSE wire format tests
```

## Quick Verification

After implementation, verify the transport layer works:

```bash
# Run all tests
go test ./pkg/transport/...

# Run with verbose output
go test -v ./pkg/transport/...

# Run specific test
go test -v -run TestSSEStreamFormat ./pkg/transport/http/
```

## Key Usage Patterns

### Starting a Server

```go
creator := &myHandler{} // implements transport.ResponseCreator
store := &myStore{}      // implements transport.ResponseStore (optional)

srv := transport.NewServer(
    transport.WithAddr(":8080"),
    transport.WithCreator(creator),
    transport.WithStore(store),              // optional
    transport.WithMaxBodySize(10 << 20),     // 10 MB
    transport.WithShutdownTimeout(30 * time.Second),
)

// Blocks until shutdown signal (SIGINT/SIGTERM)
if err := srv.ListenAndServe(); err != nil {
    log.Fatal(err)
}
```

### Implementing ResponseCreator

```go
type myHandler struct{}

func (h *myHandler) CreateResponse(
    ctx context.Context,
    req *api.CreateResponseRequest,
    w transport.ResponseWriter,
) error {
    if req.Stream {
        // Streaming: emit events via ResponseWriter
        w.WriteEvent(ctx, api.StreamEvent{
            Type:           api.EventResponseCreated,
            SequenceNumber: 0,
            Response:       &api.Response{ID: api.NewResponseID(), Status: api.ResponseStatusInProgress},
        })
        // ... emit delta events ...
        w.WriteEvent(ctx, api.StreamEvent{
            Type:           api.EventResponseCompleted,
            SequenceNumber: n,
            Response:       &completedResponse,
        })
    } else {
        // Non-streaming: send complete response
        w.WriteResponse(ctx, &completedResponse)
    }
    return nil
}
```

### Adding Custom Middleware

```go
// Custom middleware that adds a header
func addHeader(next transport.ResponseCreator) transport.ResponseCreator {
    return transport.ResponseCreatorFunc(func(
        ctx context.Context,
        req *api.CreateResponseRequest,
        w transport.ResponseWriter,
    ) error {
        // pre-processing
        return next.CreateResponse(ctx, req, w)
    })
}

srv := transport.NewServer(
    transport.WithCreator(creator),
    transport.WithMiddleware(addHeader),
)
```

## What This Package Does NOT Do

- **No business logic**: See Spec 03 (Provider Abstraction)
- **No persistence**: See Spec 05 (Storage)
- **No authentication**: See Spec 06 (Auth)
- **No provider calls**: See Spec 03 (Provider)

This package handles HTTP serving, SSE streaming, request routing, middleware, and connection lifecycle. Business logic is injected via the ResponseCreator and ResponseStore interfaces.
