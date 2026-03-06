//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Background Mode E2E Tests
//
// These tests run against a live antwort server (set ANTWORT_BASE_URL).
// The server must be running in integrated mode with a mock backend.
// ---------------------------------------------------------------------------

// TestE2EBackgroundSubmitAndPoll submits a background request and polls
// until it completes. Verifies the full lifecycle: queued -> completed.
func TestE2EBackgroundSubmitAndPoll(t *testing.T) {
	body := map[string]any{
		"model":      model,
		"background": true,
		"input": []map[string]any{
			{"role": "user", "content": "hello from background"},
		},
	}

	resp := postJSON(t, "/responses", body)
	data := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("submit: status %d, body: %s", resp.StatusCode, data)
	}

	var queued map[string]any
	if err := json.Unmarshal(data, &queued); err != nil {
		t.Fatalf("unmarshal queued response: %v", err)
	}

	status, _ := queued["status"].(string)
	if status != "queued" {
		t.Errorf("expected status queued, got %q", status)
	}
	bg, _ := queued["background"].(bool)
	if !bg {
		t.Error("expected background=true")
	}

	id, _ := queued["id"].(string)
	if id == "" {
		t.Fatal("queued response has no id")
	}

	// Poll until completed (up to 30 seconds for E2E).
	completed := pollUntilTerminal(t, id, 30*time.Second)

	compStatus, _ := completed["status"].(string)
	if compStatus != "completed" {
		t.Errorf("expected completed, got %q", compStatus)
	}

	output, _ := completed["output"].([]any)
	if len(output) == 0 {
		t.Error("completed response has no output")
	}
}

// TestE2EBackgroundValidation verifies that invalid background combinations
// are rejected.
func TestE2EBackgroundValidation(t *testing.T) {
	t.Run("background+store_false", func(t *testing.T) {
		body := map[string]any{
			"model":      model,
			"background": true,
			"store":      false,
			"input":      []map[string]any{{"role": "user", "content": "hi"}},
		}
		resp := postJSON(t, "/responses", body)
		readBody(t, resp)
		if resp.StatusCode == http.StatusOK {
			t.Error("expected error for background+store=false")
		}
	})

	t.Run("background+stream", func(t *testing.T) {
		body := map[string]any{
			"model":      model,
			"background": true,
			"stream":     true,
			"input":      []map[string]any{{"role": "user", "content": "hi"}},
		}
		resp := postJSON(t, "/responses", body)
		readBody(t, resp)
		if resp.StatusCode == http.StatusOK {
			t.Error("expected error for background+stream")
		}
	})
}

// TestE2EBackgroundCancel submits a background request and cancels it.
func TestE2EBackgroundCancel(t *testing.T) {
	body := map[string]any{
		"model":      model,
		"background": true,
		"input":      []map[string]any{{"role": "user", "content": "cancel me"}},
	}

	resp := postJSON(t, "/responses", body)
	data := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("submit: status %d, body: %s", resp.StatusCode, data)
	}

	var queued map[string]any
	json.Unmarshal(data, &queued)
	id, _ := queued["id"].(string)

	// Cancel it immediately.
	delResp := deleteHTTP(t, "/responses/"+id)
	readBody(t, delResp)
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE: expected 204, got %d", delResp.StatusCode)
	}

	// Check status is cancelled (or completed if the worker was fast).
	getResp := getJSON(t, "/responses/"+id)
	getData := readBody(t, getResp)
	var result map[string]any
	json.Unmarshal(getData, &result)

	resultStatus, _ := result["status"].(string)
	if resultStatus != "cancelled" && resultStatus != "completed" {
		t.Errorf("expected cancelled or completed, got %q", resultStatus)
	}
}

// TestE2EBackgroundListFilter creates background and sync responses, then
// verifies list filtering by background and status query parameters.
func TestE2EBackgroundListFilter(t *testing.T) {
	// Create a sync response.
	createResponse(t)

	// Create a background response.
	bgBody := map[string]any{
		"model":      model,
		"background": true,
		"input":      []map[string]any{{"role": "user", "content": "bg list test"}},
	}
	bgResp := postJSON(t, "/responses", bgBody)
	bgData := readBody(t, bgResp)
	var bgResult map[string]any
	json.Unmarshal(bgData, &bgResult)
	bgID, _ := bgResult["id"].(string)

	// Wait for the background response to complete.
	pollUntilTerminal(t, bgID, 30*time.Second)

	// Filter by background=true.
	listResp := getJSON(t, "/responses?background=true")
	listData := readBody(t, listResp)
	var listResult map[string]any
	json.Unmarshal(listData, &listResult)

	items, _ := listResult["data"].([]any)
	for _, item := range items {
		m, _ := item.(map[string]any)
		bg, _ := m["background"].(bool)
		if !bg {
			t.Errorf("response %v has background=false in background=true filter", m["id"])
		}
	}
}

// pollUntilTerminal polls GET /responses/{id} until the status is terminal.
func pollUntilTerminal(t *testing.T, id string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp := getJSON(t, "/responses/"+id)
		data := readBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		var result map[string]any
		json.Unmarshal(data, &result)
		status, _ := result["status"].(string)
		if status == "completed" || status == "failed" || status == "cancelled" {
			return result
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("response %s did not reach terminal status within %s", id, timeout)
	return nil
}
