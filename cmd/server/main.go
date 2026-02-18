// Command server runs the antwort OpenResponses gateway.
//
// Configuration via environment variables:
//
//	ANTWORT_BACKEND_URL  - Chat Completions backend URL (required)
//	ANTWORT_MODEL        - Default model name (optional)
//	ANTWORT_PORT         - Listen port (default: 8080)
//	ANTWORT_STORAGE      - Storage type: "memory" or "none" (default: "memory")
//	ANTWORT_STORAGE_SIZE - Max responses in memory store (default: 10000)
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rhuss/antwort/pkg/engine"
	"github.com/rhuss/antwort/pkg/provider/vllm"
	"github.com/rhuss/antwort/pkg/storage/memory"
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
	storageType := envOrDefault("ANTWORT_STORAGE", "memory")
	storageSizeStr := envOrDefault("ANTWORT_STORAGE_SIZE", "10000")

	storageSize, err := strconv.Atoi(storageSizeStr)
	if err != nil {
		return fmt.Errorf("invalid ANTWORT_STORAGE_SIZE: %w", err)
	}

	// Create provider.
	prov, err := vllm.New(vllm.Config{
		BaseURL: backendURL,
		Timeout: 120 * time.Second,
	})
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

	// Create engine.
	eng, err := engine.New(prov, store, engine.Config{
		DefaultModel: defaultModel,
	})
	if err != nil {
		return fmt.Errorf("creating engine: %w", err)
	}

	// Create HTTP adapter.
	adapter := transporthttp.NewAdapter(eng, store, transporthttp.DefaultConfig())

	// Build HTTP mux with health endpoint.
	mux := http.NewServeMux()
	mux.Handle("/", adapter.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok\n"))
	})

	// Create server.
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in background.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", port, "backend", backendURL, "model", defaultModel)
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

func envOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
