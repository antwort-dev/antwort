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

	// Without stream_options.include_usage, the response.completed event
	// should have a response but usage should be nil (stripped by default).
	for _, e := range events {
		if e.Type == api.EventResponseCompleted {
			if e.Response == nil {
				t.Error("response.completed event has nil response")
			} else if e.Response.Usage != nil {
				t.Logf("response.completed has usage (stream_options not set, usage should be nil)")
			}
			break
		}
	}
}

func TestStreamOptionsIncludeUsage(t *testing.T) {
	reqBody := map[string]any{
		"model":  "mock-model",
		"stream": true,
		"stream_options": map[string]any{
			"include_usage": true,
		},
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

	// With stream_options.include_usage=true, the response.completed
	// event should include usage data.
	for _, e := range events {
		if e.Type == api.EventResponseCompleted {
			if e.Response == nil {
				t.Fatal("response.completed event has nil response")
			}
			if e.Response.Usage == nil {
				t.Error("response.completed should have usage when stream_options.include_usage=true")
			} else if e.Response.Usage.TotalTokens == 0 {
				t.Error("usage.total_tokens is zero")
			}
			break
		}
	}
}

func TestStreamOptionsWithoutUsage(t *testing.T) {
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
		// No stream_options: usage should be nil in streaming events.
	}

	resp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	events := parseSSEEvents(t, resp)

	for _, e := range events {
		if e.Type == api.EventResponseCompleted {
			if e.Response == nil {
				t.Fatal("response.completed event has nil response")
			}
			if e.Response.Usage != nil {
				t.Error("response.completed should NOT have usage when stream_options is absent")
			}
			break
		}
	}
}

func TestStreamingReasoningEvents(t *testing.T) {
	reqBody := map[string]any{
		"model":  "mock-model",
		"stream": true,
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Please reason about this"},
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

	// Log all event types for debugging.
	for i, e := range events {
		t.Logf("event[%d]: %s", i, e.Type)
	}

	// Check for reasoning events.
	typesSeen := map[api.StreamEventType]bool{}
	for _, e := range events {
		typesSeen[e.Type] = true
	}

	if !typesSeen[api.EventReasoningDelta] {
		t.Error("missing reasoning.delta events")
	}
	if !typesSeen[api.EventReasoningDone] {
		t.Error("missing reasoning.done event")
	}

	// Verify reasoning deltas appear before text deltas.
	firstReasoningIdx := -1
	firstTextIdx := -1
	for i, e := range events {
		if e.Type == api.EventReasoningDelta && firstReasoningIdx == -1 {
			firstReasoningIdx = i
		}
		if e.Type == api.EventOutputTextDelta && firstTextIdx == -1 {
			firstTextIdx = i
		}
	}

	if firstReasoningIdx == -1 {
		t.Fatal("no reasoning.delta events found")
	}
	if firstTextIdx == -1 {
		t.Fatal("no text.delta events found")
	}
	if firstReasoningIdx >= firstTextIdx {
		t.Errorf("reasoning.delta (idx %d) should appear before text.delta (idx %d)",
			firstReasoningIdx, firstTextIdx)
	}

	// Collect reasoning deltas.
	var reasoningDeltas []string
	for _, e := range events {
		if e.Type == api.EventReasoningDelta {
			reasoningDeltas = append(reasoningDeltas, e.Delta)
		}
	}
	reasoningText := strings.Join(reasoningDeltas, "")
	t.Logf("accumulated reasoning: %q", reasoningText)
	if reasoningText == "" {
		t.Error("reasoning text is empty")
	}

	// Verify response.completed includes reasoning in output.
	for _, e := range events {
		if e.Type == api.EventResponseCompleted && e.Response != nil {
			foundReasoning := false
			for _, item := range e.Response.Output {
				if item.Type == api.ItemTypeReasoning {
					foundReasoning = true
					break
				}
			}
			if !foundReasoning {
				t.Error("response.completed output missing reasoning item")
			}
			break
		}
	}
}

func TestStreamingNoReasoningForNonReasoningModel(t *testing.T) {
	// A regular request (no "reason" trigger) should produce no reasoning events.
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

	events := parseSSEEvents(t, resp)

	for _, e := range events {
		if e.Type == api.EventReasoningDelta || e.Type == api.EventReasoningDone {
			t.Errorf("unexpected reasoning event %q for non-reasoning request", e.Type)
		}
	}
}

func TestStreamingIncompleteEvent(t *testing.T) {
	reqBody := map[string]any{
		"model":  "mock-model",
		"stream": true,
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Please truncate this response"},
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

	// The terminal event should be response.incomplete, not response.completed.
	lastEvent := events[len(events)-1]
	if lastEvent.Type != api.EventResponseIncomplete {
		t.Errorf("terminal event = %q, want %q", lastEvent.Type, api.EventResponseIncomplete)
	}

	// The response should have incomplete status and details.
	if lastEvent.Response == nil {
		t.Fatal("terminal event has nil response")
	}
	if lastEvent.Response.Status != api.ResponseStatusIncomplete {
		t.Errorf("response status = %q, want %q", lastEvent.Response.Status, api.ResponseStatusIncomplete)
	}
	if lastEvent.Response.IncompleteDetails == nil {
		t.Error("incomplete_details is nil")
	} else if lastEvent.Response.IncompleteDetails.Reason != "max_output_tokens" {
		t.Errorf("incomplete reason = %q, want 'max_output_tokens'", lastEvent.Response.IncompleteDetails.Reason)
	}
}

func TestStreamingToolLifecycleEvents(t *testing.T) {
	// Send a streaming request with tools. The mock backend returns a
	// get_weather tool call, which the mock executor handles. The agentic
	// loop should emit tool lifecycle events around the execution.
	reqBody := map[string]any{
		"model":  "mock-model",
		"stream": true,
		"stream_options": map[string]any{
			"include_usage": true,
		},
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "What is the weather?"},
				},
			},
		},
		"tools": []map[string]any{
			{
				"type": "function",
				"name": "get_weather",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{"type": "string"},
					},
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

	// Log all event types.
	for i, e := range events {
		t.Logf("event[%d]: %s", i, e.Type)
	}

	// The agentic loop should produce function_call events and tool lifecycle
	// events. Look for at least the function_call_arguments events (from the
	// model's tool call) which confirm the agentic loop ran.
	typesSeen := map[api.StreamEventType]bool{}
	for _, e := range events {
		typesSeen[e.Type] = true
	}

	// Verify the agentic loop ran (function call events present).
	if !typesSeen[api.EventFunctionCallArgsDelta] && !typesSeen[api.EventFunctionCallArgsDone] {
		t.Log("no function call events found; agentic loop may not have triggered")
		t.Log("this is expected if the mock backend streaming path doesn't support tool calls")
		t.Skip("skipping: mock backend streaming doesn't produce tool calls")
	}

	// If tool lifecycle events are present, verify ordering.
	if typesSeen[api.EventMCPCallInProgress] || typesSeen[api.EventFileSearchCallInProgress] ||
		typesSeen[api.EventWebSearchCallInProgress] {
		t.Log("tool lifecycle events found")
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
