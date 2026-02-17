package vllm

import "time"

// Config holds configuration for the vLLM provider adapter.
type Config struct {
	// BaseURL is the vLLM server URL (e.g., "http://localhost:8000").
	BaseURL string

	// APIKey for vLLM authentication (optional).
	APIKey string

	// Timeout for individual HTTP requests. Defaults to 120s.
	Timeout time.Duration

	// MaxRetries for transient failures. Defaults to 0 (no retries).
	MaxRetries int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(baseURL string) Config {
	return Config{
		BaseURL:    baseURL,
		Timeout:    120 * time.Second,
		MaxRetries: 0,
	}
}
