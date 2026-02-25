package openaicompat

import "encoding/json"

// Chat Completions request/response types shared across OpenAI-compatible adapters.
// These mirror the OpenAI Chat Completions API format.

// ChatCompletionRequest is the request body for /v1/chat/completions.
type ChatCompletionRequest struct {
	Model         string             `json:"model"`
	Messages      []ChatMessage      `json:"messages"`
	Tools         []ChatTool         `json:"tools,omitempty"`
	ToolChoice    any                `json:"tool_choice,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	MaxTokens     *int               `json:"max_tokens,omitempty"`
	Stop          []string           `json:"stop,omitempty"`
	N                int                `json:"n"`
	Stream           bool               `json:"stream"`
	StreamOptions    *ChatStreamOptions `json:"stream_options,omitempty"`
	FrequencyPenalty *float64           `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64           `json:"presence_penalty,omitempty"`
	TopLogprobs      *int               `json:"top_logprobs,omitempty"`
	User             string             `json:"user,omitempty"`
	ResponseFormat   any                `json:"response_format,omitempty"`
}

// ChatStreamOptions controls streaming behavior.
type ChatStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ChatMessage represents a message in the Chat Completions format.
type ChatMessage struct {
	Role             string         `json:"role"`
	Content          any            `json:"content"`
	ToolCalls        []ChatToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	Name             string         `json:"name,omitempty"`
	ReasoningContent *string        `json:"reasoning_content,omitempty"`
}

// ChatToolCall represents a tool call in an assistant message.
type ChatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ChatFunctionCall `json:"function"`
}

// ChatFunctionCall holds function name and arguments.
type ChatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatTool represents a tool definition.
type ChatTool struct {
	Type     string          `json:"type"`
	Function ChatFunctionDef `json:"function"`
}

// ChatFunctionDef is a function definition for a tool.
type ChatFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ChatCompletionResponse is the non-streaming response from /v1/chat/completions.
type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *ChatUsage   `json:"usage,omitempty"`
}

// ChatChoice represents one completion choice.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatUsage holds token usage from the Chat Completions API.
type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionChunk is a single SSE chunk in a streaming response.
type ChatCompletionChunk struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Model   string            `json:"model"`
	Choices []ChatChunkChoice `json:"choices"`
	Usage   *ChatUsage        `json:"usage,omitempty"`
}

// ChatChunkChoice represents a streaming choice delta.
type ChatChunkChoice struct {
	Index        int            `json:"index"`
	Delta        ChatChunkDelta `json:"delta"`
	FinishReason *string        `json:"finish_reason"`
}

// ChatChunkDelta holds incremental content in a streaming chunk.
type ChatChunkDelta struct {
	Role             string              `json:"role,omitempty"`
	Content          *string             `json:"content,omitempty"`
	ToolCalls        []ChatChunkToolCall `json:"tool_calls,omitempty"`
	ReasoningContent *string             `json:"reasoning_content,omitempty"`
}

// ChatChunkToolCall represents an incremental tool call in a streaming chunk.
type ChatChunkToolCall struct {
	Index    int                    `json:"index"`
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Function ChatChunkFunctionCall  `json:"function"`
}

// ChatChunkFunctionCall holds incremental function call data.
type ChatChunkFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ChatErrorResponse is the error format returned by Chat Completions backends.
type ChatErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

// ChatModelsResponse is the response from /v1/models.
type ChatModelsResponse struct {
	Object string      `json:"object"`
	Data   []ChatModel `json:"data"`
}

// ChatModel represents a model in the /v1/models response.
type ChatModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}
