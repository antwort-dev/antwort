package litellm

import "time"

// Config holds configuration for the LiteLLM provider adapter.
type Config struct {
	// BaseURL is the LiteLLM proxy URL (e.g., "http://localhost:4000").
	BaseURL string

	// APIKey for LiteLLM authentication (optional).
	APIKey string

	// Timeout for individual HTTP requests. Defaults to 120s.
	Timeout time.Duration

	// ModelMapping maps requested model names to LiteLLM model identifiers.
	// For example: {"gpt-4": "openai/gpt-4", "claude": "anthropic/claude-3-opus"}.
	// If a model is not in the map, it is passed through unchanged.
	ModelMapping map[string]string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(baseURL string) Config {
	return Config{
		BaseURL: baseURL,
		Timeout: 120 * time.Second,
	}
}
