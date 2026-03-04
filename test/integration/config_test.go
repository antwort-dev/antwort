package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rhuss/antwort/pkg/config"
)

func TestConfigEnvOverrides(t *testing.T) {
	// Create a minimal YAML config file with a backend_url to pass validation.
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	yamlContent := `
engine:
  backend_url: "http://localhost:9999"
  default_model: "original-model"
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	// Set env var to override the model.
	t.Setenv("ANTWORT_MODEL", "env-override-model")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if cfg.Engine.DefaultModel != "env-override-model" {
		t.Errorf("default_model = %q, want %q", cfg.Engine.DefaultModel, "env-override-model")
	}

	// Verify the YAML value was read for backend_url (not overridden).
	if cfg.Engine.BackendURL != "http://localhost:9999" {
		t.Errorf("backend_url = %q, want %q", cfg.Engine.BackendURL, "http://localhost:9999")
	}
}

func TestConfigDefaults(t *testing.T) {
	// Create a minimal config with only the required backend_url.
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	yamlContent := `
engine:
  backend_url: "http://localhost:8888"
`
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Verify defaults.
	if cfg.Server.Port != 8080 {
		t.Errorf("server.port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("server.read_timeout = %v, want 30s", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 120*time.Second {
		t.Errorf("server.write_timeout = %v, want 120s", cfg.Server.WriteTimeout)
	}
	if cfg.Storage.Type != "memory" {
		t.Errorf("storage.type = %q, want %q", cfg.Storage.Type, "memory")
	}
	if cfg.Storage.MaxSize != 10000 {
		t.Errorf("storage.max_size = %d, want 10000", cfg.Storage.MaxSize)
	}
	if cfg.Auth.Type != "none" {
		t.Errorf("auth.type = %q, want %q", cfg.Auth.Type, "none")
	}
	if cfg.Engine.Provider != "vllm" {
		t.Errorf("engine.provider = %q, want %q", cfg.Engine.Provider, "vllm")
	}
	if cfg.Engine.MaxTurns != 10 {
		t.Errorf("engine.max_turns = %d, want 10", cfg.Engine.MaxTurns)
	}
	if !cfg.Observability.Metrics.Enabled {
		t.Error("observability.metrics.enabled should be true by default")
	}
	if cfg.Observability.Metrics.Path != "/metrics" {
		t.Errorf("observability.metrics.path = %q, want %q", cfg.Observability.Metrics.Path, "/metrics")
	}
}
