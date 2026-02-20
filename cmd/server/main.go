// Command server runs the antwort OpenResponses gateway.
//
// Configuration can be provided via:
//   - YAML config file (--config flag, ANTWORT_CONFIG env, ./config.yaml, /etc/antwort/config.yaml)
//   - Environment variables with ANTWORT_ prefix (override config file values)
//   - Legacy env vars: ANTWORT_BACKEND_URL, ANTWORT_MODEL, ANTWORT_PORT, etc.
//
// See config.example.yaml for full documentation of available settings.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rhuss/antwort/pkg/auth"
	"github.com/rhuss/antwort/pkg/auth/apikey"
	"github.com/rhuss/antwort/pkg/auth/noop"
	"github.com/rhuss/antwort/pkg/config"
	"github.com/rhuss/antwort/pkg/engine"
	"github.com/rhuss/antwort/pkg/observability"
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
	// Parse command-line flags.
	configPath := flag.String("config", "", "path to YAML config file")
	flag.Parse()

	// Load configuration (YAML file + env overrides + defaults).
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Create provider from config.
	prov, err := createProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating provider: %w", err)
	}
	defer prov.Close()

	// Create storage from config.
	store := createStore(cfg)

	// Create MCP executor if configured.
	var executors []tools.ToolExecutor
	mcpExecutor, err := createMCPExecutor(cfg)
	if err != nil {
		return fmt.Errorf("creating MCP executor: %w", err)
	}
	if mcpExecutor != nil {
		executors = append(executors, mcpExecutor)
		defer mcpExecutor.Close()
	}

	// Create engine.
	eng, err := engine.New(prov, store, engine.Config{
		DefaultModel:    cfg.Engine.DefaultModel,
		MaxAgenticTurns: cfg.Engine.MaxTurns,
		Executors:       executors,
	})
	if err != nil {
		return fmt.Errorf("creating engine: %w", err)
	}

	// Create HTTP adapter.
	adapter := transporthttp.NewAdapter(eng, store, transporthttp.DefaultConfig())

	// Build auth chain from config.
	authChain := buildAuthChain(cfg)

	// Build HTTP mux with health endpoint.
	mux := http.NewServeMux()
	mux.Handle("/", adapter.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok\n"))
	})

	// Register Prometheus metrics endpoint if enabled.
	if cfg.Observability.Metrics.Enabled {
		metricsPath := cfg.Observability.Metrics.Path
		mux.Handle("GET "+metricsPath, promhttp.Handler())
		slog.Info("metrics endpoint enabled", "path", metricsPath)
	}

	// Wrap with CORS middleware (for browser-based compliance testing).
	var handler http.Handler = corsMiddleware(mux)

	// Wrap with metrics middleware (before auth so all requests are counted).
	if cfg.Observability.Metrics.Enabled {
		handler = observability.MetricsMiddleware(handler)
	}

	// Wrap with auth middleware.
	if authChain != nil {
		authMiddleware := auth.Middleware(authChain, nil, auth.DefaultBypassEndpoints)
		handler = authMiddleware(handler)
	}

	// Create server with configured timeouts.
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in background.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting",
			"port", cfg.Server.Port,
			"backend", cfg.Engine.BackendURL,
			"provider", cfg.Engine.Provider,
			"model", cfg.Engine.DefaultModel,
		)
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

// createProvider creates a provider.Provider from the config.
func createProvider(cfg *config.Config) (provider.Provider, error) {
	switch cfg.Engine.Provider {
	case "vllm", "":
		return vllm.New(vllm.Config{
			BaseURL: cfg.Engine.BackendURL,
			APIKey:  cfg.Engine.APIKey,
			Timeout: cfg.Server.WriteTimeout,
		})

	case "litellm":
		return litellm.New(litellm.Config{
			BaseURL: cfg.Engine.BackendURL,
			APIKey:  cfg.Engine.APIKey,
			Timeout: cfg.Server.WriteTimeout,
		})

	default:
		return nil, fmt.Errorf("unknown provider type %q (supported: vllm, litellm)", cfg.Engine.Provider)
	}
}

// createStore creates a ResponseStore from the config.
func createStore(cfg *config.Config) transport.ResponseStore {
	switch cfg.Storage.Type {
	case "memory":
		store := memory.New(cfg.Storage.MaxSize)
		slog.Info("storage enabled", "type", "memory", "max_size", cfg.Storage.MaxSize)
		return store
	default:
		slog.Info("storage disabled")
		return nil
	}
}

// createMCPExecutor creates an MCP executor from the config.
// Returns nil if no MCP servers are configured.
func createMCPExecutor(cfg *config.Config) (*mcptools.MCPExecutor, error) {
	if len(cfg.MCP.Servers) == 0 {
		return nil, nil
	}

	ctx := context.Background()
	clients := make(map[string]*mcptools.MCPClient, len(cfg.MCP.Servers))

	for _, serverCfg := range cfg.MCP.Servers {
		if serverCfg.Name == "" {
			return nil, fmt.Errorf("MCP server config missing 'name'")
		}
		if serverCfg.URL == "" {
			return nil, fmt.Errorf("MCP server %q missing 'url'", serverCfg.Name)
		}

		mcpCfg := mcptools.ServerConfig{
			Name:      serverCfg.Name,
			Transport: serverCfg.Transport,
			URL:       serverCfg.URL,
			Headers:   serverCfg.Headers,
		}

		// Configure auth provider based on auth type.
		mcpCfg.Auth = buildMCPAuthConfig(serverCfg.Auth)

		client := mcptools.NewMCPClient(mcpCfg)
		if err := client.Connect(ctx); err != nil {
			// Close already-connected clients on failure.
			for _, c := range clients {
				_ = c.Close()
			}
			return nil, fmt.Errorf("connecting to MCP server %q: %w", serverCfg.Name, err)
		}

		clients[serverCfg.Name] = client
		authType := serverCfg.Auth.Type
		if authType == "" {
			authType = "none"
		}
		slog.Info("MCP server connected", "name", serverCfg.Name, "url", serverCfg.URL, "transport", serverCfg.Transport, "auth", authType)
	}

	return mcptools.NewMCPExecutor(clients), nil
}

// buildMCPAuthConfig converts a config.MCPAuthConfig to the MCP package's MCPAuthConfig.
func buildMCPAuthConfig(authCfg config.MCPAuthConfig) mcptools.MCPAuthConfig {
	return mcptools.MCPAuthConfig{
		Type:             authCfg.Type,
		TokenURL:         authCfg.TokenURL,
		ClientID:         authCfg.ClientID,
		ClientIDFile:     authCfg.ClientIDFile,
		ClientSecret:     authCfg.ClientSecret,
		ClientSecretFile: authCfg.ClientSecretFile,
		Scopes:           authCfg.Scopes,
	}
}

// buildAuthChain creates an auth chain from config.
// Returns nil when auth is disabled (type=none).
func buildAuthChain(cfg *config.Config) *auth.AuthChain {
	switch cfg.Auth.Type {
	case "apikey":
		keys := convertAPIKeys(cfg.Auth.APIKeys)
		if len(keys) == 0 {
			slog.Warn("auth.type=apikey but no api_keys configured")
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
		slog.Warn("unknown auth type, auth disabled", "type", cfg.Auth.Type)
		return nil
	}
}

// convertAPIKeys converts config API key entries to the apikey package format.
func convertAPIKeys(keys []config.APIKeyConfig) []apikey.RawKeyEntry {
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
