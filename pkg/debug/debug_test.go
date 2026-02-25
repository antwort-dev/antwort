package debug

import (
	"log/slog"
	"testing"
)

func TestParseCategories(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]bool
	}{
		{"empty", "", map[string]bool{}},
		{"single", "providers", map[string]bool{"providers": true}},
		{"multiple", "providers,engine", map[string]bool{"providers": true, "engine": true}},
		{"all", "all", map[string]bool{"all": true}},
		{"with spaces", " providers , engine ", map[string]bool{"providers": true, "engine": true}},
		{"uppercase normalized", "PROVIDERS,Engine", map[string]bool{"providers": true, "engine": true}},
		{"empty segments", "providers,,engine", map[string]bool{"providers": true, "engine": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCategories(tt.input)
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("got[%q] = %v, want %v", k, got[k], v)
				}
			}
			if len(got) != len(tt.want) {
				t.Errorf("len(got) = %d, want %d", len(got), len(tt.want))
			}
		})
	}
}

func TestEnabled(t *testing.T) {
	// Save and restore.
	orig := categories
	defer func() { categories = orig }()

	categories = parseCategories("providers,engine")

	if !Enabled("providers") {
		t.Error("providers should be enabled")
	}
	if !Enabled("engine") {
		t.Error("engine should be enabled")
	}
	if Enabled("mcp") {
		t.Error("mcp should not be enabled")
	}
	if Enabled("all") {
		t.Error("all should not be enabled (not in categories)")
	}
}

func TestEnabled_All(t *testing.T) {
	orig := categories
	defer func() { categories = orig }()

	categories = parseCategories("all")

	if !Enabled("providers") {
		t.Error("providers should be enabled via 'all'")
	}
	if !Enabled("engine") {
		t.Error("engine should be enabled via 'all'")
	}
	if !Enabled("anything") {
		t.Error("anything should be enabled via 'all'")
	}
}

func TestEnabled_Empty(t *testing.T) {
	orig := categories
	defer func() { categories = orig }()

	categories = parseCategories("")

	if Enabled("providers") {
		t.Error("nothing should be enabled when no categories set")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"TRACE", LevelTrace},
		{"trace", LevelTrace},
		{"DEBUG", slog.LevelDebug},
		{"debug", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"info", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"unknown", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLevel(tt.input)
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("short", 10); got != "short" {
		t.Errorf("Truncate short = %q, want %q", got, "short")
	}
	if got := Truncate("this is a long string", 10); got != "this is a ..." {
		t.Errorf("Truncate long = %q, want %q", got, "this is a ...")
	}
}

func TestLog_DisabledCategory(t *testing.T) {
	orig := categories
	defer func() { categories = orig }()

	categories = parseCategories("")

	// Should not panic or produce output.
	Log("providers", "test message", "key", "value")
	Trace("providers", "trace message", "key", "value")
}
