package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/auth"
	"github.com/rhuss/antwort/pkg/storage"
)

func TestNilLoggerNoOp(t *testing.T) {
	// A nil Logger must not panic on any method call.
	var l *Logger
	l.Log(context.Background(), "test.event", "key", "value")
	l.LogWarn(context.Background(), "test.warn", "key", "value")
}

func TestJSONFormatOutput(t *testing.T) {
	var buf bytes.Buffer
	l := NewFromHandler(slog.NewJSONHandler(&buf, nil))

	l.Log(context.Background(), "test.json", "extra", "data")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nbody: %s", err, buf.String())
	}

	if m["msg"] != "test.json" {
		t.Errorf("msg = %q, want %q", m["msg"], "test.json")
	}
	if m["event"] != "test.json" {
		t.Errorf("event = %q, want %q", m["event"], "test.json")
	}
	if m["extra"] != "data" {
		t.Errorf("extra = %q, want %q", m["extra"], "data")
	}
	if m["level"] != "INFO" {
		t.Errorf("level = %q, want %q", m["level"], "INFO")
	}
}

func TestTextFormatOutput(t *testing.T) {
	var buf bytes.Buffer
	l := NewFromHandler(slog.NewTextHandler(&buf, nil))

	l.Log(context.Background(), "test.text", "key", "val")

	out := buf.String()
	if !strings.Contains(out, "test.text") {
		t.Errorf("text output should contain event name, got: %s", out)
	}
	if !strings.Contains(out, "key=val") {
		t.Errorf("text output should contain key=val, got: %s", out)
	}
}

func TestWarnLevel(t *testing.T) {
	var buf bytes.Buffer
	l := NewFromHandler(slog.NewJSONHandler(&buf, nil))

	l.LogWarn(context.Background(), "test.warn")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if m["level"] != "WARN" {
		t.Errorf("level = %q, want %q", m["level"], "WARN")
	}
}

func TestIdentityExtraction(t *testing.T) {
	var buf bytes.Buffer
	l := NewFromHandler(slog.NewJSONHandler(&buf, nil))

	ctx := context.Background()
	ctx = auth.SetIdentity(ctx, &auth.Identity{Subject: "alice"})
	ctx = storage.SetTenant(ctx, "team-a")

	l.Log(ctx, "test.identity")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if m["subject"] != "alice" {
		t.Errorf("subject = %q, want %q", m["subject"], "alice")
	}
	if m["tenant_id"] != "team-a" {
		t.Errorf("tenant_id = %q, want %q", m["tenant_id"], "team-a")
	}
}

func TestMissingIdentity(t *testing.T) {
	var buf bytes.Buffer
	l := NewFromHandler(slog.NewJSONHandler(&buf, nil))

	l.Log(context.Background(), "test.noident")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if _, ok := m["subject"]; ok {
		t.Errorf("subject should not be present without identity, got %v", m["subject"])
	}
	if _, ok := m["tenant_id"]; ok {
		t.Errorf("tenant_id should not be present without tenant, got %v", m["tenant_id"])
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantNil bool
		wantErr string
	}{
		{
			name:    "disabled returns nil",
			cfg:     Config{Enabled: false},
			wantNil: true,
		},
		{
			name:    "invalid format",
			cfg:     Config{Enabled: true, Format: "xml"},
			wantErr: "invalid format",
		},
		{
			name:    "invalid output",
			cfg:     Config{Enabled: true, Output: "syslog"},
			wantErr: "invalid output",
		},
		{
			name:    "file output without path",
			cfg:     Config{Enabled: true, Output: "file"},
			wantErr: "requires a file path",
		},
		{
			name: "valid json stdout",
			cfg:  Config{Enabled: true, Format: "json", Output: "stdout"},
		},
		{
			name: "defaults to json stdout",
			cfg:  Config{Enabled: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := New(tt.cfg)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil && l != nil {
				t.Error("expected nil logger for disabled config")
			}
			if !tt.wantNil && l == nil {
				t.Error("expected non-nil logger")
			}
		})
	}
}
