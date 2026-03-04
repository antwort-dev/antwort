package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/audit"
	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/tools"
)

// captureAuditLogger creates an audit logger that writes to a buffer,
// and returns both the logger and the buffer for inspection.
func captureAuditLogger() (*audit.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	return audit.NewFromHandler(handler), &buf
}

// parseAuditEvents parses newline-delimited JSON log entries from a buffer.
func parseAuditEvents(buf *bytes.Buffer) []map[string]any {
	var events []map[string]any
	decoder := json.NewDecoder(buf)
	for decoder.More() {
		var entry map[string]any
		if err := decoder.Decode(&entry); err != nil {
			break
		}
		events = append(events, entry)
	}
	return events
}

// findAuditEvents returns events matching the given event name.
func findAuditEvents(events []map[string]any, eventName string) []map[string]any {
	var matched []map[string]any
	for _, e := range events {
		if e["event"] == eventName {
			matched = append(matched, e)
		}
	}
	return matched
}

// mockToolExecutor is a configurable mock executor for audit tests.
type mockToolExecutor struct {
	kind    tools.ToolKind
	canExec func(string) bool
	execFn  func(context.Context, tools.ToolCall) (*tools.ToolResult, error)
}

func (m *mockToolExecutor) Kind() tools.ToolKind        { return m.kind }
func (m *mockToolExecutor) CanExecute(name string) bool { return m.canExec(name) }
func (m *mockToolExecutor) Execute(ctx context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
	return m.execFn(ctx, call)
}

func TestToolAuditEvents(t *testing.T) {
	tests := []struct {
		name           string
		executor       *mockToolExecutor // nil means no executor registered
		toolCalls      []tools.ToolCall
		wantEvent      string // expected audit event name
		wantToolType   string
		wantToolName   string
		wantError      bool // true if the event should contain an error field
		wantLevel      string
		useNilLogger   bool
		parallel       bool
	}{
		{
			name: "successful tool execution emits tool.executed",
			executor: &mockToolExecutor{
				kind:    tools.ToolKindMCP,
				canExec: func(string) bool { return true },
				execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
					return &tools.ToolResult{CallID: call.ID, Output: "result"}, nil
				},
			},
			toolCalls:    []tools.ToolCall{{ID: "c1", Name: "my_tool", Arguments: "{}"}},
			wantEvent:    "tool.executed",
			wantToolType: "mcp",
			wantToolName: "my_tool",
			wantError:    false,
			wantLevel:    "INFO",
		},
		{
			name: "tool execution Go error emits tool.failed",
			executor: &mockToolExecutor{
				kind:    tools.ToolKindBuiltin,
				canExec: func(string) bool { return true },
				execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
					return nil, fmt.Errorf("connection refused")
				},
			},
			toolCalls:    []tools.ToolCall{{ID: "c2", Name: "web_search", Arguments: "{}"}},
			wantEvent:    "tool.failed",
			wantToolType: "web_search",
			wantToolName: "web_search",
			wantError:    true,
			wantLevel:    "WARN",
		},
		{
			name: "tool result IsError emits tool.failed",
			executor: &mockToolExecutor{
				kind:    tools.ToolKindBuiltin,
				canExec: func(string) bool { return true },
				execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
					return &tools.ToolResult{CallID: call.ID, Output: "bad input", IsError: true}, nil
				},
			},
			toolCalls:    []tools.ToolCall{{ID: "c3", Name: "file_search", Arguments: "{}"}},
			wantEvent:    "tool.failed",
			wantToolType: "file_search",
			wantToolName: "file_search",
			wantError:    true,
			wantLevel:    "WARN",
		},
		{
			name:         "no executor emits tool.failed with unknown type",
			executor:     nil,
			toolCalls:    []tools.ToolCall{{ID: "c4", Name: "missing_tool", Arguments: "{}"}},
			wantEvent:    "tool.failed",
			wantToolType: "unknown",
			wantToolName: "missing_tool",
			wantError:    true,
			wantLevel:    "WARN",
		},
		{
			name:         "nil audit logger produces no panics",
			executor:     nil,
			toolCalls:    []tools.ToolCall{{ID: "c5", Name: "some_tool", Arguments: "{}"}},
			useNilLogger: true,
		},
		{
			name: "concurrent execution emits tool.executed",
			executor: &mockToolExecutor{
				kind:    tools.ToolKindMCP,
				canExec: func(string) bool { return true },
				execFn: func(_ context.Context, call tools.ToolCall) (*tools.ToolResult, error) {
					return &tools.ToolResult{CallID: call.ID, Output: "ok"}, nil
				},
			},
			toolCalls: []tools.ToolCall{
				{ID: "c6", Name: "tool_a", Arguments: "{}"},
				{ID: "c7", Name: "tool_b", Arguments: "{}"},
			},
			wantEvent:    "tool.executed",
			wantToolType: "mcp",
			wantToolName: "tool_a", // We check at least one matches.
			wantError:    false,
			wantLevel:    "INFO",
			parallel:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var executors []tools.ToolExecutor
			if tt.executor != nil {
				executors = append(executors, tt.executor)
			}

			var auditLogger AuditLogger
			var buf *bytes.Buffer
			if !tt.useNilLogger {
				al, b := captureAuditLogger()
				auditLogger = al
				buf = b
			}

			eng, err := New(&turnAwareProvider{
				responses: []*provider.ProviderResponse{
					{Status: api.ResponseStatusCompleted},
				},
			}, nil, Config{

				Executors:   executors,
				AuditLogger: auditLogger,
			})
			if err != nil {
				t.Fatalf("failed to create engine: %v", err)
			}

			ctx := context.Background()
			if tt.parallel {
				eng.executeToolsConcurrently(ctx, tt.toolCalls)
			} else {
				eng.executeToolsSequentially(ctx, tt.toolCalls)
			}

			// For nil logger test, just verify no panic occurred.
			if tt.useNilLogger {
				return
			}

			events := parseAuditEvents(buf)
			matched := findAuditEvents(events, tt.wantEvent)

			if len(matched) == 0 {
				t.Fatalf("expected audit event %q, got events: %v", tt.wantEvent, events)
			}

			// Check at least one matching event has the expected fields.
			var found bool
			for _, ev := range matched {
				if ev["tool_name"] == tt.wantToolName && ev["tool_type"] == tt.wantToolType {
					found = true
					if tt.wantError {
						if _, ok := ev["error"]; !ok {
							t.Errorf("expected error field in audit event")
						}
					}
					// Verify log level.
					if level, ok := ev["level"]; ok {
						if level != tt.wantLevel {
							t.Errorf("level = %v, want %v", level, tt.wantLevel)
						}
					}
				}
			}
			if !found {
				t.Errorf("no audit event with tool_name=%q tool_type=%q found in %v",
					tt.wantToolName, tt.wantToolType, matched)
			}
		})
	}
}

