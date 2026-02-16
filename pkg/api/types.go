package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Content types (T007)
// ---------------------------------------------------------------------------

// ContentPart represents a part of user input content.
// The Type field indicates the kind of content: input_text, input_image,
// input_audio, or input_video.
type ContentPart struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	URL       string `json:"url,omitempty"`
	Data      string `json:"data,omitempty"`
	MediaType string `json:"media_type,omitempty"`
}

// OutputContentPart represents a part of model output content.
// The Type field indicates the kind: output_text or summary_text.
type OutputContentPart struct {
	Type        string       `json:"type"`
	Text        string       `json:"text"`
	Annotations []Annotation `json:"annotations,omitempty"`
	Logprobs    []TokenLogprob `json:"logprobs,omitempty"`
}

// Annotation represents an annotation on output text, such as a citation.
type Annotation struct {
	Type       string `json:"type"`
	Text       string `json:"text,omitempty"`
	StartIndex int    `json:"start_index,omitempty"`
	EndIndex   int    `json:"end_index,omitempty"`
}

// TokenLogprob holds log probability information for a single token.
type TokenLogprob struct {
	Token       string       `json:"token"`
	Logprob     float64      `json:"logprob"`
	TopLogprobs []TopLogprob `json:"top_logprobs,omitempty"`
}

// TopLogprob holds a candidate token and its log probability.
type TopLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
}

// ---------------------------------------------------------------------------
// Item type-specific data structs (T008)
// ---------------------------------------------------------------------------

// MessageRole represents the role of a message sender.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

// ItemType represents the type of an item in a conversation.
type ItemType string

const (
	ItemTypeMessage            ItemType = "message"
	ItemTypeFunctionCall       ItemType = "function_call"
	ItemTypeFunctionCallOutput ItemType = "function_call_output"
	ItemTypeReasoning          ItemType = "reasoning"
)

// ItemStatus represents the processing status of an item.
type ItemStatus string

const (
	ItemStatusInProgress ItemStatus = "in_progress"
	ItemStatusIncomplete ItemStatus = "incomplete"
	ItemStatusCompleted  ItemStatus = "completed"
	ItemStatusFailed     ItemStatus = "failed"
)

// MessageData holds the data specific to a message item.
type MessageData struct {
	Role    MessageRole       `json:"role"`
	Content []ContentPart     `json:"content,omitempty"`
	Output  []OutputContentPart `json:"output,omitempty"`
}

// FunctionCallData holds the data specific to a function call item.
type FunctionCallData struct {
	Name      string `json:"name"`
	CallID    string `json:"call_id"`
	Arguments string `json:"arguments"`
}

