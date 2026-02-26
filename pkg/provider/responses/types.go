// Package responses implements a Provider adapter for backends that support
// the OpenAI Responses API (/v1/responses). It forwards inference requests
// using the Responses API wire format and consumes native SSE events.
package responses

import "encoding/json"

// --- Request types ---

// responsesRequest is the wire format for POST /v1/responses.
type responsesRequest struct {
	Model       string            `json:"model"`
	Input       json.RawMessage   `json:"input"`
	Tools       []responsesTool   `json:"tools,omitempty"`
	ToolChoice  any               `json:"tool_choice,omitempty"`
	Store       bool              `json:"store"`
	Stream      bool              `json:"stream,omitempty"`
	Temperature *float64          `json:"temperature,omitempty"`
	TopP        *float64          `json:"top_p,omitempty"`
	MaxOutputTokens *int          `json:"max_output_tokens,omitempty"`
	Stop        []string          `json:"stop,omitempty"`
	User        string            `json:"user,omitempty"`

	// ResponseFormat maps to text.format in the Responses API.
	Text *responsesTextConfig `json:"text,omitempty"`
}

// responsesTextConfig carries the text output format constraint.
type responsesTextConfig struct {
	Format json.RawMessage `json:"format,omitempty"`
}

// responsesTool is a tool definition in the Responses API format.
type responsesTool struct {
	Type     string              `json:"type"`
	Name     string              `json:"name,omitempty"`
	Description string           `json:"description,omitempty"`
	Parameters json.RawMessage   `json:"parameters,omitempty"`
	Strict   bool                `json:"strict,omitempty"`
}

// --- Response types ---

// responsesResponse is the wire format returned by POST /v1/responses (non-streaming).
type responsesResponse struct {
	ID          string             `json:"id"`
	Object      string             `json:"object"`
	CreatedAt   int64              `json:"created_at"`
	Status      string             `json:"status"`
	Model       string             `json:"model"`
	Output      []responsesItem    `json:"output"`
	Usage       *responsesUsage    `json:"usage,omitempty"`
	Error       *responsesError    `json:"error,omitempty"`
}

// responsesItem represents an output item (message, function_call, etc.).
type responsesItem struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"` // "message", "function_call"
	Status  string          `json:"status,omitempty"`
	Role    string          `json:"role,omitempty"`
	Content json.RawMessage `json:"content,omitempty"` // array of content parts for messages
	CallID  string          `json:"call_id,omitempty"`
	Name    string          `json:"name,omitempty"`
	Arguments string        `json:"arguments,omitempty"`
}

// responsesContentPart is a content part within a message item.
type responsesContentPart struct {
	Type string `json:"type"` // "output_text"
	Text string `json:"text,omitempty"`
}

// responsesUsage holds token usage from the backend.
type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// responsesError is the error format in Responses API responses.
type responsesError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// --- SSE event types ---

// sseEvent represents a parsed SSE event from the streaming Responses API.
type sseEvent struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// Common SSE event type strings from the Responses API.
const (
	eventResponseCreated     = "response.created"
	eventResponseCompleted   = "response.completed"
	eventResponseFailed      = "response.failed"
	eventOutputItemAdded     = "response.output_item.added"
	eventOutputItemDone      = "response.output_item.done"
	eventContentPartAdded    = "response.content_part.added"
	eventContentPartDone     = "response.content_part.done"
	eventTextDelta           = "response.output_text.delta"
	eventTextDone            = "response.output_text.done"
	eventFuncCallArgsDelta   = "response.function_call_arguments.delta"
	eventFuncCallArgsDone    = "response.function_call_arguments.done"
	eventReasoningDelta      = "response.reasoning.delta"
	eventReasoningDone       = "response.reasoning.done"
)

// textDeltaData is the data payload for response.output_text.delta events.
type textDeltaData struct {
	Delta string `json:"delta"`
}

// funcCallArgsDeltaData is the payload for response.function_call_arguments.delta events.
type funcCallArgsDeltaData struct {
	Delta       string `json:"delta"`
	CallID      string `json:"call_id,omitempty"`
	Name        string `json:"name,omitempty"`
	ItemID      string `json:"item_id,omitempty"`
	OutputIndex int    `json:"output_index"`
}

// funcCallArgsDoneData is the payload for response.function_call_arguments.done events.
type funcCallArgsDoneData struct {
	Arguments   string `json:"arguments"`
	CallID      string `json:"call_id,omitempty"`
	Name        string `json:"name,omitempty"`
	OutputIndex int    `json:"output_index"`
}

// responseCompletedData wraps the full response in a completed event.
type responseCompletedData struct {
	Response responsesResponse `json:"response"`
}
