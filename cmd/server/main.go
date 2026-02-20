// Command server runs the antwort OpenResponses gateway.
//
// Configuration via environment variables:
//
//	ANTWORT_BACKEND_URL  - Chat Completions backend URL (required)
//	ANTWORT_MODEL        - Default model name (optional)
//	ANTWORT_PORT         - Listen port (default: 8080)
//	ANTWORT_PROVIDER     - Provider type: "vllm" (default) or "litellm"
//	ANTWORT_STORAGE      - Storage type: "memory" or "none" (default: "memory")
//	ANTWORT_STORAGE_SIZE - Max responses in memory store (default: 10000)
//	ANTWORT_MCP_SERVERS  - JSON array of MCP server configs (optional)
//	                       Format: [{"name":"my-server","transport":"streamable-http","url":"http://...","headers":{"Authorization":"Bearer ..."}}]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rhuss/antwort/pkg/auth"
	"github.com/rhuss/antwort/pkg/auth/apikey"
	"github.com/rhuss/antwort/pkg/auth/noop"
	"github.com/rhuss/antwort/pkg/engine"
	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/provider/litellm"
	"github.com/rhuss/antwort/pkg/provider/vllm"
	"github.com/rhuss/antwort/pkg/storage/memory"
	"github.com/rhuss/antwort/pkg/tools"
	mcptools "github.com/rhuss/antwort/pkg/tools/mcp"
	"github.com/rhuss/antwort/pkg/transport"
	transporthttp "github.com/rhuss/antwort/pkg/transport/http"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Read configuration from environment.
	backendURL := os.Getenv("ANTWORT_BACKEND_URL")
	if backendURL == "" {
		return fmt.Errorf("ANTWORT_BACKEND_URL is required")
	}

	defaultModel := os.Getenv("ANTWORT_MODEL")
	port := envOrDefault("ANTWORT_PORT", "8080")
	providerType := envOrDefault("ANTWORT_PROVIDER", "vllm")
	storageType := envOrDefault("ANTWORT_STORAGE", "memory")
	storageSizeStr := envOrDefault("ANTWORT_STORAGE_SIZE", "10000")

	storageSize, err := strconv.Atoi(storageSizeStr)
	if err != nil {
		return fmt.Errorf("invalid ANTWORT_STORAGE_SIZE: %w", err)
	}

	// Create provider.
	prov, err := createProvider(providerType, backendURL)
	if err != nil {
		return fmt.Errorf("creating provider: %w", err)
	}
	defer prov.Close()

	// Create optional store.
	var store transport.ResponseStore
	if storageType == "memory" {
		store = memory.New(storageSize)
		slog.Info("storage enabled", "type", "memory", "max_size", storageSize)
	} else {
		slog.Info("storage disabled")
	}

	// Create MCP executor if configured.
	var executors []tools.ToolExecutor
	mcpExecutor, err := createMCPExecutor()
	if err != nil {
		return fmt.Errorf("creating MCP executor: %w", err)
	}
	if mcpExecutor != nil {
		executors = append(executors, mcpExecutor)
		defer mcpExecutor.Close()
	}

	// Create engine.
	eng, err := engine.New(prov, store, engine.Config{
		DefaultModel: defaultModel,
		Executors:    executors,
	})
	if err != nil {
		return fmt.Errorf("creating engine: %w", err)
	}

	// Create HTTP adapter.
	adapter := transporthttp.NewAdapter(eng, store, transporthttp.DefaultConfig())

	// Build auth chain.
	authChain := buildAuthChain()

	// Build HTTP mux with health endpoint.
	mux := http.NewServeMux()
	mux.Handle("/", adapter.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok\n"))
	})

	// Wrap with CORS middleware (for browser-based compliance testing).
	var handler http.Handler = corsMiddleware(mux)

	// Wrap with auth middleware.
	if authChain != nil {
		authMiddleware := auth.Middleware(authChain, nil, auth.DefaultBypassEndpoints)
		handler = authMiddleware(handler)
	}

	// Create server.
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in background.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", port, "backend", backendURL, "provider", providerType, "model", defaultModel)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for shutdown signal or error.
	select {
	case <-ctx.Done():
		slog.Info("shutting down gracefully")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// buildAuthChain creates an auth chain from environment configuration.
// Returns nil when auth is disabled (ANTWORT_AUTH_TYPE=none or unset).
func buildAuthChain() *auth.AuthChain {
	authType := os.Getenv("ANTWORT_AUTH_TYPE")

	switch authType {
	case "apikey":
		keys := parseAPIKeys(os.Getenv("ANTWORT_API_KEYS"))
		if len(keys) == 0 {
			slog.Warn("ANTWORT_AUTH_TYPE=apikey but no ANTWORT_API_KEYS configured")
			return nil
		}
		slog.Info("auth enabled", "type", "apikey", "keys", len(keys))
		return &auth.AuthChain{
			Authenticators:  []auth.Authenticator{apikey.New(keys)},
			DefaultDecision: auth.No,
		}

	case "none", "":
		// No auth (development mode).
		return nil

	default:
		slog.Warn("unknown ANTWORT_AUTH_TYPE, auth disabled", "type", authType)
		return nil
	}
}

// parseAPIKeys parses a JSON array of key entries from an env var.
// Format: [{"key":"sk-...","subject":"alice","tenant_id":"org-1","service_tier":"standard"}]
func parseAPIKeys(jsonStr string) []apikey.RawKeyEntry {
	if jsonStr == "" {
		return nil
	}

	type rawKey struct {
		Key         string `json:"key"`
		Subject     string `json:"subject"`
		TenantID    string `json:"tenant_id"`
		ServiceTier string `json:"service_tier"`
	}

	var keys []rawKey
	if err := json.Unmarshal([]byte(jsonStr), &keys); err != nil {
		slog.Error("failed to parse ANTWORT_API_KEYS", "error", err)
		return nil
	}

	var entries []apikey.RawKeyEntry
	for _, k := range keys {
		metadata := map[string]string{}
		if k.TenantID != "" {
			metadata["tenant_id"] = k.TenantID
		}
		entries = append(entries, apikey.RawKeyEntry{
			Key: k.Key,
			Identity: auth.Identity{
				Subject:     k.Subject,
				ServiceTier: k.ServiceTier,
				Metadata:    metadata,
			},
		})
	}
	return entries
}

// corsMiddleware adds CORS headers for browser-based compliance testing.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Ensure noop package is available (used indirectly via auth chain default).
var _ auth.Authenticator = (*noop.Authenticator)(nil)

// createProvider creates a provider.Provider based on the given type name.
// Supported types: "vllm" (default), "litellm".
func createProvider(providerType, backendURL string) (provider.Provider, error) {
	apiKey := os.Getenv("ANTWORT_API_KEY")

	switch providerType {
	case "vllm", "":
		return vllm.New(vllm.Config{
			BaseURL: backendURL,
			APIKey:  apiKey,
			Timeout: 120 * time.Second,
		})

	case "litellm":
		cfg := litellm.Config{
			BaseURL: backendURL,
			APIKey:  apiKey,
			Timeout: 120 * time.Second,
		}
		// Parse model mapping from ANTWORT_MODEL_MAPPING if set.
		// Format: JSON object, e.g. {"gpt-4":"openai/gpt-4","claude":"anthropic/claude-3-opus"}
		if mappingJSON := os.Getenv("ANTWORT_MODEL_MAPPING"); mappingJSON != "" {
			var mapping map[string]string
			if err := json.Unmarshal([]byte(mappingJSON), &mapping); err != nil {
				return nil, fmt.Errorf("invalid ANTWORT_MODEL_MAPPING: %w", err)
			}
			cfg.ModelMapping = mapping
			slog.Info("model mapping configured", "mappings", len(mapping))
		}
		return litellm.New(cfg)

	default:
		return nil, fmt.Errorf("unknown provider type %q (supported: vllm, litellm)", providerType)
	}
}

// createMCPExecutor reads ANTWORT_MCP_SERVERS, connects to each MCP server,
// and returns an MCPExecutor. Returns nil if the env var is not set.
func createMCPExecutor() (*mcptools.MCPExecutor, error) {
	mcpJSON := os.Getenv("ANTWORT_MCP_SERVERS")
	if mcpJSON == "" {
		return nil, nil
	}

	var servers []mcptools.ServerConfig
	if err := json.Unmarshal([]byte(mcpJSON), &servers); err != nil {
		return nil, fmt.Errorf("invalid ANTWORT_MCP_SERVERS JSON: %w", err)
	}

	if len(servers) == 0 {
		return nil, nil
	}

	ctx := context.Background()
	clients := make(map[string]*mcptools.MCPClient, len(servers))

	for _, cfg := range servers {
		if cfg.Name == "" {
			return nil, fmt.Errorf("MCP server config missing 'name'")
		}
		if cfg.URL == "" {
			return nil, fmt.Errorf("MCP server %q missing 'url'", cfg.Name)
		}

		client := mcptools.NewMCPClient(cfg)
		if err := client.Connect(ctx); err != nil {
			// Close already-connected clients on failure.
			for _, c := range clients {
				_ = c.Close()
			}
			return nil, fmt.Errorf("connecting to MCP server %q: %w", cfg.Name, err)
		}

		clients[cfg.Name] = client
		slog.Info("MCP server connected", "name", cfg.Name, "url", cfg.URL, "transport", cfg.Transport)
	}

	return mcptools.NewMCPExecutor(clients), nil
}

func envOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
