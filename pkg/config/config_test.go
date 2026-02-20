package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.Server.Port != 8080 {
		t.Errorf("default server.port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("default server.read_timeout = %v, want 30s", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 120*time.Second {
		t.Errorf("default server.write_timeout = %v, want 120s", cfg.Server.WriteTimeout)
	}
	if cfg.Engine.Provider != "vllm" {
		t.Errorf("default engine.provider = %q, want \"vllm\"", cfg.Engine.Provider)
	}
	if cfg.Engine.MaxTurns != 10 {
		t.Errorf("default engine.max_turns = %d, want 10", cfg.Engine.MaxTurns)
	}
	if cfg.Storage.Type != "memory" {
		t.Errorf("default storage.type = %q, want \"memory\"", cfg.Storage.Type)
	}
	if cfg.Storage.MaxSize != 10000 {
		t.Errorf("default storage.max_size = %d, want 10000", cfg.Storage.MaxSize)
	}
	if cfg.Storage.Postgres.MaxConns != 25 {
		t.Errorf("default storage.postgres.max_conns = %d, want 25", cfg.Storage.Postgres.MaxConns)
	}
	if cfg.Auth.Type != "none" {
		t.Errorf("default auth.type = %q, want \"none\"", cfg.Auth.Type)
	}
}

