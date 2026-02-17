// Package engine implements the core orchestration logic for Antwort.
// The Engine struct implements transport.ResponseCreator, bridging incoming
// OpenResponses API requests to provider backends. It handles request
// translation, provider invocation, streaming event mapping, conversation
// history reconstruction, and capability validation. Optional capabilities
// (storage, tools) use nil-safe composition for graceful degradation.
package engine
