package engine

import (
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
}

// maxTurns returns the effective max turns value, defaulting to 10.
func (c Config) maxTurns() int {
	if c.MaxAgenticTurns <= 0 {
		return 10
	}
	return c.MaxAgenticTurns
}
