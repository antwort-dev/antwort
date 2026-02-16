// Package api defines the core protocol types for the Antwort OpenResponses proxy.
//
// This package provides all data types needed to implement the OpenResponses
// specification (openresponses.org): Items, Content, Request/Response,
// streaming events, error types, state machine validation, and ID generation.
//
// The package has zero external dependencies (Go standard library only) and
// performs no I/O. All types produce JSON compatible with the OpenAI Responses
// API wire format, enabling client library compatibility.
//
// Core types:
//   - [Item]: Polymorphic unit of conversation (message, function_call, function_call_output, reasoning)
//   - [CreateResponseRequest]: Client request for model inference
//   - [Response]: Server response containing output items
//   - [StreamEvent]: Server-sent event for streaming responses
//   - [APIError]: Structured error with type, code, param, and message
//
// Extension support:
//
// Provider-specific item types use the "provider:type" naming convention
// (e.g., "acme:telemetry_chunk"). Extension data is preserved as raw JSON
// through round-trip serialization without schema validation.
package api
