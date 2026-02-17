// Package vllm implements the Provider interface for vLLM and any
// OpenAI-compatible Chat Completions backend. It translates between
// Antwort's provider types and the /v1/chat/completions HTTP API,
// handling request serialization, response parsing, SSE chunk streaming,
// tool call argument buffering, and error mapping.
package vllm
