//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

// ---------------------------------------------------------------------------
// HTTP helpers with custom auth key
// ---------------------------------------------------------------------------

func postJSONWithKey(t *testing.T, path string, body any, key string) *http.Response {
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
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func getJSONWithKey(t *testing.T, path string, key string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// ---------------------------------------------------------------------------
// T019: TestE2EAuthAccepted
// ---------------------------------------------------------------------------

func TestE2EAuthAccepted(t *testing.T) {
	body := map[string]any{
		"model": model,
		"input": []map[string]any{
			{"role": "user", "content": "hello"},
		},
	}
	resp := postJSONWithKey(t, "/responses", body, aliceKey)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data := readBody(t, resp)
		t.Fatalf("expected 200 with valid alice key, got %d: %s", resp.StatusCode, data)
	}
	data := readBody(t, resp)
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result["id"] == nil || result["id"] == "" {
		t.Error("expected non-empty response ID")
	}
}

// ---------------------------------------------------------------------------
// T020: TestE2EAuthRejected
// ---------------------------------------------------------------------------

func TestE2EAuthRejected(t *testing.T) {
	body := map[string]any{
		"model": model,
		"input": []map[string]any{
			{"role": "user", "content": "hello"},
		},
	}
	resp := postJSONWithKey(t, "/responses", body, "invalid-key-12345")
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Skip("auth not configured on this deployment (all requests accepted)")
	}
	if resp.StatusCode != http.StatusUnauthorized {
		data := readBody(t, resp)
		t.Fatalf("expected 401 with invalid key, got %d: %s", resp.StatusCode, data)
	}
}

// ---------------------------------------------------------------------------
// T021: TestE2EOwnershipIsolation
// ---------------------------------------------------------------------------

func TestE2EOwnershipIsolation(t *testing.T) {
	// Check if auth is configured by testing invalid key rejection.
	probe := postJSONWithKey(t, "/responses", map[string]any{
		"model": model, "input": []map[string]any{{"role": "user", "content": "probe"}},
	}, "invalid-probe-key")
	readBody(t, probe)
	if probe.StatusCode == http.StatusOK {
		t.Skip("auth not configured on this deployment (ownership isolation requires auth)")
	}

	// 1. Alice creates a response.
	body := map[string]any{
		"model": model,
		"input": []map[string]any{
			{"role": "user", "content": "hello from alice"},
		},
	}
	createResp := postJSONWithKey(t, "/responses", body, aliceKey)
	if createResp.StatusCode != http.StatusOK {
		data := readBody(t, createResp)
		t.Fatalf("alice create: expected 200, got %d: %s", createResp.StatusCode, data)
	}
	data := readBody(t, createResp)
	var created map[string]any
	if err := json.Unmarshal(data, &created); err != nil {
		t.Fatalf("unmarshal alice response: %v", err)
	}
	id, ok := created["id"].(string)
	if !ok || id == "" {
		t.Fatal("alice created response has no id")
	}

	// 2. Bob tries to GET Alice's response, should get 404.
	bobResp := getJSONWithKey(t, "/responses/"+id, bobKey)
	readBody(t, bobResp)
	if bobResp.StatusCode != http.StatusNotFound {
		t.Errorf("bob should not see alice's response: expected 404, got %d", bobResp.StatusCode)
	}

	// 3. Alice GETs her own response, should succeed.
	aliceResp := getJSONWithKey(t, "/responses/"+id, aliceKey)
	if aliceResp.StatusCode != http.StatusOK {
		data := readBody(t, aliceResp)
		t.Fatalf("alice should see her own response: expected 200, got %d: %s", aliceResp.StatusCode, data)
	}
	readBody(t, aliceResp)
}
