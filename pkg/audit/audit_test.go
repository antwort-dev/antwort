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

// --- T012: Nil-safe integration test ---

func TestNilLogger_FullRequestFlow(t *testing.T) {
	// Create a nil Logger and call all methods. None should panic.
	var l *Logger

	ctx := context.Background()
	ctx = auth.SetIdentity(ctx, &auth.Identity{Subject: "test-user"})
	ctx = storage.SetTenant(ctx, "test-tenant")

	// Simulate a full request flow with nil logger.
	l.Log(ctx, "auth.success", "auth_method", "jwt", "remote_addr", "1.2.3.4")
	l.LogWarn(ctx, "auth.failure", "auth_method", "unknown", "error", "bad token")
	l.LogWarn(ctx, "auth.rate_limited", "tier", "basic", "remote_addr", "1.2.3.4")
	l.LogWarn(ctx, "authz.scope_denied", "endpoint", "POST /v1/responses", "required_scope", "responses:create")
	l.Log(ctx, "authz.ownership_denied", "resource_type", "response", "resource_id", "resp-123")
	l.Log(ctx, "authz.admin_override", "resource_type", "response", "resource_id", "resp-123")
	// If we got here without panic, the test passes.
}

// --- T013: Enabled integration test ---

func TestEnabled_EventsHaveRequiredFields(t *testing.T) {
	var buf bytes.Buffer
	l := NewFromHandler(slog.NewJSONHandler(&buf, nil))

	ctx := context.Background()
	ctx = auth.SetIdentity(ctx, &auth.Identity{Subject: "alice"})
	ctx = storage.SetTenant(ctx, "team-a")

	l.Log(ctx, "auth.success", "auth_method", "jwt", "remote_addr", "10.0.0.1:8080")

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("failed to parse JSON: %v\nbody: %s", err, buf.String())
	}

	// Must have timestamp.
	if _, ok := m["time"]; !ok {
		t.Error("expected 'time' field in audit event")
	}

	// Must have event name.
	if m["event"] != "auth.success" {
		t.Errorf("event = %q, want %q", m["event"], "auth.success")
	}

	// Must have severity (level).
	if m["level"] != "INFO" {
		t.Errorf("level = %q, want %q", m["level"], "INFO")
	}

	// Must have subject from context.
	if m["subject"] != "alice" {
		t.Errorf("subject = %q, want %q", m["subject"], "alice")
	}

	// Must have tenant from context.
	if m["tenant_id"] != "team-a" {
		t.Errorf("tenant_id = %q, want %q", m["tenant_id"], "team-a")
	}

	// Must have caller-provided attrs.
	if m["auth_method"] != "jwt" {
		t.Errorf("auth_method = %q, want %q", m["auth_method"], "jwt")
	}
}

// --- T014: Config validation edge case tests ---

func TestConfig_DisabledIgnoresOtherFields(t *testing.T) {
	// Even with invalid fields, disabled config should return nil.
	l, err := New(Config{
		Enabled: false,
		Format:  "invalid",
		Output:  "invalid",
		File:    "/nonexistent",
	})
	if err != nil {
		t.Fatalf("disabled config should not return error, got %v", err)
	}
	if l != nil {
		t.Error("disabled config should return nil logger")
	}
}

func TestConfig_FileOutputWithWritablePath(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/audit.log"

	l, err := New(Config{
		Enabled: true,
		Format:  "json",
		Output:  "file",
		File:    path,
	})
	if err != nil {
		t.Fatalf("file output with writable path should succeed, got %v", err)
	}
	if l == nil {
		t.Fatal("expected non-nil logger for file output")
	}

	// Write an event to verify the file works.
	l.Log(context.Background(), "test.file_output")
}

func TestConfig_FileOutputWithNonWritablePath(t *testing.T) {
	_, err := New(Config{
		Enabled: true,
		Format:  "json",
		Output:  "file",
		File:    "/nonexistent-dir/audit.log",
	})
	if err == nil {
		t.Fatal("file output with non-writable path should return error")
	}
	if !strings.Contains(err.Error(), "opening file") {
		t.Errorf("error should mention opening file, got %q", err.Error())
	}
}
