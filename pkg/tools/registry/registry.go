package registry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/tools"
)

// Prometheus metrics for built-in tool execution and API routes.
var (
	builtinToolExecutions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_builtin_tool_executions_total",
			Help: "Total built-in tool executions",
		},
		[]string{"provider", "tool_name", "status"},
	)

	builtinToolDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "antwort_builtin_tool_duration_seconds",
			Help:    "Built-in tool execution duration",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"provider", "tool_name"},
	)

	builtinAPIRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "antwort_builtin_api_requests_total",
			Help: "Total built-in provider API requests",
		},
		[]string{"provider", "method", "path", "status"},
	)

	builtinAPIDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "antwort_builtin_api_duration_seconds",
			Help:    "Built-in provider API request duration",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"provider", "method", "path"},
	)
)

func init() {
	prometheus.MustRegister(
		builtinToolExecutions,
		builtinToolDuration,
		builtinAPIRequests,
		builtinAPIDuration,
	)
}

// FunctionRegistry aggregates FunctionProviders and implements tools.ToolExecutor.
// It routes tool calls to the correct provider, records metrics, and serves
// merged HTTP routes from all providers.
type FunctionRegistry struct {
	mu sync.RWMutex

	// providers stores registered providers in insertion order.
	providers []FunctionProvider

	// toolToProvider maps tool name to the provider that owns it.
	toolToProvider map[string]FunctionProvider
}

// Ensure FunctionRegistry implements tools.ToolExecutor at compile time.
var _ tools.ToolExecutor = (*FunctionRegistry)(nil)

// New creates an empty FunctionRegistry.
func New() *FunctionRegistry {
	return &FunctionRegistry{
		toolToProvider: make(map[string]FunctionProvider),
	}
}

// Register adds a provider to the registry. Tool names are resolved on a
// first-come, first-served basis: if two providers supply a tool with the
// same name, the first registered provider wins and a warning is logged.
//
// Any provider-specific Prometheus collectors are also registered.
func (r *FunctionRegistry) Register(p FunctionProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers = append(r.providers, p)

	for _, td := range p.Tools() {
		if existing, ok := r.toolToProvider[td.Name]; ok {
			slog.Warn("builtin tool name conflict, keeping first provider",
				"tool", td.Name,
				"winner", existing.Name(),
				"loser", p.Name(),
			)
			continue
		}
		r.toolToProvider[td.Name] = p
	}

	// Register provider-specific Prometheus collectors.
	for _, c := range p.Collectors() {
		if err := prometheus.Register(c); err != nil {
			// Already registered is not an error worth crashing for.
			slog.Debug("collector already registered", "provider", p.Name(), "error", err)
		}
	}

	slog.Info("registered builtin provider",
		"provider", p.Name(),
		"tools", len(p.Tools()),
		"routes", len(p.Routes()),
	)
}

// Kind returns ToolKindBuiltin.
func (r *FunctionRegistry) Kind() tools.ToolKind {
	return tools.ToolKindBuiltin
}

// CanExecute returns true if any registered provider handles the named tool.
func (r *FunctionRegistry) CanExecute(toolName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.toolToProvider[toolName]
	return ok
}

// Execute routes the tool call to the correct provider, records metrics,
// and recovers from panics.
func (r *FunctionRegistry) Execute(ctx context.Context, call tools.ToolCall) (result *tools.ToolResult, err error) {
	r.mu.RLock()
	p, ok := r.toolToProvider[call.Name]
	r.mu.RUnlock()

	if !ok {
		return &tools.ToolResult{
			CallID:  call.ID,
			Output:  fmt.Sprintf("no builtin provider handles tool %q", call.Name),
			IsError: true,
		}, nil
	}

	providerName := p.Name()
	start := time.Now()

	// Recover from panics inside the provider.
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("builtin tool provider panicked",
				"provider", providerName,
				"tool", call.Name,
				"panic", rec,
			)
			result = &tools.ToolResult{
				CallID:  call.ID,
				Output:  fmt.Sprintf("internal error: builtin tool %q panicked", call.Name),
				IsError: true,
			}
			err = nil

			builtinToolExecutions.WithLabelValues(providerName, call.Name, "panic").Inc()
			builtinToolDuration.WithLabelValues(providerName, call.Name).Observe(time.Since(start).Seconds())
		}
	}()

	result, err = p.Execute(ctx, call)
	duration := time.Since(start).Seconds()

	status := "success"
	if err != nil {
		status = "error"
	} else if result != nil && result.IsError {
		status = "tool_error"
	}

	builtinToolExecutions.WithLabelValues(providerName, call.Name, status).Inc()
	builtinToolDuration.WithLabelValues(providerName, call.Name).Observe(duration)

	return result, err
}

// DiscoveredTools returns the merged tool definitions from all registered providers.
func (r *FunctionRegistry) DiscoveredTools() []api.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var allTools []api.ToolDefinition
	for _, p := range r.providers {
		allTools = append(allTools, p.Tools()...)
	}
	return allTools
}

// HTTPHandler returns an http.Handler that serves all provider routes,
// each wrapped with metrics middleware. The returned handler can be mounted
// behind the server's auth middleware.
func (r *FunctionRegistry) HTTPHandler() http.Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mux := http.NewServeMux()

	for _, p := range r.providers {
		for _, route := range p.Routes() {
			wrapped := wrapRoute(p.Name(), route)
			pattern := route.Method + " " + route.Pattern
			if route.Method == "" {
				pattern = route.Pattern
			}
			mux.HandleFunc(pattern, wrapped)
		}
	}

	return mux
}

// Close closes all registered providers, returning the last error encountered.
func (r *FunctionRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for _, p := range r.providers {
		if err := p.Close(); err != nil {
			slog.Warn("failed to close builtin provider", "provider", p.Name(), "error", err)
			lastErr = err
		}
	}
	return lastErr
}

// HasProviders returns true if at least one provider is registered.
func (r *FunctionRegistry) HasProviders() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers) > 0
}
