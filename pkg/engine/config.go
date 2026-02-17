package engine

// Config holds configuration for the core engine.
type Config struct {
	// DefaultModel is used when the request omits the model field.
	// Empty string means a model is always required in the request.
	DefaultModel string
}
