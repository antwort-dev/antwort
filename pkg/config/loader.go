package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load loads configuration from a layered set of sources.
//
// The loading order is:
//  1. Built-in defaults
//  2. YAML config file (explicit path, ANTWORT_CONFIG env, ./config.yaml, /etc/antwort/config.yaml)
//  3. Backward-compatible environment variable mapping
//  4. File reference resolution (_file suffix)
//  5. Validation
func Load(configPath string) (*Config, error) {
	// Start with defaults.
	cfg := Defaults()

	// Discover and load YAML config file.
	filePath := discoverConfigFile(configPath)
	if filePath != "" {
		if err := loadYAMLFile(filePath, &cfg); err != nil {
			return nil, fmt.Errorf("loading config file %s: %w", filePath, err)
		}
	}

	// Apply backward-compatible environment variable overrides.
	applyEnvOverrides(&cfg)

	// Resolve _file references.
	if err := resolveFileReferences(&cfg); err != nil {
		return nil, fmt.Errorf("resolving file references: %w", err)
	}

	// Validate.
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return &cfg, nil
}

// discoverConfigFile finds the config file path using the discovery order:
// 1. Explicit configPath argument
// 2. ANTWORT_CONFIG environment variable
// 3. ./config.yaml in the current directory
// 4. /etc/antwort/config.yaml
//
// Returns empty string if no config file is found.
func discoverConfigFile(configPath string) string {
	// Explicit path takes priority.
	if configPath != "" {
		return configPath
	}

	// Check ANTWORT_CONFIG env var.
	if envPath := os.Getenv("ANTWORT_CONFIG"); envPath != "" {
		return envPath
	}

	// Check common locations.
	candidates := []string{
		"config.yaml",
		"/etc/antwort/config.yaml",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// loadYAMLFile reads and parses a YAML file into the Config struct.
// Fields not present in the YAML retain their current (default) values.
func loadYAMLFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

// applyEnvOverrides maps environment variables to config fields.
// This provides backward compatibility with the legacy ANTWORT_* env vars
// and also supports the new structured env var names.
func applyEnvOverrides(cfg *Config) {
	// Legacy env var mappings.
	if v := os.Getenv("ANTWORT_BACKEND_URL"); v != "" {
		cfg.Engine.BackendURL = v
	}
	if v := os.Getenv("ANTWORT_MODEL"); v != "" {
		cfg.Engine.DefaultModel = v
	}
	if v := os.Getenv("ANTWORT_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("ANTWORT_PROVIDER"); v != "" {
		cfg.Engine.Provider = v
	}
	if v := os.Getenv("ANTWORT_API_KEY"); v != "" {
		cfg.Engine.APIKey = v
	}
	if v := os.Getenv("ANTWORT_STORAGE"); v != "" {
		cfg.Storage.Type = v
	}
	if v := os.Getenv("ANTWORT_STORAGE_SIZE"); v != "" {
		if size, err := strconv.Atoi(v); err == nil {
			cfg.Storage.MaxSize = size
		}
	}
	if v := os.Getenv("ANTWORT_AUTH_TYPE"); v != "" {
		cfg.Auth.Type = v
	}

	// ANTWORT_API_KEYS: JSON array of API key configs.
	if v := os.Getenv("ANTWORT_API_KEYS"); v != "" {
		keys, err := parseAPIKeysJSON(v)
		if err == nil && len(keys) > 0 {
			cfg.Auth.APIKeys = keys
		}
	}

	// ANTWORT_MCP_SERVERS: JSON array of MCP server configs.
	if v := os.Getenv("ANTWORT_MCP_SERVERS"); v != "" {
		servers, err := parseMCPServersJSON(v)
		if err == nil && len(servers) > 0 {
			cfg.MCP.Servers = servers
		}
	}
}

// parseAPIKeysJSON parses a JSON array of API key configurations.
func parseAPIKeysJSON(jsonStr string) ([]APIKeyConfig, error) {
	var keys []APIKeyConfig
	if err := json.Unmarshal([]byte(jsonStr), &keys); err != nil {
		return nil, fmt.Errorf("parsing API keys JSON: %w", err)
	}
	return keys, nil
}

// parseMCPServersJSON parses a JSON array of MCP server configurations.
func parseMCPServersJSON(jsonStr string) ([]MCPServerConfig, error) {
	var servers []MCPServerConfig
	if err := json.Unmarshal([]byte(jsonStr), &servers); err != nil {
		return nil, fmt.Errorf("parsing MCP servers JSON: %w", err)
	}
	return servers, nil
}

// resolveFileReferences reads _file fields and populates the corresponding value fields.
// For each field ending in _file, if the value field is empty and the file field is set,
// the file is read, whitespace is trimmed, and the value field is populated.
func resolveFileReferences(cfg *Config) error {
	// engine.api_key_file -> engine.api_key
	if cfg.Engine.APIKeyFile != "" && cfg.Engine.APIKey == "" {
		val, err := readSecretFile(cfg.Engine.APIKeyFile)
		if err != nil {
			return fmt.Errorf("engine.api_key_file: %w", err)
		}
		cfg.Engine.APIKey = val
	}

	// storage.postgres.dsn_file -> storage.postgres.dsn
	if cfg.Storage.Postgres.DSNFile != "" && cfg.Storage.Postgres.DSN == "" {
		val, err := readSecretFile(cfg.Storage.Postgres.DSNFile)
		if err != nil {
			return fmt.Errorf("storage.postgres.dsn_file: %w", err)
		}
		cfg.Storage.Postgres.DSN = val
	}

	// auth.api_keys[*].key_file -> auth.api_keys[*].key
	for i := range cfg.Auth.APIKeys {
		if cfg.Auth.APIKeys[i].KeyFile != "" && cfg.Auth.APIKeys[i].Key == "" {
			val, err := readSecretFile(cfg.Auth.APIKeys[i].KeyFile)
			if err != nil {
				return fmt.Errorf("auth.api_keys[%d].key_file: %w", i, err)
			}
			cfg.Auth.APIKeys[i].Key = val
		}
	}

	// mcp.servers[*].auth.client_id_file -> mcp.servers[*].auth.client_id
	// mcp.servers[*].auth.client_secret_file -> mcp.servers[*].auth.client_secret
	for i := range cfg.MCP.Servers {
		if cfg.MCP.Servers[i].Auth.ClientIDFile != "" && cfg.MCP.Servers[i].Auth.ClientID == "" {
			val, err := readSecretFile(cfg.MCP.Servers[i].Auth.ClientIDFile)
			if err != nil {
				return fmt.Errorf("mcp.servers[%d].auth.client_id_file: %w", i, err)
			}
			cfg.MCP.Servers[i].Auth.ClientID = val
		}
		if cfg.MCP.Servers[i].Auth.ClientSecretFile != "" && cfg.MCP.Servers[i].Auth.ClientSecret == "" {
			val, err := readSecretFile(cfg.MCP.Servers[i].Auth.ClientSecretFile)
			if err != nil {
				return fmt.Errorf("mcp.servers[%d].auth.client_secret_file: %w", i, err)
			}
			cfg.MCP.Servers[i].Auth.ClientSecret = val
		}
	}

	return nil
}

// readSecretFile reads a file and returns its content with surrounding whitespace trimmed.
func readSecretFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
