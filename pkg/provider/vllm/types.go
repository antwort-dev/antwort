package vllm

import "github.com/rhuss/antwort/pkg/provider/openaicompat"

// Chat Completions request/response types (internal to the vLLM adapter).
// These are type aliases to the shared openaicompat package, preserving
// the original unexported names for backward compatibility with tests.

type chatCompletionRequest = openaicompat.ChatCompletionRequest
type chatStreamOptions = openaicompat.ChatStreamOptions
type chatMessage = openaicompat.ChatMessage
type chatToolCall = openaicompat.ChatToolCall
type chatFunctionCall = openaicompat.ChatFunctionCall
type chatTool = openaicompat.ChatTool
type chatFunctionDef = openaicompat.ChatFunctionDef
type chatCompletionResponse = openaicompat.ChatCompletionResponse
type chatChoice = openaicompat.ChatChoice
type chatUsage = openaicompat.ChatUsage
type chatCompletionChunk = openaicompat.ChatCompletionChunk
type chatChunkChoice = openaicompat.ChatChunkChoice
type chatChunkDelta = openaicompat.ChatChunkDelta
type chatChunkToolCall = openaicompat.ChatChunkToolCall
type chatChunkFunctionCall = openaicompat.ChatChunkFunctionCall
type chatErrorResponse = openaicompat.ChatErrorResponse
type chatModelsResponse = openaicompat.ChatModelsResponse
type chatModel = openaicompat.ChatModel
