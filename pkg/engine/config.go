package engine

import (
	"context"

	"github.com/rhuss/antwort/pkg/agent"
	"github.com/rhuss/antwort/pkg/tools"
)

// Config holds configuration for the core engine.
type Config struct {
	// DefaultModel is used when the request omits the model field.
	// Empty string means a model is always required in the request.
	DefaultModel string

	// MaxAgenticTurns is the maximum number of turns in the agentic loop
	// before returning an incomplete response. Zero or negative means
	// use the default of 10.
	MaxAgenticTurns int

	// Executors is the list of tool executors available for the agentic
	// loop. When nil or empty, the engine falls back to single-shot
	// behavior (tool calls returned as function_call items).
	Executors []tools.ToolExecutor

	// Annotator generates citations on output text from tool result sources.
	// When nil, no annotations are generated (feature disabled).
	Annotator AnnotationGenerator

	// ProfileResolver resolves agent profiles by name.
	// When nil, the agent and prompt fields on requests are rejected.
	ProfileResolver agent.ProfileResolver

	// AuditLogger emits structured audit events for tool execution.
	// When nil, no audit events are emitted.
	AuditLogger AuditLogger
}

// AuditLogger defines the interface for emitting audit events.
// This avoids an import cycle between engine and audit packages.
// The audit.Logger type satisfies this interface.
type AuditLogger interface {
	Log(ctx context.Context, event string, attrs ...any)
	LogWarn(ctx context.Context, event string, attrs ...any)
}

// noopAuditLogger is a no-op implementation used when no audit logger is configured.
type noopAuditLogger struct{}

func (noopAuditLogger) Log(context.Context, string, ...any)     {}
func (noopAuditLogger) LogWarn(context.Context, string, ...any) {}

// maxTurns returns the effective max turns value, defaulting to 10.
func (c Config) maxTurns() int {
	if c.MaxAgenticTurns <= 0 {
		return 10
	}
	return c.MaxAgenticTurns
}
