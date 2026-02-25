// Package debug provides category-based debug logging for antwort.
//
// Two orthogonal controls:
//   - Categories (WHAT to debug): controlled via ANTWORT_DEBUG env or config
//   - Levels (HOW MUCH detail): controlled via ANTWORT_LOG_LEVEL env or config
//
// Usage:
//
//	debug.Log("providers", "request", "method", "POST", "url", url)
//	if debug.Enabled("providers") { /* expensive formatting */ }
//
// Categories: providers, engine, tools, sandbox, mcp, auth, transport, streaming, config, all.
// Levels: ERROR, WARN, INFO, DEBUG, TRACE.
package debug

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// LevelTrace is below slog.LevelDebug for maximum verbosity.
// At TRACE, full untruncated request/response bodies are logged.
const LevelTrace = slog.LevelDebug - 4

// categories holds the set of enabled debug categories.
// Access is read-only after Init(), so no synchronization needed.
var categories map[string]bool

func init() {
	// Initialize from environment for immediate availability.
	// Can be re-initialized later via Init() with config values.
	env := os.Getenv("ANTWORT_DEBUG")
	categories = parseCategories(env)
}

// Init configures the debug system. Called at startup with values
// from config and/or environment. Environment overrides config.
func Init(configCategories string, configLevel string) {
	// Environment takes precedence over config.
	cats := os.Getenv("ANTWORT_DEBUG")
	if cats == "" {
		cats = configCategories
	}
	categories = parseCategories(cats)

	// Configure slog level.
	level := os.Getenv("ANTWORT_LOG_LEVEL")
	if level == "" {
		level = configLevel
	}
	if level == "" {
		level = "INFO"
	}

	slogLevel := ParseLevel(level)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slogLevel,
	})))
}

// Enabled reports whether debug output is active for the given category.
// This is a constant-time map lookup with zero allocation.
func Enabled(category string) bool {
	return categories["all"] || categories[category]
}

// Log emits a debug message for the given category.
// If the category is not enabled, this is a no-op (zero overhead).
func Log(category string, msg string, args ...any) {
	if !Enabled(category) {
		return
	}
	slog.Debug(msg, append([]any{"debug", category}, args...)...)
}

// Trace emits a trace-level message for the given category.
// Only visible when ANTWORT_LOG_LEVEL=TRACE.
func Trace(category string, msg string, args ...any) {
	if !Enabled(category) {
		return
	}
	slog.Log(nil, LevelTrace, msg, append([]any{"debug", category}, args...)...)
}

// TraceIsEnabled reports whether TRACE level is active for the given category.
func TraceIsEnabled(category string) bool {
	if !Enabled(category) {
		return false
	}
	return slog.Default().Enabled(nil, LevelTrace)
}

// Raw writes plain text to stderr without any slog formatting.
// Use this for copy-paste-ready output (full HTTP bodies, headers).
// Only emitted when category is enabled AND level is TRACE.
func Raw(category string, text string) {
	if !TraceIsEnabled(category) {
		return
	}
	fmt.Fprintln(os.Stderr, text)
}

// ParseLevel converts a level string to a slog.Level.
func ParseLevel(s string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRACE":
		return LevelTrace
	case "DEBUG":
		return slog.LevelDebug
	case "INFO", "":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Categories returns the list of enabled categories (for health/status reporting).
func Categories() []string {
	var result []string
	for k := range categories {
		result = append(result, k)
	}
	return result
}

// Truncate returns s truncated to maxLen characters, with "..." appended if truncated.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func parseCategories(s string) map[string]bool {
	m := make(map[string]bool)
	if s == "" {
		return m
	}
	for _, cat := range strings.Split(s, ",") {
		cat = strings.TrimSpace(strings.ToLower(cat))
		if cat != "" {
			m[cat] = true
		}
	}
	return m
}
