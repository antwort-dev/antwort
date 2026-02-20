// Package config provides unified configuration for the antwort gateway.
//
// Configuration is loaded with a layered approach:
//  1. Built-in defaults
//  2. YAML config file (discovered or explicitly specified)
//  3. Environment variable overrides (ANTWORT_ prefix)
//  4. Backward-compatible env var mapping for legacy variable names
//  5. File reference resolution (_file suffix fields)
//  6. Validation
package config

import "time"

// Config holds all configuration for the antwort gateway.
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Engine        EngineConfig        `yaml:"engine"`
	Storage       StorageConfig       `yaml:"storage"`
	Auth          AuthConfig          `yaml:"auth"`
	MCP           MCPConfig           `yaml:"mcp"`
	Observability ObservabilityConfig `yaml:"observability"`
}

// ObservabilityConfig holds monitoring and instrumentation settings.
type ObservabilityConfig struct {
	Metrics MetricsConfig `yaml:"metrics"`
}

// MetricsConfig holds Prometheus metrics endpoint settings.
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"` // default: true
	Path    string `yaml:"path"`    // default: "/metrics"
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port         int           `yaml:"port"`          // default: 8080
	ReadTimeout  time.Duration `yaml:"read_timeout"`  // default: 30s
	WriteTimeout time.Duration `yaml:"write_timeout"` // default: 120s
}

// EngineConfig holds inference engine and provider settings.
type EngineConfig struct {
	Provider     string `yaml:"provider"`      // "vllm" or "litellm", default: "vllm"
	BackendURL   string `yaml:"backend_url"`   // required
	APIKey       string `yaml:"api_key"`       // optional
	APIKeyFile   string `yaml:"api_key_file"`  // _file variant for api_key
	DefaultModel string `yaml:"default_model"` // optional
	MaxTurns     int    `yaml:"max_turns"`     // default: 10
}

// StorageConfig holds state management settings.
type StorageConfig struct {
	Type     string         `yaml:"type"`     // "memory" or "postgres", default: "memory"
	MaxSize  int            `yaml:"max_size"` // for memory store, default: 10000
	Postgres PostgresConfig `yaml:"postgres"`
}

// PostgresConfig holds PostgreSQL-specific settings.
type PostgresConfig struct {
	DSN            string `yaml:"dsn"`
	DSNFile        string `yaml:"dsn_file"`         // _file variant for dsn
	MaxConns       int32  `yaml:"max_conns"`        // default: 25
	MigrateOnStart bool   `yaml:"migrate_on_start"` // default: false
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Type    string         `yaml:"type"`     // "none", "apikey", default: "none"
	APIKeys []APIKeyConfig `yaml:"api_keys"` // API key entries for type=apikey
}

// APIKeyConfig describes a single API key entry.
type APIKeyConfig struct {
	Key         string `yaml:"key"`
	KeyFile     string `yaml:"key_file"` // _file variant for key
	Subject     string `yaml:"subject"`
	TenantID    string `yaml:"tenant_id"`
	ServiceTier string `yaml:"service_tier"`
}

// MCPConfig holds MCP (Model Context Protocol) server settings.
type MCPConfig struct {
	Servers []MCPServerConfig `yaml:"servers"`
}

// MCPServerConfig describes a single MCP server connection.
type MCPServerConfig struct {
	Name      string            `yaml:"name"`
	Transport string            `yaml:"transport"` // "sse" or "streamable-http"
	URL       string            `yaml:"url"`
	Headers   map[string]string `yaml:"headers"`
}

// Defaults returns a Config with all default values filled in.
func Defaults() Config {
	return Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 120 * time.Second,
		},
		Engine: EngineConfig{
			Provider: "vllm",
			MaxTurns: 10,
		},
		Storage: StorageConfig{
			Type:    "memory",
			MaxSize: 10000,
			Postgres: PostgresConfig{
				MaxConns: 25,
			},
		},
		Auth: AuthConfig{
			Type: "none",
		},
		Observability: ObservabilityConfig{
			Metrics: MetricsConfig{
				Enabled: true,
				Path:    "/metrics",
			},
		},
	}
}
