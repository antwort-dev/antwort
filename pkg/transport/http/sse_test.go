package http

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
)

func TestWriteResponseJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := newSSEResponseWriter(rec, nil)

	resp := &api.Response{
		ID:     "resp_abc123",
		Object: "response",
		Status: api.ResponseStatusCompleted,
		Model:  "test-model",
	}

	if err := rw.WriteResponse(context.Background(), resp); err != nil {
		t.Fatalf("WriteResponse error: %v", err)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var got api.Response
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.ID != "resp_abc123" {
		t.Errorf("ID = %q, want %q", got.ID, "resp_abc123")
	}
	if got.Status != api.ResponseStatusCompleted {
		t.Errorf("Status = %q, want %q", got.Status, api.ResponseStatusCompleted)
	}
}

func TestWriteEventSSEFormat(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := newSSEResponseWriter(rec, nil)

	event := api.StreamEvent{
		Type:           api.EventOutputTextDelta,
		SequenceNumber: 1,
		Delta:          "Hello",
		ItemID:         "item_001",
	}

	if err := rw.WriteEvent(context.Background(), event); err != nil {
		t.Fatalf("WriteEvent error: %v", err)
	}

	body := rec.Body.String()

	// Check SSE format: event: {type}\ndata: {json}\n\n
	if !strings.Contains(body, "event: response.output_text.delta\n") {
		t.Errorf("missing event type line in:\n%s", body)
	}
	if !strings.Contains(body, "data: ") {
		t.Errorf("missing data line in:\n%s", body)
	}

	// Extract and parse the JSON data.
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			jsonStr := strings.TrimPrefix(line, "data: ")
			var got api.StreamEvent
			if err := json.Unmarshal([]byte(jsonStr), &got); err != nil {
				t.Fatalf("failed to parse event JSON: %v", err)
			}
			if got.Type != api.EventOutputTextDelta {
				t.Errorf("event type = %q, want %q", got.Type, api.EventOutputTextDelta)
			}
			if got.Delta != "Hello" {
				t.Errorf("delta = %q, want %q", got.Delta, "Hello")
			}
		}
	}
}

func TestWriteEventSSEHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := newSSEResponseWriter(rec, nil)

	event := api.StreamEvent{Type: api.EventResponseCreated, SequenceNumber: 0}
	rw.WriteEvent(context.Background(), event)

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", cc, "no-cache")
	}
	if conn := rec.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("Connection = %q, want %q", conn, "keep-alive")
	}
}

func TestWriteEventTerminalSendsDone(t *testing.T) {
	tests := []struct {
		name      string
		eventType api.StreamEventType
	}{
		{"completed", api.EventResponseCompleted},
		{"failed", api.EventResponseFailed},
		{"cancelled", api.EventResponseCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rw := newSSEResponseWriter(rec, nil)

			event := api.StreamEvent{
				Type:     tt.eventType,
				Response: &api.Response{Status: api.ResponseStatus(tt.eventType)},
			}
			if err := rw.WriteEvent(context.Background(), event); err != nil {
				t.Fatalf("WriteEvent error: %v", err)
			}

			body := rec.Body.String()
			if !strings.Contains(body, "data: [DONE]\n") {
				t.Errorf("missing [DONE] sentinel in:\n%s", body)
			}
		})
	}
}

func TestWriteEventAfterTerminalReturnsError(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := newSSEResponseWriter(rec, nil)

	// Send terminal event.
	rw.WriteEvent(context.Background(), api.StreamEvent{
		Type:     api.EventResponseCompleted,
		Response: &api.Response{Status: api.ResponseStatusCompleted},
	})

	// Attempt another write.
	err := rw.WriteEvent(context.Background(), api.StreamEvent{
		Type:  api.EventOutputTextDelta,
		Delta: "should fail",
	})
	if err == nil {
		t.Error("expected error after terminal event, got nil")
	}
}

func TestWriteResponseAfterWriteEventReturnsError(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := newSSEResponseWriter(rec, nil)

	// Start streaming.
	rw.WriteEvent(context.Background(), api.StreamEvent{
		Type: api.EventResponseCreated,
	})

	// Attempt non-streaming response.
	err := rw.WriteResponse(context.Background(), &api.Response{})
	if err == nil {
		t.Error("expected error for WriteResponse after WriteEvent, got nil")
	}
}

func TestWriteEventAfterWriteResponseReturnsError(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := newSSEResponseWriter(rec, nil)

	// Send non-streaming response.
	rw.WriteResponse(context.Background(), &api.Response{})

	// Attempt streaming event.
	err := rw.WriteEvent(context.Background(), api.StreamEvent{
		Type: api.EventOutputTextDelta,
	})
	if err == nil {
		t.Error("expected error for WriteEvent after WriteResponse, got nil")
	}
}

func TestOnResponseCreatedCallback(t *testing.T) {
	rec := httptest.NewRecorder()
	var capturedID string

	rw := newSSEResponseWriter(rec, func(id string) {
		capturedID = id
	})

	event := api.StreamEvent{
		Type:     api.EventResponseCreated,
		Response: &api.Response{ID: "resp_test123"},
	}
	rw.WriteEvent(context.Background(), event)

	if capturedID != "resp_test123" {
		t.Errorf("captured ID = %q, want %q", capturedID, "resp_test123")
	}

	// Second response.created should not trigger callback again.
	capturedID = ""
	rw.WriteEvent(context.Background(), api.StreamEvent{
		Type:     api.EventResponseCreated,
		Response: &api.Response{ID: "resp_second"},
	})
	if capturedID != "" {
		t.Error("callback should only be called once")
	}
}