func TestLoadFromYAML(t *testing.T) {
	yamlContent := `
server:
  port: 9090
  read_timeout: 60s
  write_timeout: 180s
engine:
  provider: litellm
  backend_url: http://localhost:4000
  api_key: sk-test-key
  default_model: gpt-4
  max_turns: 5
storage:
  type: postgres
  max_size: 5000
  postgres:
    dsn: "postgres://user:pass@localhost/db"
    max_conns: 50
    migrate_on_start: true
auth:
  type: apikey
  api_keys:
    - key: sk-key-1
      subject: alice
      tenant_id: org-1
      service_tier: premium
    - key: sk-key-2
      subject: bob
mcp:
  servers:
    - name: my-server
      transport: streamable-http
      url: http://localhost:3000/mcp
      headers:
        Authorization: "Bearer tok-123"
`

	tmpFile := writeTemp(t, "config-*.yaml", yamlContent)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Server
	if cfg.Server.Port != 9090 {
		t.Errorf("server.port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 60*time.Second {
		t.Errorf("server.read_timeout = %v, want 60s", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 180*time.Second {
		t.Errorf("server.write_timeout = %v, want 180s", cfg.Server.WriteTimeout)
	}

	// Engine
	if cfg.Engine.Provider != "litellm" {
		t.Errorf("engine.provider = %q, want \"litellm\"", cfg.Engine.Provider)
	}
	if cfg.Engine.BackendURL != "http://localhost:4000" {
		t.Errorf("engine.backend_url = %q, want \"http://localhost:4000\"", cfg.Engine.BackendURL)
	}
	if cfg.Engine.APIKey != "sk-test-key" {
		t.Errorf("engine.api_key = %q, want \"sk-test-key\"", cfg.Engine.APIKey)
	}
	if cfg.Engine.DefaultModel != "gpt-4" {
		t.Errorf("engine.default_model = %q, want \"gpt-4\"", cfg.Engine.DefaultModel)
	}
	if cfg.Engine.MaxTurns != 5 {
		t.Errorf("engine.max_turns = %d, want 5", cfg.Engine.MaxTurns)
	}

	// Storage
	if cfg.Storage.Type != "postgres" {
		t.Errorf("storage.type = %q, want \"postgres\"", cfg.Storage.Type)
	}
	if cfg.Storage.MaxSize != 5000 {
		t.Errorf("storage.max_size = %d, want 5000", cfg.Storage.MaxSize)
	}
	if cfg.Storage.Postgres.DSN != "postgres://user:pass@localhost/db" {
		t.Errorf("storage.postgres.dsn = %q, want correct DSN", cfg.Storage.Postgres.DSN)
	}
	if cfg.Storage.Postgres.MaxConns != 50 {
		t.Errorf("storage.postgres.max_conns = %d, want 50", cfg.Storage.Postgres.MaxConns)
	}
	if !cfg.Storage.Postgres.MigrateOnStart {
		t.Error("storage.postgres.migrate_on_start = false, want true")
	}

	// Auth
	if cfg.Auth.Type != "apikey" {
		t.Errorf("auth.type = %q, want \"apikey\"", cfg.Auth.Type)
	}
	if len(cfg.Auth.APIKeys) != 2 {
		t.Fatalf("auth.api_keys length = %d, want 2", len(cfg.Auth.APIKeys))
	}
	if cfg.Auth.APIKeys[0].Key != "sk-key-1" {
		t.Errorf("auth.api_keys[0].key = %q, want \"sk-key-1\"", cfg.Auth.APIKeys[0].Key)
	}
	if cfg.Auth.APIKeys[0].Subject != "alice" {
		t.Errorf("auth.api_keys[0].subject = %q, want \"alice\"", cfg.Auth.APIKeys[0].Subject)
	}
	if cfg.Auth.APIKeys[0].TenantID != "org-1" {
		t.Errorf("auth.api_keys[0].tenant_id = %q, want \"org-1\"", cfg.Auth.APIKeys[0].TenantID)
	}
	if cfg.Auth.APIKeys[0].ServiceTier != "premium" {
		t.Errorf("auth.api_keys[0].service_tier = %q, want \"premium\"", cfg.Auth.APIKeys[0].ServiceTier)
	}

	// MCP
	if len(cfg.MCP.Servers) != 1 {
		t.Fatalf("mcp.servers length = %d, want 1", len(cfg.MCP.Servers))
	}
	if cfg.MCP.Servers[0].Name != "my-server" {
		t.Errorf("mcp.servers[0].name = %q, want \"my-server\"", cfg.MCP.Servers[0].Name)
	}
	if cfg.MCP.Servers[0].Transport != "streamable-http" {
		t.Errorf("mcp.servers[0].transport = %q, want \"streamable-http\"", cfg.MCP.Servers[0].Transport)
	}
	if cfg.MCP.Servers[0].URL != "http://localhost:3000/mcp" {
		t.Errorf("mcp.servers[0].url = %q, want \"http://localhost:3000/mcp\"", cfg.MCP.Servers[0].URL)
	}
	if cfg.MCP.Servers[0].Headers["Authorization"] != "Bearer tok-123" {
		t.Errorf("mcp.servers[0].headers[Authorization] = %q, want \"Bearer tok-123\"", cfg.MCP.Servers[0].Headers["Authorization"])
	}
}

func TestEnvOverride(t *testing.T) {
	// Create a YAML config with specific values.
	yamlContent := `
engine:
  backend_url: http://from-yaml:8000
  provider: vllm
  default_model: yaml-model
server:
  port: 9090
storage:
  type: memory
  max_size: 5000
`
	tmpFile := writeTemp(t, "config-*.yaml", yamlContent)

	// Set env vars that should override the YAML values.
	t.Setenv("ANTWORT_BACKEND_URL", "http://from-env:8000")
	t.Setenv("ANTWORT_MODEL", "env-model")
	t.Setenv("ANTWORT_PORT", "7070")
	t.Setenv("ANTWORT_PROVIDER", "litellm")
	t.Setenv("ANTWORT_STORAGE", "memory")
	t.Setenv("ANTWORT_STORAGE_SIZE", "2000")

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Engine.BackendURL != "http://from-env:8000" {
		t.Errorf("engine.backend_url = %q, want env override", cfg.Engine.BackendURL)
	}
	if cfg.Engine.DefaultModel != "env-model" {
		t.Errorf("engine.default_model = %q, want env override", cfg.Engine.DefaultModel)
	}
	if cfg.Server.Port != 7070 {
		t.Errorf("server.port = %d, want env override 7070", cfg.Server.Port)
	}
	if cfg.Engine.Provider != "litellm" {
		t.Errorf("engine.provider = %q, want env override \"litellm\"", cfg.Engine.Provider)
	}
	if cfg.Storage.MaxSize != 2000 {
		t.Errorf("storage.max_size = %d, want env override 2000", cfg.Storage.MaxSize)
	}
}

func TestBackwardCompatEnvVars(t *testing.T) {
	// No config file, only env vars.
	t.Setenv("ANTWORT_BACKEND_URL", "http://legacy-backend:8000")
	t.Setenv("ANTWORT_MODEL", "legacy-model")
	t.Setenv("ANTWORT_PORT", "3000")
	t.Setenv("ANTWORT_PROVIDER", "litellm")
	t.Setenv("ANTWORT_STORAGE", "memory")
	t.Setenv("ANTWORT_STORAGE_SIZE", "500")
	t.Setenv("ANTWORT_AUTH_TYPE", "apikey")
	t.Setenv("ANTWORT_API_KEYS", `[{"key":"sk-legacy","subject":"legacy-user","tenant_id":"org-legacy","service_tier":"standard"}]`)
	t.Setenv("ANTWORT_MCP_SERVERS", `[{"name":"legacy-mcp","transport":"sse","url":"http://mcp:3000"}]`)

	// Use a nonexistent config path to skip file loading.
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Engine.BackendURL != "http://legacy-backend:8000" {
		t.Errorf("engine.backend_url = %q, want legacy env value", cfg.Engine.BackendURL)
	}
	if cfg.Engine.DefaultModel != "legacy-model" {
		t.Errorf("engine.default_model = %q, want legacy env value", cfg.Engine.DefaultModel)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("server.port = %d, want 3000", cfg.Server.Port)
	}
	if cfg.Engine.Provider != "litellm" {
		t.Errorf("engine.provider = %q, want \"litellm\"", cfg.Engine.Provider)
	}
	if cfg.Storage.Type != "memory" {
		t.Errorf("storage.type = %q, want \"memory\"", cfg.Storage.Type)
	}
	if cfg.Storage.MaxSize != 500 {
		t.Errorf("storage.max_size = %d, want 500", cfg.Storage.MaxSize)
	}
	if cfg.Auth.Type != "apikey" {
		t.Errorf("auth.type = %q, want \"apikey\"", cfg.Auth.Type)
	}
	if len(cfg.Auth.APIKeys) != 1 {
		t.Fatalf("auth.api_keys length = %d, want 1", len(cfg.Auth.APIKeys))
	}
	if cfg.Auth.APIKeys[0].Key != "sk-legacy" {
		t.Errorf("auth.api_keys[0].key = %q, want \"sk-legacy\"", cfg.Auth.APIKeys[0].Key)
	}
	if cfg.Auth.APIKeys[0].Subject != "legacy-user" {
		t.Errorf("auth.api_keys[0].subject = %q, want \"legacy-user\"", cfg.Auth.APIKeys[0].Subject)
	}
	if len(cfg.MCP.Servers) != 1 {
		t.Fatalf("mcp.servers length = %d, want 1", len(cfg.MCP.Servers))
	}
	if cfg.MCP.Servers[0].Name != "legacy-mcp" {
		t.Errorf("mcp.servers[0].name = %q, want \"legacy-mcp\"", cfg.MCP.Servers[0].Name)
	}
}

func TestFileReference(t *testing.T) {
	// Write a secret file.
	secretFile := writeTemp(t, "secret-*.txt", "  sk-from-file-123  \n")

	yamlContent := `
engine:
  backend_url: http://localhost:8000
  api_key_file: ` + secretFile + `
`
	tmpFile := writeTemp(t, "config-*.yaml", yamlContent)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Engine.APIKey != "sk-from-file-123" {
		t.Errorf("engine.api_key = %q, want \"sk-from-file-123\" (from file, trimmed)", cfg.Engine.APIKey)
	}
}

func TestFileReferenceForAPIKeys(t *testing.T) {
	// Write a key file.
	keyFile := writeTemp(t, "apikey-*.txt", "  sk-key-from-file  \n")

	yamlContent := `
engine:
  backend_url: http://localhost:8000
auth:
  type: apikey
  api_keys:
    - key_file: ` + keyFile + `
      subject: file-user
`
	tmpFile := writeTemp(t, "config-*.yaml", yamlContent)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(cfg.Auth.APIKeys) != 1 {
		t.Fatalf("auth.api_keys length = %d, want 1", len(cfg.Auth.APIKeys))
	}
	if cfg.Auth.APIKeys[0].Key != "sk-key-from-file" {
		t.Errorf("auth.api_keys[0].key = %q, want \"sk-key-from-file\"", cfg.Auth.APIKeys[0].Key)
	}
}

func TestFileReferencePostgresDSN(t *testing.T) {
	dsnFile := writeTemp(t, "dsn-*.txt", "  postgres://user:pass@db:5432/app  \n")

	yamlContent := `
engine:
  backend_url: http://localhost:8000
storage:
  type: postgres
  postgres:
    dsn_file: ` + dsnFile + `
`
	tmpFile := writeTemp(t, "config-*.yaml", yamlContent)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Storage.Postgres.DSN != "postgres://user:pass@db:5432/app" {
		t.Errorf("storage.postgres.dsn = %q, want DSN from file", cfg.Storage.Postgres.DSN)
	}
}

func TestFileDiscovery(t *testing.T) {
	// Test 1: Explicit path.
	yamlContent := `
engine:
  backend_url: http://explicit:8000
`
	tmpFile := writeTemp(t, "config-*.yaml", yamlContent)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load(explicit) error: %v", err)
	}
	if cfg.Engine.BackendURL != "http://explicit:8000" {
		t.Errorf("explicit path: backend_url = %q, want explicit value", cfg.Engine.BackendURL)
	}

	// Test 2: ANTWORT_CONFIG env var.
	envFile := writeTemp(t, "envconfig-*.yaml", `
engine:
  backend_url: http://env-config:8000
`)
	t.Setenv("ANTWORT_CONFIG", envFile)

	cfg, err = Load("")
	if err != nil {
		t.Fatalf("Load(ANTWORT_CONFIG) error: %v", err)
	}
	if cfg.Engine.BackendURL != "http://env-config:8000" {
		t.Errorf("ANTWORT_CONFIG: backend_url = %q, want env config value", cfg.Engine.BackendURL)
	}

	// Test 3: No file, no env config, uses defaults + env overrides.
	t.Setenv("ANTWORT_CONFIG", "")
	t.Setenv("ANTWORT_BACKEND_URL", "http://defaults-only:8000")

	cfg, err = Load("")
	if err != nil {
		t.Fatalf("Load(no file) error: %v", err)
	}
	if cfg.Engine.BackendURL != "http://defaults-only:8000" {
		t.Errorf("no file: backend_url = %q, want env override", cfg.Engine.BackendURL)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr string
	}{
		{
			name: "missing backend_url",
			modify: func(c *Config) {
				c.Engine.BackendURL = ""
			},
			wantErr: "engine.backend_url is required",
		},
		{
			name: "invalid port",
			modify: func(c *Config) {
				c.Engine.BackendURL = "http://localhost:8000"
				c.Server.Port = 0
			},
			wantErr: "server.port must be > 0",
		},
		{
			name: "invalid storage type",
			modify: func(c *Config) {
				c.Engine.BackendURL = "http://localhost:8000"
				c.Storage.Type = "redis"
			},
			wantErr: "storage.type must be",
		},
		{
			name: "postgres without DSN",
			modify: func(c *Config) {
				c.Engine.BackendURL = "http://localhost:8000"
				c.Storage.Type = "postgres"
				c.Storage.Postgres.DSN = ""
				c.Storage.Postgres.DSNFile = ""
			},
			wantErr: "storage.postgres.dsn",
		},
		{
			name: "invalid auth type",
			modify: func(c *Config) {
				c.Engine.BackendURL = "http://localhost:8000"
				c.Auth.Type = "oauth2"
			},
			wantErr: "auth.type must be",
		},
		{
			name: "invalid provider",
			modify: func(c *Config) {
				c.Engine.BackendURL = "http://localhost:8000"
				c.Engine.Provider = "openai"
			},
			wantErr: "engine.provider must be",
		},
		{
			name: "valid config",
			modify: func(c *Config) {
				c.Engine.BackendURL = "http://localhost:8000"
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			tt.modify(&cfg)
			err := cfg.Validate()

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("Validate() expected error containing %q, got nil", tt.wantErr)
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestEnvOverrideAPIKey(t *testing.T) {
	yamlContent := `
engine:
  backend_url: http://localhost:8000
`
	tmpFile := writeTemp(t, "config-*.yaml", yamlContent)

	t.Setenv("ANTWORT_API_KEY", "sk-env-api-key")

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Engine.APIKey != "sk-env-api-key" {
		t.Errorf("engine.api_key = %q, want \"sk-env-api-key\"", cfg.Engine.APIKey)
	}
}

func TestFileReferenceDoesNotOverrideExplicitValue(t *testing.T) {
	secretFile := writeTemp(t, "secret-*.txt", "sk-from-file")

	yamlContent := `
engine:
  backend_url: http://localhost:8000
  api_key: sk-explicit
  api_key_file: ` + secretFile + `
`
	tmpFile := writeTemp(t, "config-*.yaml", yamlContent)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// When both api_key and api_key_file are set, the explicit value takes precedence.
	if cfg.Engine.APIKey != "sk-explicit" {
		t.Errorf("engine.api_key = %q, want \"sk-explicit\" (explicit value should win over file)", cfg.Engine.APIKey)
	}
}

func TestYAMLDefaultsMerge(t *testing.T) {
	// A minimal YAML that only sets backend_url.
	// All other fields should retain defaults.
	yamlContent := `
engine:
  backend_url: http://localhost:8000
`
	tmpFile := writeTemp(t, "config-*.yaml", yamlContent)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Check that defaults are preserved for unset fields.
	if cfg.Server.Port != 8080 {
		t.Errorf("server.port = %d, want default 8080", cfg.Server.Port)
	}
	if cfg.Engine.Provider != "vllm" {
		t.Errorf("engine.provider = %q, want default \"vllm\"", cfg.Engine.Provider)
	}
	if cfg.Storage.Type != "memory" {
		t.Errorf("storage.type = %q, want default \"memory\"", cfg.Storage.Type)
	}
	if cfg.Engine.MaxTurns != 10 {
		t.Errorf("engine.max_turns = %d, want default 10", cfg.Engine.MaxTurns)
	}
}

// writeTemp creates a temporary file with the given content and returns its path.
// The file is automatically cleaned up when the test finishes.
func writeTemp(t *testing.T, pattern, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, pattern)

	// Replace * in pattern with a fixed string for predictable file names.
	// os.CreateTemp handles this, but we use a simpler approach for clarity.
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	path = f.Name()

	if _, err := f.WriteString(content); err != nil {
		f.Close()
		t.Fatalf("writing temp file: %v", err)
	}
	f.Close()

	return path
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
