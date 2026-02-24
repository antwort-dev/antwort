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

func TestPassthroughFieldsRoundTrip(t *testing.T) {
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
		"metadata":            map[string]any{"env": "test", "version": "1.0"},
		"user":                "test-user-123",
		"frequency_penalty":   0.5,
		"presence_penalty":    0.3,
		"top_logprobs":        3,
		"parallel_tool_calls": false,
		"max_tool_calls":      5,
	}

	resp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var raw map[string]json.RawMessage
	decodeJSON(t, resp, &raw)

	// Verify metadata round-trip.
	if _, ok := raw["metadata"]; !ok {
		t.Error("metadata missing from response")
	} else {
		var meta map[string]any
		json.Unmarshal(raw["metadata"], &meta)
		if meta["env"] != "test" {
			t.Errorf("metadata.env = %v, want 'test'", meta["env"])
		}
	}

	// Verify user round-trip.
	if _, ok := raw["user"]; !ok {
		t.Error("user missing from response")
	} else {
		var user string
		json.Unmarshal(raw["user"], &user)
		if user != "test-user-123" {
			t.Errorf("user = %q, want 'test-user-123'", user)
		}
	}

	// Verify frequency_penalty.
	if _, ok := raw["frequency_penalty"]; !ok {
		t.Error("frequency_penalty missing from response")
	} else {
		var fp float64
		json.Unmarshal(raw["frequency_penalty"], &fp)
		if fp != 0.5 {
			t.Errorf("frequency_penalty = %v, want 0.5", fp)
		}
	}

	// Verify presence_penalty.
	if _, ok := raw["presence_penalty"]; !ok {
		t.Error("presence_penalty missing from response")
	} else {
		var pp float64
		json.Unmarshal(raw["presence_penalty"], &pp)
		if pp != 0.3 {
			t.Errorf("presence_penalty = %v, want 0.3", pp)
		}
	}

	// Verify top_logprobs.
	if _, ok := raw["top_logprobs"]; !ok {
		t.Error("top_logprobs missing from response")
	} else {
		var tl float64
		json.Unmarshal(raw["top_logprobs"], &tl)
		if tl != 3 {
			t.Errorf("top_logprobs = %v, want 3", tl)
		}
	}

	// Verify parallel_tool_calls echoed as false.
	if _, ok := raw["parallel_tool_calls"]; !ok {
		t.Error("parallel_tool_calls missing from response")
	} else {
		var ptc bool
		json.Unmarshal(raw["parallel_tool_calls"], &ptc)
		if ptc != false {
			t.Errorf("parallel_tool_calls = %v, want false", ptc)
		}
	}

	// Verify max_tool_calls.
	if _, ok := raw["max_tool_calls"]; !ok {
		t.Error("max_tool_calls missing from response")
	} else {
		var mtc float64
		json.Unmarshal(raw["max_tool_calls"], &mtc)
		if mtc != 5 {
			t.Errorf("max_tool_calls = %v, want 5", mtc)
		}
	}
}

func TestIncludeFilterOmitsUsage(t *testing.T) {
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
		// Only include reasoning, explicitly exclude usage.
		"include": []string{"reasoning"},
	}

	resp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var raw map[string]json.RawMessage
	decodeJSON(t, resp, &raw)

	// Usage should be nil/absent when not in include list.
	if usageRaw, ok := raw["usage"]; ok {
		if string(usageRaw) != "null" {
			t.Errorf("usage should be null when not in include list, got %s", string(usageRaw))
		}
	}
}

func TestIncludeFilterDefaultIncludesEverything(t *testing.T) {
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
		// No include field: all sections should be present.
	}

	resp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var raw map[string]json.RawMessage
	decodeJSON(t, resp, &raw)

	// Usage should be present when include is omitted.
	if usageRaw, ok := raw["usage"]; !ok {
		t.Error("usage missing from response when include is omitted")
	} else if string(usageRaw) == "null" {
		t.Error("usage should not be null when include is omitted")
	}
}

func TestNonStreamingReasoningItem(t *testing.T) {
	reqBody := map[string]any{
		"model": "mock-model",
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
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var response api.Response
	decodeJSON(t, resp, &response)

	// Find the reasoning item in output.
	foundReasoning := false
	reasoningIdx := -1
	textIdx := -1
	for i, item := range response.Output {
		if item.Type == api.ItemTypeReasoning {
			foundReasoning = true
			reasoningIdx = i
			if item.Reasoning == nil {
				t.Error("reasoning item has nil Reasoning data")
			} else if item.Reasoning.Content == "" {
				t.Error("reasoning item has empty content")
			} else {
				t.Logf("reasoning content: %q", item.Reasoning.Content)
			}
		}
		if item.Type == api.ItemTypeMessage {
			textIdx = i
		}
	}

	if !foundReasoning {
		t.Error("no reasoning item found in output")
	}

	// Reasoning should come before text (FR-006).
	if reasoningIdx >= 0 && textIdx >= 0 && reasoningIdx >= textIdx {
		t.Errorf("reasoning item (idx %d) should appear before text item (idx %d)",
			reasoningIdx, textIdx)
	}
}

func TestNonStreamingNoReasoningForNonReasoningModel(t *testing.T) {
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

	for _, item := range response.Output {
		if item.Type == api.ItemTypeReasoning {
			t.Error("unexpected reasoning item in non-reasoning response")
		}
	}
}
