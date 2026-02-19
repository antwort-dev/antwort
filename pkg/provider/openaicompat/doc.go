// Package openaicompat provides shared translation code for any OpenAI-compatible
// Chat Completions backend. It handles request serialization, response parsing,
// SSE chunk streaming, tool call argument buffering, and error mapping.
//
// Provider adapters (vLLM, LiteLLM, etc.) embed the Client from this package
// and delegate their Complete/Stream/ListModels calls to it.
package openaicompat
