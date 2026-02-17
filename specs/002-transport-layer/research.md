# Research: Transport Layer

**Feature**: 002-transport-layer
**Date**: 2026-02-17

## RT-1: Go 1.22+ ServeMux Routing Patterns

**Decision**: Use Go 1.22 `http.ServeMux` with method+pattern routing.

**Rationale**: Go 1.22 added method-aware routing to `http.ServeMux`. Patterns now support `METHOD /path` syntax and `{param}` path parameters. This eliminates the need for external routers like chi or gorilla/mux. The mux automatically returns 405 Method Not Allowed when a path matches but the method doesn't, satisfying FR-006a.

**Key patterns**:
- `mux.HandleFunc("POST /v1/responses", handler)` for method+path matching
- `mux.HandleFunc("GET /v1/responses/{id}", handler)` for path parameters
- `r.PathValue("id")` to extract path parameters
- Unmatched methods on valid paths return 405 automatically

**Alternatives considered**:
- chi router: good middleware support, but adds a dependency
- gorilla/mux: mature but unmaintained, adds a dependency
- Custom router: unnecessary complexity

## RT-2: SSE Implementation with http.NewResponseController

**Decision**: Use `http.NewResponseController` (Go 1.20+) for flushing SSE events.

**Rationale**: `http.NewResponseController` replaces the old `http.Flusher` type assertion pattern. It works through middleware ResponseWriter wrappers (via `Unwrap()`), returns errors from `Flush()`, and supports per-request deadline control. This is important for long-lived SSE connections.

**Key pattern**:
```
rc := http.NewResponseController(w)
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")
w.WriteHeader(200)

// For each event:
fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonPayload)
rc.Flush()  // returns error if client disconnected
```

**Alternatives considered**:
- `http.Flusher` type assertion: legacy pattern, breaks with ResponseWriter wrappers, no error return from Flush()
- Third-party SSE libraries: add dependencies, unnecessary for our use case

## RT-3: Graceful Shutdown

**Decision**: Use `http.Server.Shutdown()` with `signal.NotifyContext` and a timeout context.

**Rationale**: Standard Go pattern. `Shutdown` stops accepting new connections, waits for in-flight requests to complete, then closes idle connections. Combined with `signal.NotifyContext` for clean OS signal handling. The server runs `ListenAndServe` in a goroutine while the main goroutine waits for shutdown signals.

**Key pattern**:
```
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

go srv.ListenAndServe()
<-ctx.Done()

shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
srv.Shutdown(shutdownCtx)
```

**Alternatives considered**:
- `server.Close()`: forceful, doesn't wait for in-flight requests
- ory/graceful: adds dependency, wraps the same stdlib pattern

## RT-4: Structured Logging with log/slog

**Decision**: Use `log/slog` for structured logging middleware.

**Rationale**: `log/slog` (Go 1.21+) provides structured logging in the standard library. It supports key-value pairs, log levels, and handler-based output formatting. Request-scoped attributes can be stored in context and extracted by a custom `slog.Handler`.

**Key pattern**:
- Middleware stores request ID and other attributes in context
- Handlers use `slog.With()` or context-aware logging
- Output format (JSON/text) configured at server startup via `slog.NewJSONHandler` or `slog.NewTextHandler`

**Alternatives considered**:
- zerolog: fast, but adds a dependency
- zap: feature-rich, but adds a dependency
- log package: no structured logging support

## RT-5: Request Body Size Limiting

**Decision**: Use `http.MaxBytesReader` to wrap `r.Body` before JSON decoding.

**Rationale**: `http.MaxBytesReader` is specifically designed for HTTP body limiting. It returns a typed `*http.MaxBytesError` on overflow, closes the connection to prevent further reads, and works efficiently as a streaming wrapper (no need to read the entire body into memory first). Combined with `json.NewDecoder` for streaming JSON parsing.

**Key pattern**:
```
r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
var req api.CreateResponseRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    var maxErr *http.MaxBytesError
    if errors.As(err, &maxErr) {
        // Return 413
    }
    // Return 400 for other parse errors
}
```

**Alternatives considered**:
- `io.LimitReader`: doesn't close connection, no typed error, not HTTP-aware
- `http.MaxBytesHandler`: middleware approach, but we need per-endpoint control
- Content-Length header check: unreliable (optional header, can be spoofed)

## RT-6: Client Disconnect Detection

**Decision**: Rely on `http.Request.Context()` cancellation for client disconnect detection.

**Rationale**: The Go HTTP server automatically cancels the request context when the client disconnects. For SSE streaming, `rc.Flush()` returns an error when the connection is broken. Both mechanisms work together: the handler checks `ctx.Done()` for cooperative cancellation, and `WriteEvent` detects connection failures through flush errors.

**Caveats**:
- HTTP/2 has a known race condition where `ctx.Err()` may briefly be nil after disconnect
- `ReadTimeout` also cancels context (avoid setting it for SSE endpoints; use `SetReadDeadline` per-request instead)
- Pass the request context through all layers, never create new root contexts

**Alternatives considered**:
- Polling with `CloseNotifier`: deprecated since Go 1.11
- Custom heartbeat mechanism: adds complexity, unnecessary with context-based detection

## RT-7: Middleware Pattern

**Decision**: Use function composition: `type Middleware func(http.Handler) http.Handler`.

**Rationale**: This is the standard Go middleware pattern. Each middleware wraps an `http.Handler`, forming a chain. Recovery middleware is outermost (catches panics from everything inside), then request ID (available to all inner middleware), then logging (records the final status).

For the transport layer's own middleware (wrapping `ResponseCreator`), we use a parallel pattern: `type Middleware func(ResponseCreator) ResponseCreator`. The HTTP adapter converts between the two layers.

**Alternatives considered**:
- Interface-based middleware: more verbose, no practical advantage
- Middleware slice with Apply: essentially the same as composition with extra indirection
