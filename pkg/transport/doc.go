// Package transport defines the handler interfaces and middleware chain for
// the antwort HTTP/SSE transport layer.
//
// The transport layer bridges external clients and antwort's internal
// processing engine. It deserializes incoming requests into the core protocol
// types defined in pkg/api, dispatches them for processing, and serializes
// responses back to the client in either synchronous (JSON) or streaming
// (SSE) format.
//
// # Handler Interfaces
//
// Two handler interfaces define the contract between the transport layer and
// the processing engine:
//
//   - ResponseCreator handles the core create-response operation, available
//     in both stateless and stateful deployments.
//   - ResponseStore handles get and delete operations for stored responses,
//     available only in stateful deployments with persistence.
//
// The ResponseWriter interface abstracts streaming and non-streaming output,
// allowing the handler to emit SSE events or complete JSON responses without
// knowing the underlying transport protocol.
//
// # Middleware
//
// The middleware chain wraps ResponseCreator with cross-cutting concerns.
// Built-in middleware provides panic recovery, request ID assignment
// (X-Request-ID), and structured logging via log/slog. Custom middleware
// can be added for application-specific concerns.
//
// # Zero Dependencies
//
// This package uses only Go standard library packages, consistent with
// the project's zero-external-dependency constraint. HTTP serving uses
// net/http with Go 1.22+ ServeMux routing patterns. SSE flushing uses
// http.NewResponseController. Structured logging uses log/slog.
package transport