// FunctionCallOutputData holds the data specific to a function call output item.
type FunctionCallOutputData struct {
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

// ReasoningData holds the data specific to a reasoning item.
type ReasoningData struct {
	Content          string `json:"content,omitempty"`
	EncryptedContent string `json:"encrypted_content,omitempty"`
	Summary          string `json:"summary,omitempty"`
}

// ---------------------------------------------------------------------------
// Item struct (T009)
// ---------------------------------------------------------------------------

// Item represents a single item in a conversation, which can be a message,
// function call, function call output, reasoning step, or a provider extension type.
type Item struct {
	ID     string     `json:"id"`
	Type   ItemType   `json:"type"`
	Status ItemStatus `json:"status"`

	Message            *MessageData            `json:"message,omitempty"`
	FunctionCall       *FunctionCallData       `json:"function_call,omitempty"`
	FunctionCallOutput *FunctionCallOutputData `json:"function_call_output,omitempty"`
	Reasoning          *ReasoningData          `json:"reasoning,omitempty"`

	Extension json.RawMessage `json:"extension,omitempty"`
}

// IsExtensionType checks whether the given ItemType represents a provider
// extension type, identified by a colon in the type string (e.g., "provider:type").
func IsExtensionType(t ItemType) bool {
	return strings.Contains(string(t), ":")
}

// ---------------------------------------------------------------------------
// ToolChoice union type (T010)
// ---------------------------------------------------------------------------

// ToolChoice represents a tool selection strategy. It can be a simple string
// value ("auto", "required", "none") or a structured function selection.
type ToolChoice struct {
	String   string              `json:"-"`
	Function *ToolChoiceFunction `json:"-"`
}

// ToolChoiceFunction specifies a particular function to call by name.
type ToolChoiceFunction struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

var (
	// ToolChoiceAuto lets the model decide whether to use a tool.
	ToolChoiceAuto = ToolChoice{String: "auto"}
	// ToolChoiceRequired forces the model to use a tool.
	ToolChoiceRequired = ToolChoice{String: "required"}
	// ToolChoiceNone prevents the model from using any tool.
	ToolChoiceNone = ToolChoice{String: "none"}
)

// NewToolChoiceFunction creates a ToolChoice that selects a specific function by name.
func NewToolChoiceFunction(name string) ToolChoice {
	return ToolChoice{
		Function: &ToolChoiceFunction{
			Type: "function",
			Name: name,
		},
	}
}

// MarshalJSON serializes ToolChoice as either a JSON string or a JSON object.
func (tc ToolChoice) MarshalJSON() ([]byte, error) {
	if tc.String != "" {
		return json.Marshal(tc.String)
	}
	if tc.Function != nil {
		return json.Marshal(tc.Function)
	}
	return nil, fmt.Errorf("ToolChoice has neither string value nor function")
}

// UnmarshalJSON deserializes ToolChoice from either a JSON string or a JSON object.
func (tc *ToolChoice) UnmarshalJSON(data []byte) error {
	// Try string first.
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		tc.String = s
		tc.Function = nil
		return nil
	}

	// Try structured object.
	var f ToolChoiceFunction
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("tool_choice must be a string or object: %w", err)
	}
	tc.String = ""
	tc.Function = &f
	return nil
}

// ToolDefinition describes a tool available to the model.
type ToolDefinition struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ---------------------------------------------------------------------------
// Request and Response types (T011)
// ---------------------------------------------------------------------------

// CreateResponseRequest represents the request body for creating a new response.
type CreateResponseRequest struct {
	Model              string                       `json:"model"`
	Input              []Item                       `json:"input"`
	Instructions       string                       `json:"instructions,omitempty"`
	Tools              []ToolDefinition             `json:"tools,omitempty"`
	ToolChoice         *ToolChoice                  `json:"tool_choice,omitempty"`
	AllowedTools       []string                     `json:"allowed_tools,omitempty"`
	Store              *bool                        `json:"store,omitempty"`
	Stream             bool                         `json:"stream,omitempty"`
	PreviousResponseID string                       `json:"previous_response_id,omitempty"`
	Truncation         string                       `json:"truncation,omitempty"`
	ServiceTier        string                       `json:"service_tier,omitempty"`
	MaxOutputTokens    *int                         `json:"max_output_tokens,omitempty"`
	Temperature        *float64                     `json:"temperature,omitempty"`
	TopP               *float64                     `json:"top_p,omitempty"`
	Extensions         map[string]json.RawMessage   `json:"extensions,omitempty"`
}

// ResponseStatus represents the overall status of a response.
type ResponseStatus string

const (
	ResponseStatusQueued     ResponseStatus = "queued"
	ResponseStatusInProgress ResponseStatus = "in_progress"
	ResponseStatusCompleted  ResponseStatus = "completed"
	ResponseStatusFailed     ResponseStatus = "failed"
	ResponseStatusCancelled  ResponseStatus = "cancelled"
)

// Response represents the API response object returned by the Responses API.
type Response struct {
	ID                 string                     `json:"id"`
	Object             string                     `json:"object"`
	Status             ResponseStatus             `json:"status"`
	Output             []Item                     `json:"output"`
	Model              string                     `json:"model"`
	Usage              *Usage                     `json:"usage,omitempty"`
	Error              *APIError                  `json:"error,omitempty"`
	PreviousResponseID string                     `json:"previous_response_id,omitempty"`
	CreatedAt          int64                      `json:"created_at"`
	Extensions         map[string]json.RawMessage `json:"extensions,omitempty"`
}

// Usage holds token usage information for a response.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
