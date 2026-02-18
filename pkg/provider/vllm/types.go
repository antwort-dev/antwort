package vllm

import "encoding/json"

// Chat Completions request/response types (internal to the vLLM adapter).
// These mirror the OpenAI Chat Completions API format.

// chatCompletionRequest is the request body for /v1/chat/completions.
type chatCompletionRequest struct {
	Model         string                `json:"model"`
	Messages      []chatMessage         `json:"messages"`
	Tools         []chatTool            `json:"tools,omitempty"`
	ToolChoice    any                   `json:"tool_choice,omitempty"`
	Temperature   *float64              `json:"temperature,omitempty"`
	TopP          *float64              `json:"top_p,omitempty"`
	MaxTokens     *int                  `json:"max_tokens,omitempty"`
	Stop          []string              `json:"stop,omitempty"`
	N             int                   `json:"n"`
	Stream        bool                  `json:"stream"`
	StreamOptions *chatStreamOptions    `json:"stream_options,omitempty"`
}

// chatStreamOptions controls streaming behavior.
type chatStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// chatMessage represents a message in the Chat Completions format.
type chatMessage struct {
	Role             string         `json:"role"`
	Content          any            `json:"content"`
	ToolCalls        []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string         `json:"tool_call_id,omitempty"`
	Name             string         `json:"name,omitempty"`
	ReasoningContent *string        `json:"reasoning_content,omitempty"`
}

// chatToolCall represents a tool call in an assistant message.
type chatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function chatFunctionCall `json:"function"`
}

// chatFunctionCall holds function name and arguments.
type chatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// chatTool represents a tool definition.
type chatTool struct {
	Type     string         `json:"type"`
	Function chatFunctionDef `json:"function"`
}

// chatFunctionDef is a function definition for a tool.
type chatFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// chatCompletionResponse is the non-streaming response from /v1/chat/completions.
type chatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   *chatUsage   `json:"usage,omitempty"`
}

// chatChoice represents one completion choice.
type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// chatUsage holds token usage from the Chat Completions API.
type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// chatCompletionChunk is a single SSE chunk in a streaming response.
type chatCompletionChunk struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Model   string            `json:"model"`
	Choices []chatChunkChoice `json:"choices"`
	Usage   *chatUsage        `json:"usage,omitempty"`
}

// chatChunkChoice represents a streaming choice delta.
type chatChunkChoice struct {
	Index        int              `json:"index"`
	Delta        chatChunkDelta   `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

// chatChunkDelta holds incremental content in a streaming chunk.
type chatChunkDelta struct {
	Role             string               `json:"role,omitempty"`
	Content          *string              `json:"content,omitempty"`
	ToolCalls        []chatChunkToolCall  `json:"tool_calls,omitempty"`
	ReasoningContent *string              `json:"reasoning_content,omitempty"`
}

// chatChunkToolCall represents an incremental tool call in a streaming chunk.
type chatChunkToolCall struct {
	Index    int                    `json:"index"`
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Function chatChunkFunctionCall  `json:"function"`
}

// chatChunkFunctionCall holds incremental function call data.
type chatChunkFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// chatErrorResponse is the error format returned by Chat Completions backends.
type chatErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

// chatModelsResponse is the response from /v1/models.
type chatModelsResponse struct {
	Object string      `json:"object"`
	Data   []chatModel `json:"data"`
}

// chatModel represents a model in the /v1/models response.
type chatModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}
