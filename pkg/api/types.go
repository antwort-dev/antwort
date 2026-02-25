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
	Type        string         `json:"-"`
	Text        string         `json:"-"`
	Annotations []Annotation   `json:"-"`
	Logprobs    []TokenLogprob `json:"-"`
}

// MarshalJSON ensures annotations and logprobs are always arrays, never null.
func (p OutputContentPart) MarshalJSON() ([]byte, error) {
	type wire struct {
		Type        string         `json:"type"`
		Text        string         `json:"text"`
		Annotations []Annotation   `json:"annotations"`
		Logprobs    []TokenLogprob `json:"logprobs"`
	}
	w := wire{
		Type:        p.Type,
		Text:        p.Text,
		Annotations: p.Annotations,
		Logprobs:    p.Logprobs,
	}
	if w.Annotations == nil {
		w.Annotations = []Annotation{}
	}
	if w.Logprobs == nil {
		w.Logprobs = []TokenLogprob{}
	}
	return json.Marshal(w)
}

// UnmarshalJSON deserializes an OutputContentPart.
func (p *OutputContentPart) UnmarshalJSON(data []byte) error {
	type wire struct {
		Type        string         `json:"type"`
		Text        string         `json:"text"`
		Annotations []Annotation   `json:"annotations"`
		Logprobs    []TokenLogprob `json:"logprobs"`
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	p.Type = w.Type
	p.Text = w.Text
	p.Annotations = w.Annotations
	p.Logprobs = w.Logprobs
	return nil
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
	ItemTypeReasoning              ItemType = "reasoning"
	ItemTypeCodeInterpreterCall    ItemType = "code_interpreter_call"
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

// CodeInterpreterCallData holds the data specific to a code_interpreter_call item.
type CodeInterpreterCallData struct {
	Code    string                    `json:"code"`
	Outputs []CodeInterpreterOutput   `json:"outputs"`
}

// CodeInterpreterOutput represents a single output from code execution.
type CodeInterpreterOutput struct {
	Type  string                        `json:"type"` // "logs" or "image"
	Logs  string                        `json:"logs,omitempty"`
	Image *CodeInterpreterOutputImage   `json:"image,omitempty"`
}

// CodeInterpreterOutputImage holds file reference for an image output.
type CodeInterpreterOutputImage struct {
	FileID string `json:"file_id"`
	URL    string `json:"url,omitempty"`
}

// ---------------------------------------------------------------------------
// Item struct (T009)
// ---------------------------------------------------------------------------

// Item represents a single item in a conversation, which can be a message,
// function call, function call output, reasoning step, code interpreter call,
// or a provider extension type.
type Item struct {
	ID     string     `json:"id"`
	Type   ItemType   `json:"type"`
	Status ItemStatus `json:"status"`

	Message              *MessageData              `json:"message,omitempty"`
	FunctionCall         *FunctionCallData         `json:"function_call,omitempty"`
	FunctionCallOutput   *FunctionCallOutputData   `json:"function_call_output,omitempty"`
	Reasoning            *ReasoningData            `json:"reasoning,omitempty"`
	CodeInterpreterCall  *CodeInterpreterCallData  `json:"code_interpreter,omitempty"`

	Extension json.RawMessage `json:"extension,omitempty"`
}

// MarshalJSON serializes an Item to the OpenResponses wire format.
// The wire format is flat: type-specific fields are at the top level,
// not nested in a wrapper object (message, function_call, etc.).
func (item Item) MarshalJSON() ([]byte, error) {
	switch item.Type {
	case ItemTypeMessage:
		return item.marshalMessage()
	case ItemTypeFunctionCall:
		return item.marshalFunctionCall()
	case ItemTypeFunctionCallOutput:
		return item.marshalFunctionCallOutput()
	case ItemTypeReasoning:
		return item.marshalReasoning()
	case ItemTypeCodeInterpreterCall:
		return item.marshalCodeInterpreterCall()
	default:
		// Extension types or unknown: include extension data.
		type wireExtension struct {
			itemWireBase
			Extension json.RawMessage `json:"extension,omitempty"`
		}
		return json.Marshal(wireExtension{
			itemWireBase: itemWireBase{ID: item.ID, Type: item.Type, Status: item.Status},
			Extension:    item.Extension,
		})
	}
}

// itemWireBase contains fields common to all item types.
type itemWireBase struct {
	ID     string     `json:"id"`
	Type   ItemType   `json:"type"`
	Status ItemStatus `json:"status"`
}

// marshalMessage produces the flat message wire format:
// {type, id, status, role, content: [...]}
func (item Item) marshalMessage() ([]byte, error) {
	type wireMessage struct {
		itemWireBase
		Role    MessageRole  `json:"role"`
		Content []any        `json:"content"`
	}

	w := wireMessage{
		itemWireBase: itemWireBase{
			ID: item.ID, Type: item.Type, Status: item.Status,
		},
	}

	if item.Message != nil {
		w.Role = item.Message.Role

		// Build content array from either Output (assistant) or Content (user).
		if len(item.Message.Output) > 0 {
			for _, part := range item.Message.Output {
				w.Content = append(w.Content, part)
			}
		} else if len(item.Message.Content) > 0 {
			for _, part := range item.Message.Content {
				w.Content = append(w.Content, part)
			}
		}
	}

	if w.Content == nil {
		w.Content = []any{}
	}

	return json.Marshal(w)
}

// marshalFunctionCall produces the flat function_call wire format:
// {type, id, status, call_id, name, arguments}
func (item Item) marshalFunctionCall() ([]byte, error) {
	type wireFunctionCall struct {
		itemWireBase
		CallID    string `json:"call_id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}

	w := wireFunctionCall{
		itemWireBase: itemWireBase{
			ID: item.ID, Type: item.Type, Status: item.Status,
		},
	}

	if item.FunctionCall != nil {
		w.CallID = item.FunctionCall.CallID
		w.Name = item.FunctionCall.Name
		w.Arguments = item.FunctionCall.Arguments
	}

	return json.Marshal(w)
}

// marshalFunctionCallOutput produces the flat function_call_output wire format:
// {type, id, status, call_id, output}
func (item Item) marshalFunctionCallOutput() ([]byte, error) {
	type wireFunctionCallOutput struct {
		itemWireBase
		CallID string `json:"call_id"`
		Output string `json:"output"`
	}

	w := wireFunctionCallOutput{
		itemWireBase: itemWireBase{
			ID: item.ID, Type: item.Type, Status: item.Status,
		},
	}

	if item.FunctionCallOutput != nil {
		w.CallID = item.FunctionCallOutput.CallID
		w.Output = item.FunctionCallOutput.Output
	}

	return json.Marshal(w)
}

// marshalReasoning produces the reasoning wire format.
func (item Item) marshalReasoning() ([]byte, error) {
	type wireReasoning struct {
		itemWireBase
		Content          string `json:"content,omitempty"`
		EncryptedContent string `json:"encrypted_content,omitempty"`
		Summary          string `json:"summary,omitempty"`
	}

	w := wireReasoning{
		itemWireBase: itemWireBase{
			ID: item.ID, Type: item.Type, Status: item.Status,
		},
	}

	if item.Reasoning != nil {
		w.Content = item.Reasoning.Content
		w.EncryptedContent = item.Reasoning.EncryptedContent
		w.Summary = item.Reasoning.Summary
	}

	return json.Marshal(w)
}

// marshalCodeInterpreterCall produces the code_interpreter_call wire format.
func (item Item) marshalCodeInterpreterCall() ([]byte, error) {
	type wireCodeInterpreter struct {
		itemWireBase
		CodeInterpreter *CodeInterpreterCallData `json:"code_interpreter,omitempty"`
	}

	return json.Marshal(wireCodeInterpreter{
		itemWireBase:    itemWireBase{ID: item.ID, Type: item.Type, Status: item.Status},
		CodeInterpreter: item.CodeInterpreterCall,
	})
}

// UnmarshalJSON deserializes an Item from either the flat wire format
// or the internal nested format, handling both for compatibility.
func (item *Item) UnmarshalJSON(data []byte) error {
	// First parse the base fields to determine type.
	var base struct {
		ID     string     `json:"id"`
		Type   ItemType   `json:"type"`
		Status ItemStatus `json:"status"`

		// Flat wire format fields.
		Role      MessageRole        `json:"role"`
		Content   json.RawMessage    `json:"content"`
		CallID    string             `json:"call_id"`
		Name      string             `json:"name"`
		Arguments string             `json:"arguments"`
		Output    json.RawMessage    `json:"output"`

		// Extension data.
		Extension json.RawMessage    `json:"extension"`

		// Nested format fields (internal/legacy).
		Message              *MessageData              `json:"message"`
		FunctionCall         *FunctionCallData         `json:"function_call"`
		FunctionCallOutput   *FunctionCallOutputData   `json:"function_call_output"`
		Reasoning            *ReasoningData            `json:"reasoning"`
		CodeInterpreterCall  *CodeInterpreterCallData  `json:"code_interpreter"`
	}

	if err := json.Unmarshal(data, &base); err != nil {
		return err
	}

	item.ID = base.ID
	item.Type = base.Type
	item.Status = base.Status

	switch base.Type {
	case ItemTypeMessage:
		if base.Message != nil {
			// Nested format (internal).
			item.Message = base.Message
		} else if base.Role != "" {
			// Flat wire format.
			item.Message = &MessageData{Role: base.Role}
			if len(base.Content) > 0 && string(base.Content) != "[]" && string(base.Content) != "null" {
				// Parse content array into ContentPart or OutputContentPart
				// based on the role.
				if base.Role == RoleAssistant {
					var parts []OutputContentPart
					if err := json.Unmarshal(base.Content, &parts); err == nil && len(parts) > 0 {
						item.Message.Output = parts
					}
				} else {
					var parts []ContentPart
					if err := json.Unmarshal(base.Content, &parts); err == nil && len(parts) > 0 {
						item.Message.Content = parts
					}
				}
			}
		}

	case ItemTypeFunctionCall:
		if base.FunctionCall != nil {
			item.FunctionCall = base.FunctionCall
		} else if base.Name != "" || base.CallID != "" {
			item.FunctionCall = &FunctionCallData{
				Name:      base.Name,
				CallID:    base.CallID,
				Arguments: base.Arguments,
			}
		}

	case ItemTypeFunctionCallOutput:
		if base.FunctionCallOutput != nil {
			item.FunctionCallOutput = base.FunctionCallOutput
		} else if base.CallID != "" {
			outputStr := ""
			if len(base.Output) > 0 {
				// Try as string first.
				if err := json.Unmarshal(base.Output, &outputStr); err != nil {
					outputStr = string(base.Output)
				}
			}
			item.FunctionCallOutput = &FunctionCallOutputData{
				CallID: base.CallID,
				Output: outputStr,
			}
		}

	case ItemTypeReasoning:
		if base.Reasoning != nil {
			item.Reasoning = base.Reasoning
		} else {
			// Flat wire format: reasoning fields are at the top level.
			var r struct {
				Content          string `json:"content"`
				EncryptedContent string `json:"encrypted_content"`
				Summary          string `json:"summary"`
			}
			if err := json.Unmarshal(data, &r); err == nil && (r.Content != "" || r.EncryptedContent != "" || r.Summary != "") {
				item.Reasoning = &ReasoningData{
					Content:          r.Content,
					EncryptedContent: r.EncryptedContent,
					Summary:          r.Summary,
				}
			}
		}

	case ItemTypeCodeInterpreterCall:
		if base.CodeInterpreterCall != nil {
			item.CodeInterpreterCall = base.CodeInterpreterCall
		} else {
			var ci struct {
				CodeInterpreter *CodeInterpreterCallData `json:"code_interpreter"`
			}
			if err := json.Unmarshal(data, &ci); err == nil && ci.CodeInterpreter != nil {
				item.CodeInterpreterCall = ci.CodeInterpreter
			}
		}
	}

	// Preserve extension data.
	if len(base.Extension) > 0 {
		item.Extension = base.Extension
	}

	return nil
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
	Strict      bool            `json:"strict"`
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
	FrequencyPenalty   *float64                     `json:"frequency_penalty,omitempty"`
	PresencePenalty    *float64                     `json:"presence_penalty,omitempty"`
	TopLogprobs        *int                         `json:"top_logprobs,omitempty"`
	ParallelToolCalls  *bool                        `json:"parallel_tool_calls,omitempty"`
	MaxToolCalls       *int                         `json:"max_tool_calls,omitempty"`
	Metadata           map[string]any               `json:"metadata,omitempty"`
	User               string                       `json:"user,omitempty"`
	Reasoning          *ReasoningConfig             `json:"reasoning,omitempty"`
	Text               *TextConfig                  `json:"text,omitempty"`
	Include            []string                     `json:"include,omitempty"`
	StreamOptions      *StreamOptions               `json:"stream_options,omitempty"`
	Extensions         map[string]json.RawMessage   `json:"extensions,omitempty"`
}

// StreamOptions controls streaming behavior.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// ResponseStatus represents the overall status of a response.
type ResponseStatus string

const (
	ResponseStatusQueued      ResponseStatus = "queued"
	ResponseStatusInProgress  ResponseStatus = "in_progress"
	ResponseStatusCompleted   ResponseStatus = "completed"
	ResponseStatusIncomplete  ResponseStatus = "incomplete"
	ResponseStatusFailed         ResponseStatus = "failed"
	ResponseStatusCancelled      ResponseStatus = "cancelled"
	ResponseStatusRequiresAction ResponseStatus = "requires_action"
)

// Response represents the API response object returned by the Responses API.
// All fields are required by the OpenResponses schema (nullable fields use pointer types).
type Response struct {
	ID                 string                     `json:"id"`
	Object             string                     `json:"object"`
	CreatedAt          int64                      `json:"created_at"`
	CompletedAt        *int64                     `json:"completed_at"`
	Status             ResponseStatus             `json:"status"`
	IncompleteDetails  *IncompleteDetails         `json:"incomplete_details"`
	Model              string                     `json:"model"`
	PreviousResponseID *string                    `json:"previous_response_id"`
	Instructions       *string                    `json:"instructions"`
	Input              []Item                     `json:"input,omitempty"`
	Output             []Item                     `json:"output"`
	Error              *APIError                  `json:"error"`
	Tools              []ToolDefinition           `json:"tools"`
	ToolChoice         any                        `json:"tool_choice"`
	Truncation         string                     `json:"truncation"`
	ParallelToolCalls  bool                       `json:"parallel_tool_calls"`
	Text               *TextConfig                `json:"text"`
	TopP               float64                    `json:"top_p"`
	PresencePenalty    float64                    `json:"presence_penalty"`
	FrequencyPenalty   float64                    `json:"frequency_penalty"`
	TopLogprobs        int                        `json:"top_logprobs"`
	Temperature        float64                    `json:"temperature"`
	Reasoning          *ReasoningConfig           `json:"reasoning"`
	Usage              *Usage                     `json:"usage"`
	MaxOutputTokens    *int                       `json:"max_output_tokens"`
	MaxToolCalls       *int                       `json:"max_tool_calls"`
	Store              bool                       `json:"store"`
	Background         bool                       `json:"background"`
	ServiceTier        string                     `json:"service_tier"`
	Metadata           map[string]any             `json:"metadata"`
	User               string                     `json:"user,omitempty"`
	SafetyIdentifier   *string                    `json:"safety_identifier"`
	PromptCacheKey     *string                    `json:"prompt_cache_key"`
	Extensions         map[string]json.RawMessage `json:"extensions,omitempty"`
}

// IncompleteDetails provides information about why a response is incomplete.
type IncompleteDetails struct {
	Reason string `json:"reason,omitempty"`
}

// TextConfig holds text generation configuration echoed in the response.
type TextConfig struct {
	Format *TextFormat `json:"format,omitempty"`
}

// TextFormat specifies the output text format.
// For json_schema mode, the Name, Strict, and Schema fields carry
// the schema definition through the pipeline as opaque data.
type TextFormat struct {
	Type   string          `json:"type"`
	Name   string          `json:"name,omitempty"`
	Strict *bool           `json:"strict,omitempty"`
	Schema json.RawMessage `json:"schema,omitempty"`
}

// ReasoningConfig holds reasoning configuration echoed in the response.
type ReasoningConfig struct {
	Effort  *string `json:"effort"`
	Summary *string `json:"summary"`
}

// Usage holds token usage information for a response.
type Usage struct {
	InputTokens         int                  `json:"input_tokens"`
	OutputTokens        int                  `json:"output_tokens"`
	TotalTokens         int                  `json:"total_tokens"`
	InputTokensDetails  InputTokensDetails   `json:"input_tokens_details"`
	OutputTokensDetails OutputTokensDetails  `json:"output_tokens_details"`
}

// InputTokensDetails provides a breakdown of input token usage.
type InputTokensDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

// OutputTokensDetails provides a breakdown of output token usage.
type OutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}
