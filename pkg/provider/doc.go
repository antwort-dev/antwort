// Package provider defines the protocol-agnostic interface for LLM inference
// backends. Each adapter implementation (e.g., vllm) handles its own backend
// protocol translation internally. The interface operates on Antwort's own
// types (ProviderRequest, ProviderResponse, ProviderEvent), keeping backend
// protocol details invisible to the engine.
package provider
