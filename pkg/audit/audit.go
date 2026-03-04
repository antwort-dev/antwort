// Package audit provides nil-safe structured audit logging for security-relevant
// events. A nil *Logger is safe to use; all methods become no-ops.
package audit

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/rhuss/antwort/pkg/auth"
	"github.com/rhuss/antwort/pkg/storage"
)

// Config holds settings for audit logging.
type Config struct {
	Enabled bool   `yaml:"enabled"` // Enable audit logging (default: false)
	Format  string `yaml:"format"`  // "json" (default) or "text"
	Output  string `yaml:"output"`  // "stdout" (default) or "file"
	File    string `yaml:"file"`    // Path when Output is "file"
}

// Logger emits structured audit events. A nil *Logger is safe to call;
// all methods silently do nothing when the receiver is nil.
type Logger struct {
	logger *slog.Logger
}

// New creates an audit Logger from the given config. Returns nil when
// audit logging is disabled. Returns an error for invalid configuration.
func New(cfg Config) (*Logger, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	format := cfg.Format
	if format == "" {
		format = "json"
	}
	output := cfg.Output
	if output == "" {
		output = "stdout"
	}

	// Validate format.
	if format != "json" && format != "text" {
		return nil, fmt.Errorf("audit: invalid format %q (supported: json, text)", format)
	}

	// Validate output.
	if output != "stdout" && output != "file" {
		return nil, fmt.Errorf("audit: invalid output %q (supported: stdout, file)", output)
	}
	if output == "file" && cfg.File == "" {
		return nil, fmt.Errorf("audit: output=file requires a file path")
	}

	// Select writer.
	var w io.Writer
	switch output {
	case "stdout":
		w = os.Stdout
	case "file":
		f, err := os.OpenFile(cfg.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("audit: opening file %q: %w", cfg.File, err)
		}
		w = f
	}

	// Create handler.
	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(w, nil)
	case "text":
		handler = slog.NewTextHandler(w, nil)
	}

	return &Logger{logger: slog.New(handler)}, nil
}

// NewFromHandler creates an audit Logger that writes to the given handler.
// This is primarily useful for testing.
func NewFromHandler(h slog.Handler) *Logger {
	return &Logger{logger: slog.New(h)}
}

// Log emits an audit event at INFO level. It extracts the caller identity
// and tenant from the context automatically.
func (l *Logger) Log(ctx context.Context, event string, attrs ...any) {
	if l == nil {
		return
	}
	l.log(ctx, slog.LevelInfo, event, attrs...)
}

// LogWarn emits an audit event at WARN level.
func (l *Logger) LogWarn(ctx context.Context, event string, attrs ...any) {
	if l == nil {
		return
	}
	l.log(ctx, slog.LevelWarn, event, attrs...)
}

// log is the shared implementation for Log and LogWarn.
func (l *Logger) log(ctx context.Context, level slog.Level, event string, attrs ...any) {
	// Build base attributes: event name, identity, tenant.
	base := []any{"event", event}

	if id := auth.IdentityFromContext(ctx); id != nil {
		base = append(base, "subject", id.Subject)
	}
	if tenant := storage.GetTenant(ctx); tenant != "" {
		base = append(base, "tenant_id", tenant)
	}

	// Append caller-provided attributes after base fields.
	base = append(base, attrs...)

	l.logger.Log(ctx, level, event, base...)
}
