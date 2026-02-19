// Package litellm implements the Provider interface for LiteLLM proxy servers.
// LiteLLM exposes an OpenAI-compatible Chat Completions API, so this adapter
// delegates all HTTP communication to the shared openaicompat.Client and adds
// LiteLLM-specific model mapping support.
package litellm
