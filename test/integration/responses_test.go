package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
)

func TestPostResponseNonStreaming(t *testing.T) {
	reqBody := map[string]any{
		"model": "mock-model",
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
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var response api.Response
	decodeJSON(t, resp, &response)

	// Verify required fields.
	if response.ID == "" {
		t.Error("response ID is empty")
	}
	if !api.ValidateResponseID(response.ID) {
		t.Errorf("invalid response ID format: %s", response.ID)
	}
	if response.Object != "response" {
		t.Errorf("object = %q, want %q", response.Object, "response")
	}
	if response.Status != api.ResponseStatusCompleted {
		t.Errorf("status = %q, want %q", response.Status, api.ResponseStatusCompleted)
	}
	if response.Model == "" {
		t.Error("model is empty")
	}
	if response.CreatedAt == 0 {
		t.Error("created_at is zero")
	}

	// Verify output.
	if len(response.Output) == 0 {
		t.Fatal("output is empty")
	}

	outputItem := response.Output[0]
	if outputItem.Type != api.ItemTypeMessage {
		t.Errorf("output[0].type = %q, want %q", outputItem.Type, api.ItemTypeMessage)
	}
	if outputItem.Status != api.ItemStatusCompleted {
		t.Errorf("output[0].status = %q, want %q", outputItem.Status, api.ItemStatusCompleted)
	}

	// Verify usage.
	if response.Usage == nil {
		t.Error("usage is nil")
	} else {
		if response.Usage.TotalTokens == 0 {
			t.Error("usage.total_tokens is zero")
		}
	}
}

func TestGetResponse(t *testing.T) {
	// First create a response with store=true.
	reqBody := map[string]any{
		"model": "mock-model",
		"store": true,
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

	createResp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
	if createResp.StatusCode != http.StatusOK {
		body := readBody(t, createResp)
		t.Fatalf("create: expected 200, got %d: %s", createResp.StatusCode, body)
	}

	var created api.Response
	decodeJSON(t, createResp, &created)

	// Now retrieve it.
	getResp := getURL(t, testEnv.BaseURL()+"/v1/responses/"+created.ID)
	if getResp.StatusCode != http.StatusOK {
		body := readBody(t, getResp)
		t.Fatalf("get: expected 200, got %d: %s", getResp.StatusCode, body)
	}

	var retrieved api.Response
	decodeJSON(t, getResp, &retrieved)

	if retrieved.ID != created.ID {
		t.Errorf("retrieved ID = %q, want %q", retrieved.ID, created.ID)
	}
	if retrieved.Status != api.ResponseStatusCompleted {
		t.Errorf("retrieved status = %q, want %q", retrieved.Status, api.ResponseStatusCompleted)
	}
}

func TestDeleteResponse(t *testing.T) {
	// Create a stored response.
	reqBody := map[string]any{
		"model": "mock-model",
		"store": true,
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

	createResp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
	if createResp.StatusCode != http.StatusOK {
		body := readBody(t, createResp)
		t.Fatalf("create: expected 200, got %d: %s", createResp.StatusCode, body)
	}

	var created api.Response
	decodeJSON(t, createResp, &created)

	// Delete it.
	delResp := deleteURL(t, testEnv.BaseURL()+"/v1/responses/"+created.ID)
	if delResp.StatusCode != http.StatusNoContent {
		body := readBody(t, delResp)
		t.Fatalf("delete: expected 204, got %d: %s", delResp.StatusCode, body)
	}
	delResp.Body.Close()

	// Verify it's gone.
	getResp := getURL(t, testEnv.BaseURL()+"/v1/responses/"+created.ID)
	if getResp.StatusCode != http.StatusNotFound {
		body := readBody(t, getResp)
		t.Errorf("get after delete: expected 404, got %d: %s", getResp.StatusCode, body)
	} else {
		getResp.Body.Close()
	}
}

func TestResponseFieldValidation(t *testing.T) {
	reqBody := map[string]any{
		"model": "mock-model",
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
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	// Decode as raw JSON to validate field presence.
	var raw map[string]json.RawMessage
	decodeJSON(t, resp, &raw)

	requiredFields := []string{
		"id", "object", "created_at", "status", "model",
		"output", "tools", "tool_choice", "truncation",
		"parallel_tool_calls", "temperature", "top_p",
		"top_logprobs", "presence_penalty", "frequency_penalty",
		"store", "background", "service_tier", "metadata",
	}

	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("required field %q missing from response", field)
		}
	}
}

func TestResponseOutputFormat(t *testing.T) {
	// Verify the flat wire format for output items.
	reqBody := map[string]any{
		"model": "mock-model",
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
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var raw map[string]json.RawMessage
	decodeJSON(t, resp, &raw)

	// Parse output items as raw JSON.
	var outputItems []map[string]json.RawMessage
	if err := json.Unmarshal(raw["output"], &outputItems); err != nil {
		t.Fatalf("parsing output: %v", err)
	}

	if len(outputItems) == 0 {
		t.Fatal("output is empty")
	}

	item := outputItems[0]

	// Flat wire format should have type, id, status, role, content at top level.
	for _, field := range []string{"type", "id", "status", "role", "content"} {
		if _, ok := item[field]; !ok {
			t.Errorf("output item missing flat field %q", field)
		}
	}

	// Should NOT have nested "message" wrapper.
	if _, ok := item["message"]; ok {
		t.Error("output item has nested 'message' wrapper (should be flat wire format)")
	}
}
