package config

import (
	"errors"
	"fmt"
)

// Validate checks the configuration for required fields and valid values.
// Returns an error with a descriptive field path on failure.
func (c *Config) Validate() error {
	var errs []error

	// engine.backend_url is required.
	if c.Engine.BackendURL == "" {
		errs = append(errs, fmt.Errorf("engine.backend_url is required"))
	}

	// server.port must be positive.
	if c.Server.Port <= 0 {
		errs = append(errs, fmt.Errorf("server.port must be > 0, got %d", c.Server.Port))
	}

	// storage.type must be a known value.
	switch c.Storage.Type {
	case "memory", "postgres":
		// valid
	default:
		errs = append(errs, fmt.Errorf("storage.type must be \"memory\" or \"postgres\", got %q", c.Storage.Type))
	}

	// If storage.type is "postgres", DSN or DSNFile must be set.
	if c.Storage.Type == "postgres" {
		if c.Storage.Postgres.DSN == "" && c.Storage.Postgres.DSNFile == "" {
			errs = append(errs, fmt.Errorf("storage.postgres.dsn or storage.postgres.dsn_file is required when storage.type is \"postgres\""))
		}
	}

	// auth.type must be a known value.
	switch c.Auth.Type {
	case "none", "apikey", "jwt":
		// valid
	default:
		errs = append(errs, fmt.Errorf("auth.type must be \"none\", \"apikey\", or \"jwt\", got %q", c.Auth.Type))
	}

	// engine.provider must be a known value if set.
	switch c.Engine.Provider {
	case "vllm", "litellm", "":
		// valid
	default:
		errs = append(errs, fmt.Errorf("engine.provider must be \"vllm\" or \"litellm\", got %q", c.Engine.Provider))
	}

	return errors.Join(errs...)
}
