//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func postJSON(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req, err := http.NewRequest("POST", baseURL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func getJSON(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func deleteHTTP(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("DELETE", baseURL+path, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	return resp
}

// readBody reads and closes the response body.
func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return data
}

// createResponse creates a non-streaming response via raw HTTP and returns the
// parsed JSON as a map. Tests that need only an ID can use this helper.
func createResponse(t *testing.T) map[string]any {
	t.Helper()
	body := map[string]any{
		"model": model,
		"input": []map[string]any{
			{"role": "user", "content": "hello"},
		},
	}
	resp := postJSON(t, "/responses", body)
	if resp.StatusCode != http.StatusOK {
		data := readBody(t, resp)
		t.Fatalf("create response: status %d, body: %s", resp.StatusCode, data)
	}
	data := readBody(t, resp)
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return result
}

// ---------------------------------------------------------------------------
// T014: TestE2ECreateResponse
// ---------------------------------------------------------------------------

func TestE2ECreateResponse(t *testing.T) {
	// Use raw HTTP with array-format input (antwort's expected format).
	result := createResponse(t)
	id, _ := result["id"].(string)
	if id == "" {
		t.Error("expected non-empty response ID")
	}
	respModel, _ := result["model"].(string)
	if respModel != model {
		t.Errorf("expected model %q, got %q", model, respModel)
	}
	status, _ := result["status"].(string)
	if status != "completed" {
		t.Errorf("expected status completed, got %q", status)
	}
	// Verify output contains at least one item with text content.
	output, _ := result["output"].([]any)
	if len(output) == 0 {
		t.Fatal("expected at least one output item")
	}
	foundText := false
	for _, item := range output {
		itemMap, _ := item.(map[string]any)
		if itemMap["type"] == "message" {
			if msg, ok := itemMap["message"].(map[string]any); ok {
				if parts, ok := msg["output"].([]any); ok {
					for _, p := range parts {
						pm, _ := p.(map[string]any)
						if pm["type"] == "output_text" {
							text, _ := pm["text"].(string)
							if text != "" {
								foundText = true
							}
						}
					}
				}
			}
		}
	}
	if !foundText {
		t.Error("expected output to contain text content")
	}
}

// ---------------------------------------------------------------------------
// T015: TestE2EStreamingResponse
// ---------------------------------------------------------------------------

func TestE2EStreamingResponse(t *testing.T) {
	body := map[string]any{
		"model":  model,
		"input":  []map[string]any{{"role": "user", "content": "hello"}},
		"stream": true,
	}
	resp := postJSON(t, "/responses", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("streaming request failed: status %d, body: %s", resp.StatusCode, data)
	}

	// Collect SSE events.
	var events []string
	var collectedText strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "event: ") {
			// Collect text deltas from data lines.
			if strings.HasPrefix(line, "data: ") {
				payload := strings.TrimPrefix(line, "data: ")
				var evt map[string]any
				if json.Unmarshal([]byte(payload), &evt) == nil {
					if evt["type"] == "response.output_text.delta" {
						if delta, ok := evt["delta"].(map[string]any); ok {
							if txt, ok := delta["text"].(string); ok {
								collectedText.WriteString(txt)
							}
						}
					}
				}
			}
			continue
		}
		eventType := strings.TrimPrefix(line, "event: ")
		events = append(events, eventType)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading SSE stream: %v", err)
	}

	// Verify key lifecycle events are present.
	requiredEvents := []string{
		"response.created",
		"response.output_item.added",
		"response.completed",
	}
	for _, req := range requiredEvents {
		found := false
		for _, e := range events {
			if e == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing required SSE event %q in stream (got %v)", req, events)
		}
	}

	if collectedText.Len() == 0 {
		t.Error("expected non-empty text from streaming deltas")
	}
}

// ---------------------------------------------------------------------------
// T016: TestE2EGetResponse
// ---------------------------------------------------------------------------

func TestE2EGetResponse(t *testing.T) {
	created := createResponse(t)
	id, ok := created["id"].(string)
	if !ok || id == "" {
		t.Fatal("created response has no id")
	}

	resp := getJSON(t, "/responses/"+id)
	if resp.StatusCode != http.StatusOK {
		data := readBody(t, resp)
		t.Fatalf("GET response: status %d, body: %s", resp.StatusCode, data)
	}
	data := readBody(t, resp)
	var fetched map[string]any
	if err := json.Unmarshal(data, &fetched); err != nil {
		t.Fatalf("unmarshal GET response: %v", err)
	}
	if fetched["id"] != id {
		t.Errorf("expected id %q, got %q", id, fetched["id"])
	}
	if fetched["model"] != created["model"] {
		t.Errorf("expected model %q, got %q", created["model"], fetched["model"])
	}
}

// ---------------------------------------------------------------------------
// T017: TestE2EListResponses
// ---------------------------------------------------------------------------

func TestE2EListResponses(t *testing.T) {
	// Create two responses so we know there is something to list.
	r1 := createResponse(t)
	r2 := createResponse(t)

	resp := getJSON(t, "/responses")
	if resp.StatusCode != http.StatusOK {
		data := readBody(t, resp)
		t.Fatalf("GET /responses: status %d, body: %s", resp.StatusCode, data)
	}
	data := readBody(t, resp)

	var list map[string]any
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}

	items, ok := list["data"].([]any)
	if !ok {
		t.Fatalf("expected data array in list response, got: %s", data)
	}

	// Verify both created IDs appear in the list.
	ids := make(map[string]bool, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			if id, ok := m["id"].(string); ok {
				ids[id] = true
			}
		}
	}
	for _, want := range []string{r1["id"].(string), r2["id"].(string)} {
		if !ids[want] {
			t.Errorf("expected response %q in list, not found", want)
		}
	}
}

// ---------------------------------------------------------------------------
// T018: TestE2EDeleteResponse
// ---------------------------------------------------------------------------

func TestE2EDeleteResponse(t *testing.T) {
	created := createResponse(t)
	id, ok := created["id"].(string)
	if !ok || id == "" {
		t.Fatal("created response has no id")
	}

	// Delete the response.
	delResp := deleteHTTP(t, "/responses/"+id)
	readBody(t, delResp)
	if delResp.StatusCode != http.StatusOK && delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE response: unexpected status %d", delResp.StatusCode)
	}

	// Verify it is gone.
	getResp := getJSON(t, "/responses/"+id)
	readBody(t, getResp)
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getResp.StatusCode)
	}
}
