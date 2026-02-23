package provider

import (
	"encoding/json"

	"github.com/rhuss/antwort/pkg/api"
)

// ProviderCapabilities declares what features the backend supports.
// Used by the engine for early request validation.
type ProviderCapabilities struct {
	// Streaming indicates whether the provider supports streaming responses.
	Streaming bool

	// ToolCalling indicates whether the provider supports function/tool calls.
	ToolCalling bool

	// Vision indicates whether the provider supports image inputs.
	Vision bool

	// Audio indicates whether the provider supports audio inputs.
	Audio bool

	// Reasoning indicates whether the provider can produce reasoning items.
	Reasoning bool

	// MaxContextWindow is the maximum token count (0 = unknown/unlimited).
	MaxContextWindow int

	// SupportedModels lists models this provider can serve.
	// Empty means "ask ListModels()".
	SupportedModels []string

	// Extensions lists provider-specific extension types supported.
	Extensions []string
}

// ProviderRequest is the backend-facing request. It contains only the
// information the provider needs, stripped of transport and storage concerns.
type ProviderRequest struct {
	Model       string            `json:"model"`
	Messages    []ProviderMessage `json:"messages"`
	Tools       []ProviderTool    `json:"tools,omitempty"`
	ToolChoice  *api.ToolChoice   `json:"tool_choice,omitempty"`
	Temperature *float64          `json:"temperature,omitempty"`
	TopP        *float64          `json:"top_p,omitempty"`
	MaxTokens   *int              `json:"max_tokens,omitempty"`
	Stop             []string          `json:"stop,omitempty"`
	Stream           bool              `json:"stream,omitempty"`
	FrequencyPenalty *float64          `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64          `json:"presence_penalty,omitempty"`
	TopLogprobs      *int              `json:"top_logprobs,omitempty"`
	User             string            `json:"user,omitempty"`

	// Extra holds provider-specific parameters that don't map to standard fields.
	Extra map[string]any `json:"-"`
}

// ProviderMessage represents a message in the provider's conversation format.
type ProviderMessage struct {
	Role       string             `json:"role"`
	Content    any                `json:"content"`
	ToolCalls  []ProviderToolCall `json:"tool_calls,omitempty"`
	ToolCallID string             `json:"tool_call_id,omitempty"`
	Name       string             `json:"name,omitempty"`
}

// ProviderToolCall represents a tool call entry in an assistant message.
type ProviderToolCall struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function ProviderFunctionCall `json:"function"`
}

// ProviderFunctionCall holds the function name and arguments for a tool call.
type ProviderFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ProviderTool represents a tool definition in provider format.
type ProviderTool struct {
	Type     string              `json:"type"`
	Function ProviderFunctionDef `json:"function"`
}

// ProviderFunctionDef holds a function definition for tool use.
type ProviderFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ProviderResponse is the backend's complete non-streaming response.
type ProviderResponse struct {
	Items  []api.Item         `json:"items"`
	Usage  api.Usage          `json:"usage"`
	Model  string             `json:"model"`
	Status api.ResponseStatus `json:"status"`
}

// ProviderEventType classifies a streaming event from the backend.
type ProviderEventType int

const (
	ProviderEventTextDelta      ProviderEventType = iota // Incremental text content
	ProviderEventTextDone                                // Text content complete
	ProviderEventToolCallDelta                           // Incremental tool call arguments
	ProviderEventToolCallDone                            // Tool call complete
	ProviderEventReasoningDelta                          // Incremental reasoning content
	ProviderEventReasoningDone                           // Reasoning content complete
	ProviderEventDone                                    // Stream finished
	ProviderEventError                                   // Stream error
)

// ProviderEvent is a single streaming event from the backend.
type ProviderEvent struct {
	// Type indicates what kind of event this is.
	Type ProviderEventType

	// Delta contains incremental text or argument data.
	Delta string

	// ToolCallIndex identifies which tool call this event relates to.
	ToolCallIndex int

	// ToolCallID is the identifier for the tool call.
	ToolCallID string

	// FunctionName is the function name (populated on first tool call event).
	FunctionName string

	// Item is populated for item-level done events.
	Item *api.Item

	// Usage is populated on the final event.
	Usage *api.Usage

	// Err is populated if the stream encountered an error.
	Err error
}

// ModelInfo holds information about a model served by the provider.
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
}
