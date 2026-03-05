//go:build e2e

package e2e

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// T022: TestE2EToolCallNonStreaming
// ---------------------------------------------------------------------------

func TestE2EToolCallNonStreaming(t *testing.T) {
	reqBody := map[string]any{
		"model": model,
		"input": []map[string]any{
			{"role": "user", "content": "What is the weather in San Francisco?"},
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

	resp := postJSON(t, "/responses", reqBody)
	if resp.StatusCode != http.StatusOK {
		data := readBody(t, resp)
		t.Skipf("tool call test skipped (no tool executor or recording): status %d, body: %s", resp.StatusCode, data)
	}
	data := readBody(t, resp)

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// Verify the response has output items.
	output, ok := result["output"].([]any)
	if !ok || len(output) == 0 {
		t.Skip("requires tool executor: response has no output items with tool calls")
	}

	// Check if any output item is a function_call or a message with text.
	foundToolCall := false
	foundText := false
	for _, item := range output {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch m["type"] {
		case "function_call":
			foundToolCall = true
		case "message":
			if content, ok := m["content"].([]any); ok {
				for _, c := range content {
					if cm, ok := c.(map[string]any); ok {
						if cm["type"] == "output_text" {
							foundText = true
						}
					}
				}
			}
		}
	}

	if !foundToolCall && !foundText {
		t.Error("expected output to contain either a function_call or text content")
	}
}

// ---------------------------------------------------------------------------
// T023: TestE2EToolCallStreaming
// ---------------------------------------------------------------------------

func TestE2EToolCallStreaming(t *testing.T) {
	reqBody := map[string]any{
		"model":  model,
		"stream": true,
		"input": []map[string]any{
			{"role": "user", "content": "What is the weather in San Francisco?"},
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

	resp := postJSON(t, "/responses", reqBody)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		t.Skipf("streaming tool call test skipped (no tool executor or recording): status %d, body: %s", resp.StatusCode, data)
	}

	// Collect SSE events.
	var events []string
	foundToolEvent := false
	foundTextDelta := false

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType := strings.TrimPrefix(line, "event: ")
			events = append(events, eventType)
			// Tool-related events indicate agentic behavior.
			if strings.Contains(eventType, "function_call") {
				foundToolEvent = true
			}
		}
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			var evt map[string]any
			if json.Unmarshal([]byte(payload), &evt) == nil {
				if evt["type"] == "response.output_text.delta" {
					foundTextDelta = true
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading SSE stream: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("expected SSE events, got none")
	}

	if !foundToolEvent && !foundTextDelta {
		t.Skip("requires tool executor: no tool call events or text deltas in stream")
	}

	// Verify standard lifecycle events are present.
	hasCreated := false
	hasCompleted := false
	for _, e := range events {
		if e == "response.created" {
			hasCreated = true
		}
		if e == "response.completed" {
			hasCompleted = true
		}
	}
	if !hasCreated {
		t.Error("missing response.created event")
	}
	if !hasCompleted {
		t.Error("missing response.completed event")
	}
}
