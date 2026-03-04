package integration

import (
	"log/slog"
	"testing"

	"github.com/rhuss/antwort/pkg/debug"
)

// TestDebugEnabled verifies that debug categories are correctly enabled
// after calling Init with a category string.
func TestDebugEnabled(t *testing.T) {
	// Save and restore environment to avoid side effects.
	t.Setenv("ANTWORT_DEBUG", "")
	t.Setenv("ANTWORT_LOG_LEVEL", "")

	debug.Init("providers,engine", "INFO")

	if !debug.Enabled("providers") {
		t.Error("expected 'providers' to be enabled")
	}
	if !debug.Enabled("engine") {
		t.Error("expected 'engine' to be enabled")
	}
	if debug.Enabled("unknown") {
		t.Error("expected 'unknown' to be disabled")
	}
	if debug.Enabled("tools") {
		t.Error("expected 'tools' to be disabled")
	}
}

// TestDebugEnabledAll verifies the "all" category enables everything.
func TestDebugEnabledAll(t *testing.T) {
	t.Setenv("ANTWORT_DEBUG", "")
	t.Setenv("ANTWORT_LOG_LEVEL", "")

	debug.Init("all", "INFO")

	if !debug.Enabled("providers") {
		t.Error("expected 'providers' to be enabled with 'all'")
	}
	if !debug.Enabled("anything") {
		t.Error("expected any category to be enabled with 'all'")
	}
}

// TestDebugDisabledEmpty verifies that no categories are enabled when
// the config string is empty.
func TestDebugDisabledEmpty(t *testing.T) {
	t.Setenv("ANTWORT_DEBUG", "")
	t.Setenv("ANTWORT_LOG_LEVEL", "")

	debug.Init("", "INFO")

	if debug.Enabled("providers") {
		t.Error("expected 'providers' to be disabled with empty config")
	}
}

// TestDebugCategories verifies the Categories function returns
// the list of enabled categories.
func TestDebugCategories(t *testing.T) {
	t.Setenv("ANTWORT_DEBUG", "")
	t.Setenv("ANTWORT_LOG_LEVEL", "")

	debug.Init("tools,mcp", "INFO")

	cats := debug.Categories()
	catMap := make(map[string]bool)
	for _, c := range cats {
		catMap[c] = true
	}

	if !catMap["tools"] {
		t.Error("expected 'tools' in categories")
	}
	if !catMap["mcp"] {
		t.Error("expected 'mcp' in categories")
	}
}

// TestDebugParseLevel verifies level string parsing.
func TestDebugParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"TRACE", debug.LevelTrace},
		{"", slog.LevelInfo},       // default
		{"unknown", slog.LevelInfo}, // unknown defaults to INFO
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := debug.ParseLevel(tt.input)
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestDebugTruncate verifies the Truncate helper function.
func TestDebugTruncate(t *testing.T) {
	if got := debug.Truncate("short", 100); got != "short" {
		t.Errorf("Truncate short string = %q, want 'short'", got)
	}
	if got := debug.Truncate("this is a longer string", 10); got != "this is a ..." {
		t.Errorf("Truncate long string = %q, want 'this is a ...'", got)
	}
}

// TestDebugEnvironmentOverride verifies that environment variables override
// config values.
func TestDebugEnvironmentOverride(t *testing.T) {
	t.Setenv("ANTWORT_DEBUG", "auth")
	t.Setenv("ANTWORT_LOG_LEVEL", "")

	// Pass "providers" in config, but env says "auth".
	debug.Init("providers", "INFO")

	if !debug.Enabled("auth") {
		t.Error("expected 'auth' to be enabled (from env)")
	}
	// "providers" from config should be overridden by env.
	if debug.Enabled("providers") {
		t.Error("expected 'providers' to be disabled (env overrides config)")
	}
}
