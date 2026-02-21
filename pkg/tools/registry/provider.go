// Package registry provides a pluggable framework for built-in hosted tools.
// A FunctionProvider encapsulates a set of tools that the gateway executes
// server-side, along with optional HTTP routes (for webhooks, callbacks, etc.)
// and Prometheus collectors for custom metrics.
//
// The FunctionRegistry aggregates providers, implements tools.ToolExecutor,
// and exposes a merged HTTP handler for all provider routes.
package registry

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/tools"
)

// FunctionProvider is a pluggable built-in tool provider.
// Each provider contributes a set of tool definitions, an execution handler,
// optional HTTP routes, and optional Prometheus collectors.
type FunctionProvider interface {
	// Name returns a unique identifier for this provider (e.g., "code_interpreter").
	Name() string

	// Tools returns the tool definitions this provider contributes.
	Tools() []api.ToolDefinition

	// CanExecute reports whether this provider handles the named tool.
	CanExecute(name string) bool

	// Execute runs a tool call and returns the result.
	Execute(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error)

	// Routes returns HTTP endpoints that this provider exposes (webhooks, status, etc.).
	Routes() []Route

	// Collectors returns Prometheus collectors for provider-specific metrics.
	Collectors() []prometheus.Collector

	// Close releases any resources held by the provider.
	Close() error
}

// Route is an HTTP endpoint exposed by a provider.
type Route struct {
	Method  string
	Pattern string
	Handler http.HandlerFunc
}
