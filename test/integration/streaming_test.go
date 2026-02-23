package integration

import (
	"bufio"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
)

func TestStreamingResponse(t *testing.T) {
	reqBody := map[string]any{
		"model":  "mock-model",
		"stream": true,
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Hello"},
				},
			},
		},
	}

	resp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", contentType)
	}

	// Parse SSE events.
	events := parseSSEEvents(t, resp)

	if len(events) == 0 {
		t.Fatal("no SSE events received")
	}

	// Verify event sequence.
	verifyEventSequence(t, events)
}

func TestStreamingEventSequence(t *testing.T) {
	reqBody := map[string]any{
		"model":  "mock-model",
		"stream": true,
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Hello"},
				},
			},
		},
	}

	resp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	events := parseSSEEvents(t, resp)

	// Check that the first event is response.created.
	if len(events) > 0 && events[0].Type != api.EventResponseCreated {
		t.Errorf("first event type = %q, want %q", events[0].Type, api.EventResponseCreated)
	}

	// Check that the last non-DONE event is response.completed.
	if len(events) > 0 && events[len(events)-1].Type != api.EventResponseCompleted {
		t.Errorf("last event type = %q, want %q", events[len(events)-1].Type, api.EventResponseCompleted)
	}

	// Verify sequence numbers are monotonically increasing.
	for i := 1; i < len(events); i++ {
		if events[i].SequenceNumber <= events[i-1].SequenceNumber {
			t.Errorf("sequence_number not increasing: event[%d]=%d, event[%d]=%d",
				i-1, events[i-1].SequenceNumber, i, events[i].SequenceNumber)
		}
	}
}

func TestStreamingTextDeltas(t *testing.T) {
	reqBody := map[string]any{
		"model":  "mock-model",
		"stream": true,
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Hello"},
				},
			},
		},
	}

	resp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	events := parseSSEEvents(t, resp)

	// Collect text deltas.
	var deltas []string
	for _, e := range events {
		if e.Type == api.EventOutputTextDelta {
			deltas = append(deltas, e.Delta)
		}
	}

	if len(deltas) == 0 {
		t.Error("no text delta events received")
	}

	// Concatenated deltas should form the full response text.
	fullText := strings.Join(deltas, "")
	if fullText == "" {
		t.Error("concatenated deltas are empty")
	}
	t.Logf("accumulated text from deltas: %q", fullText)

	// Verify a text_done event was emitted.
	foundTextDone := false
	for _, e := range events {
		if e.Type == api.EventOutputTextDone {
			foundTextDone = true
			break
		}
	}
	if !foundTextDone {
		t.Error("no text_done event received")
	}
}

func TestStreamingResponsePayload(t *testing.T) {
	reqBody := map[string]any{
		"model":  "mock-model",
		"stream": true,
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Hello"},
				},
			},
		},
	}

	resp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	events := parseSSEEvents(t, resp)

	// The response.created event should have a response object.
	for _, e := range events {
		if e.Type == api.EventResponseCreated {
			if e.Response == nil {
				t.Error("response.created event has nil response")
			} else {
				if e.Response.ID == "" {
					t.Error("response.created response has empty ID")
				}
				if e.Response.Object != "response" {
					t.Errorf("response.created response.object = %q, want %q", e.Response.Object, "response")
				}
			}
			break
		}
	}

	// The response.completed event should have a response with usage.
	for _, e := range events {
		if e.Type == api.EventResponseCompleted {
			if e.Response == nil {
				t.Error("response.completed event has nil response")
			} else if e.Response.Usage == nil {
				t.Error("response.completed response has nil usage")
			}
			break
		}
	}
}

// --- SSE parsing helpers ---

// parseSSEEvents reads SSE events from an HTTP response until [DONE].
func parseSSEEvents(t *testing.T, resp *http.Response) []api.StreamEvent {
	t.Helper()

	var events []api.StreamEvent
	scanner := bufio.NewScanner(resp.Body)

	var eventType string
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Check for DONE sentinel.
			if data == "[DONE]" {
				break
			}

			var event api.StreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				t.Logf("warning: failed to parse SSE event (event=%s): %v, data=%s", eventType, err, data)
				continue
			}

			// If the event type wasn't set via the type field in JSON,
			// use the SSE event type.
			if event.Type == "" && eventType != "" {
				event.Type = api.StreamEventType(eventType)
			}

			events = append(events, event)
			eventType = ""
		}
	}

	if err := scanner.Err(); err != nil {
		t.Logf("warning: scanner error: %v", err)
	}

	return events
}

// verifyEventSequence checks that the event sequence follows the expected pattern.
func verifyEventSequence(t *testing.T, events []api.StreamEvent) {
	t.Helper()

	if len(events) == 0 {
		t.Error("no events to verify")
		return
	}

	// Expected lifecycle event order (at minimum).
	expectedStart := api.EventResponseCreated
	expectedEnd := api.EventResponseCompleted

	if events[0].Type != expectedStart {
		t.Errorf("first event = %q, want %q", events[0].Type, expectedStart)
	}

	lastEvent := events[len(events)-1]
	if lastEvent.Type != expectedEnd {
		t.Errorf("last event = %q, want %q", lastEvent.Type, expectedEnd)
	}

	// Check for required event types.
	typesSeen := map[api.StreamEventType]bool{}
	for _, e := range events {
		typesSeen[e.Type] = true
	}

	requiredTypes := []api.StreamEventType{
		api.EventResponseCreated,
		api.EventOutputItemAdded,
		api.EventContentPartAdded,
		api.EventOutputTextDelta,
		api.EventOutputTextDone,
		api.EventContentPartDone,
		api.EventOutputItemDone,
		api.EventResponseCompleted,
	}

	for _, rt := range requiredTypes {
		if !typesSeen[rt] {
			t.Errorf("missing required event type: %s", rt)
		}
	}
}
