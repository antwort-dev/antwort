package http

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rhuss/antwort/pkg/transport"
)

// Server wraps an http.Server with the transport adapter and manages
// the full lifecycle including startup and graceful shutdown.
type Server struct {
	httpServer *http.Server
	adapter    *Adapter
	config     ServerConfig
	logger     *slog.Logger
}

// ServerConfig holds configuration for the transport server.
type ServerConfig struct {
	Addr            string
	MaxBodySize     int64
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
}

// DefaultServerConfig returns a ServerConfig with sensible defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Addr:            ":8080",
		MaxBodySize:     10 << 20, // 10 MB
		ShutdownTimeout: 30 * time.Second,
		Logger:          slog.Default(),
	}
}

// ServerOption configures a Server.
type ServerOption func(*Server)

// WithAddr sets the listen address.
func WithAddr(addr string) ServerOption {
	return func(s *Server) { s.config.Addr = addr }
}

// WithMaxBodySize sets the maximum request body size.
func WithMaxBodySize(n int64) ServerOption {
	return func(s *Server) { s.config.MaxBodySize = n }
}

// WithShutdownTimeout sets the graceful shutdown deadline.
func WithShutdownTimeout(d time.Duration) ServerOption {
	return func(s *Server) { s.config.ShutdownTimeout = d }
}

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) ServerOption {
	return func(s *Server) { s.config.Logger = l; s.logger = l }
}

// NewServer creates a new transport server with the given handler and options.
// The ResponseStore is optional (pass nil for stateless-only deployments).
// Default middleware (recovery, request ID, logging) is applied automatically.
func NewServer(creator transport.ResponseCreator, store transport.ResponseStore, opts ...ServerOption) *Server {
	s := &Server{
		config: DefaultServerConfig(),
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(s)
	}

	adapterCfg := Config{
		Addr:            s.config.Addr,
		MaxBodySize:     s.config.MaxBodySize,
		ShutdownTimeout: int(s.config.ShutdownTimeout.Seconds()),
	}

	defaultMW := []transport.Middleware{
		transport.Recovery(),
		transport.RequestID(),
		transport.Logging(s.logger),
	}

	s.adapter = NewAdapter(creator, store, adapterCfg, defaultMW...)

	s.httpServer = &http.Server{
		Addr:    s.config.Addr,
		Handler: s.adapter.Handler(),
	}

	return s
}

// ListenAndServe starts the server and blocks until a shutdown signal
// (SIGINT or SIGTERM) is received. It then gracefully shuts down,
// waiting for in-flight requests to complete within the configured timeout.
func (s *Server) ListenAndServe() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return s.listenAndServeWithContext(ctx)
}

func (s *Server) listenAndServeWithContext(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("server starting", slog.String("addr", s.config.Addr))
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		s.logger.Info("shutdown signal received")
	}

	return s.shutdown()
}

// ServeOn starts the server on the given listener. Used for testing.
func (s *Server) ServeOn(ln net.Listener) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	return s.shutdown()
}

func (s *Server) shutdown() error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	s.logger.Info("shutting down gracefully", slog.Duration("timeout", s.config.ShutdownTimeout))
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("shutdown error", slog.String("error", err.Error()))
		return err
	}
	s.logger.Info("server stopped")
	return nil
}

// Shutdown gracefully shuts down the server with the given context.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
