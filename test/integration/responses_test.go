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

// --- List Responses tests (Spec 028) ---

func TestListResponsesEmpty(t *testing.T) {
	// Use a fresh server with an empty store to avoid interference.
	// The shared test environment may have stored responses from other tests,
	// so we just verify the endpoint returns a valid list response.
	resp := getURL(t, testEnv.BaseURL()+"/v1/responses?model=nonexistent-model-xyz")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var list map[string]json.RawMessage
	decodeJSON(t, resp, &list)

	if string(list["object"]) != `"list"` {
		t.Errorf("object = %s, want \"list\"", list["object"])
	}

	var hasMore bool
	json.Unmarshal(list["has_more"], &hasMore)
	if hasMore {
		t.Error("has_more should be false for empty list")
	}
}

func TestListResponsesWithResults(t *testing.T) {
	// Create two stored responses.
	for i := 0; i < 2; i++ {
		reqBody := map[string]any{
			"model": "mock-model",
			"store": true,
			"input": []map[string]any{
				{
					"type": "message",
					"role": "user",
					"content": []map[string]any{
						{"type": "input_text", "text": "list test"},
					},
				},
			},
		}
		createResp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
		if createResp.StatusCode != http.StatusOK {
			body := readBody(t, createResp)
			t.Fatalf("create %d: expected 200, got %d: %s", i, createResp.StatusCode, body)
		}
		createResp.Body.Close()
	}

	// List all responses.
	resp := getURL(t, testEnv.BaseURL()+"/v1/responses")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("list: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var list struct {
		Object  string            `json:"object"`
		Data    []json.RawMessage `json:"data"`
		HasMore bool              `json:"has_more"`
		FirstID string            `json:"first_id"`
		LastID  string            `json:"last_id"`
	}
	decodeJSON(t, resp, &list)

	if list.Object != "list" {
		t.Errorf("object = %q, want %q", list.Object, "list")
	}
	if len(list.Data) < 2 {
		t.Fatalf("expected at least 2 responses, got %d", len(list.Data))
	}
	if list.FirstID == "" {
		t.Error("first_id is empty")
	}
	if list.LastID == "" {
		t.Error("last_id is empty")
	}
}

func TestListResponsesPagination(t *testing.T) {
	// Create 3 stored responses.
	var ids []string
	for i := 0; i < 3; i++ {
		reqBody := map[string]any{
			"model": "mock-model",
			"store": true,
			"input": []map[string]any{
				{
					"type": "message",
					"role": "user",
					"content": []map[string]any{
						{"type": "input_text", "text": "pagination test"},
					},
				},
			},
		}
		createResp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
		if createResp.StatusCode != http.StatusOK {
			body := readBody(t, createResp)
			t.Fatalf("create %d: expected 200, got %d: %s", i, createResp.StatusCode, body)
		}
		var created struct{ ID string `json:"id"` }
		decodeJSON(t, createResp, &created)
		ids = append(ids, created.ID)
	}

	// List with limit=1 to test pagination.
	resp := getURL(t, testEnv.BaseURL()+"/v1/responses?limit=1")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("list: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var page1 struct {
		Data    []struct{ ID string `json:"id"` } `json:"data"`
		HasMore bool                               `json:"has_more"`
		LastID  string                             `json:"last_id"`
	}
	decodeJSON(t, resp, &page1)

	if len(page1.Data) != 1 {
		t.Fatalf("page 1: expected 1 response, got %d", len(page1.Data))
	}
	if !page1.HasMore {
		t.Error("page 1: has_more should be true")
	}

	// Get page 2 using after cursor.
	resp2 := getURL(t, testEnv.BaseURL()+"/v1/responses?limit=1&after="+page1.LastID)
	if resp2.StatusCode != http.StatusOK {
		body := readBody(t, resp2)
		t.Fatalf("page 2: expected 200, got %d: %s", resp2.StatusCode, body)
	}

	var page2 struct {
		Data []struct{ ID string `json:"id"` } `json:"data"`
	}
	decodeJSON(t, resp2, &page2)

	if len(page2.Data) != 1 {
		t.Fatalf("page 2: expected 1 response, got %d", len(page2.Data))
	}
	// Page 2 should have a different ID than page 1.
	if page2.Data[0].ID == page1.Data[0].ID {
		t.Error("page 2 returned the same response as page 1")
	}
}

func TestListResponsesModelFilter(t *testing.T) {
	// List responses filtered by model.
	resp := getURL(t, testEnv.BaseURL()+"/v1/responses?model=mock-model")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("list: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var list struct {
		Data []struct{ Model string `json:"model"` } `json:"data"`
	}
	decodeJSON(t, resp, &list)

	for i, r := range list.Data {
		if r.Model != "mock-model" {
			t.Errorf("data[%d].model = %q, want %q", i, r.Model, "mock-model")
		}
	}

	// Filter by non-existent model.
	resp2 := getURL(t, testEnv.BaseURL()+"/v1/responses?model=nonexistent")
	if resp2.StatusCode != http.StatusOK {
		body := readBody(t, resp2)
		t.Fatalf("list nonexistent: expected 200, got %d: %s", resp2.StatusCode, body)
	}

	var emptyList struct {
		Data []json.RawMessage `json:"data"`
	}
	decodeJSON(t, resp2, &emptyList)

	if len(emptyList.Data) != 0 {
		t.Errorf("expected 0 results for nonexistent model, got %d", len(emptyList.Data))
	}
}

func TestListResponsesOrdering(t *testing.T) {
	// List in ascending order.
	respAsc := getURL(t, testEnv.BaseURL()+"/v1/responses?order=asc")
	if respAsc.StatusCode != http.StatusOK {
		body := readBody(t, respAsc)
		t.Fatalf("list asc: expected 200, got %d: %s", respAsc.StatusCode, body)
	}

	var ascList struct {
		Data []struct {
			ID        string `json:"id"`
			CreatedAt int64  `json:"created_at"`
		} `json:"data"`
	}
	decodeJSON(t, respAsc, &ascList)

	// Verify ascending order.
	for i := 1; i < len(ascList.Data); i++ {
		if ascList.Data[i].CreatedAt < ascList.Data[i-1].CreatedAt {
			t.Errorf("ascending order violated at index %d: %d < %d",
				i, ascList.Data[i].CreatedAt, ascList.Data[i-1].CreatedAt)
		}
	}
}

func TestListResponsesInvalidParams(t *testing.T) {
	// Invalid order.
	resp := getURL(t, testEnv.BaseURL()+"/v1/responses?order=invalid")
	if resp.StatusCode != http.StatusBadRequest {
		body := readBody(t, resp)
		t.Errorf("invalid order: expected 400, got %d: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}

	// Invalid limit.
	resp2 := getURL(t, testEnv.BaseURL()+"/v1/responses?limit=abc")
	if resp2.StatusCode != http.StatusBadRequest {
		body := readBody(t, resp2)
		t.Errorf("invalid limit: expected 400, got %d: %s", resp2.StatusCode, body)
	} else {
		resp2.Body.Close()
	}

	// Both after and before.
	resp3 := getURL(t, testEnv.BaseURL()+"/v1/responses?after=a&before=b")
	if resp3.StatusCode != http.StatusBadRequest {
		body := readBody(t, resp3)
		t.Errorf("both cursors: expected 400, got %d: %s", resp3.StatusCode, body)
	} else {
		resp3.Body.Close()
	}
}

// --- Input Items tests (Spec 028) ---

func TestGetInputItems(t *testing.T) {
	// Create a response with input items.
	reqBody := map[string]any{
		"model": "mock-model",
		"store": true,
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "input items test"},
				},
			},
		},
	}

	createResp := postJSON(t, testEnv.BaseURL()+"/v1/responses", reqBody)
	if createResp.StatusCode != http.StatusOK {
		body := readBody(t, createResp)
		t.Fatalf("create: expected 200, got %d: %s", createResp.StatusCode, body)
	}

	var created struct{ ID string `json:"id"` }
	decodeJSON(t, createResp, &created)

	// Get input items.
	resp := getURL(t, testEnv.BaseURL()+"/v1/responses/"+created.ID+"/input_items")
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("input_items: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var list struct {
		Object  string            `json:"object"`
		Data    []json.RawMessage `json:"data"`
		HasMore bool              `json:"has_more"`
	}
	decodeJSON(t, resp, &list)

	if list.Object != "list" {
		t.Errorf("object = %q, want %q", list.Object, "list")
	}
	if len(list.Data) == 0 {
		t.Error("expected at least 1 input item")
	}
}

func TestGetInputItemsNotFound(t *testing.T) {
	resp := getURL(t, testEnv.BaseURL()+"/v1/responses/resp_aaaaaaaaaaaaaaaaaaaaaaaa/input_items")
	if resp.StatusCode != http.StatusNotFound {
		body := readBody(t, resp)
		t.Errorf("expected 404, got %d: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}
}

func TestGetInputItemsMalformedID(t *testing.T) {
	resp := getURL(t, testEnv.BaseURL()+"/v1/responses/bad-id/input_items")
	if resp.StatusCode != http.StatusBadRequest {
		body := readBody(t, resp)
		t.Errorf("expected 400, got %d: %s", resp.StatusCode, body)
	} else {
		resp.Body.Close()
	}
}

// --- Structured Output tests (Spec 029) ---

func TestStructuredOutputJsonObject(t *testing.T) {
	reqBody := map[string]any{
		"model": "mock-model",
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Give me JSON"},
				},
			},
		},
		"text": map[string]any{
			"format": map[string]any{
				"type": "json_object",
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

	// Verify text.format is echoed.
	if response.Text == nil || response.Text.Format == nil {
		t.Fatal("text.format not echoed in response")
	}
	if response.Text.Format.Type != "json_object" {
		t.Errorf("text.format.type = %q, want %q", response.Text.Format.Type, "json_object")
	}

	// Verify the output contains JSON (mock backend returns JSON when response_format is set).
	if len(response.Output) == 0 {
		t.Fatal("output is empty")
	}

	// Check the output text is valid JSON.
	outputItem := response.Output[0]
	if outputItem.Message != nil && len(outputItem.Message.Output) > 0 {
		text := outputItem.Message.Output[0].Text
		if !json.Valid([]byte(text)) {
			t.Errorf("output text is not valid JSON: %s", text)
		}
	}
}

func TestStructuredOutputJsonSchema(t *testing.T) {
	strict := true
	reqBody := map[string]any{
		"model": "mock-model",
		"input": []map[string]any{
			{
				"type": "message",
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Give me a person"},
				},
			},
		},
		"text": map[string]any{
			"format": map[string]any{
				"type": "json_schema",
				"name": "person",
				"strict": strict,
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
						"age":  map[string]any{"type": "integer"},
					},
					"required":             []string{"name", "age"},
					"additionalProperties": false,
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

	// Verify text.format is echoed with all fields.
	if response.Text == nil || response.Text.Format == nil {
		t.Fatal("text.format not echoed in response")
	}
	if response.Text.Format.Type != "json_schema" {
		t.Errorf("text.format.type = %q, want %q", response.Text.Format.Type, "json_schema")
	}
	if response.Text.Format.Name != "person" {
		t.Errorf("text.format.name = %q, want %q", response.Text.Format.Name, "person")
	}
	if response.Text.Format.Strict == nil || !*response.Text.Format.Strict {
		t.Error("text.format.strict should be true")
	}
	if response.Text.Format.Schema == nil {
		t.Error("text.format.schema should not be nil")
	}

	// Verify the output is valid JSON.
	if len(response.Output) > 0 {
		outputItem := response.Output[0]
		if outputItem.Message != nil && len(outputItem.Message.Output) > 0 {
			text := outputItem.Message.Output[0].Text
			if !json.Valid([]byte(text)) {
				t.Errorf("output text is not valid JSON: %s", text)
			}
		}
	}
}

func TestStructuredOutputDefaultText(t *testing.T) {
	// Request without text.format should not send response_format to backend.
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

	// Output should be free-form text (not JSON from response_format handler).
	if len(response.Output) > 0 {
		outputItem := response.Output[0]
		if outputItem.Message != nil && len(outputItem.Message.Output) > 0 {
			text := outputItem.Message.Output[0].Text
			// The mock returns "Hello from mock!" for normal requests.
			if text == "" {
				t.Error("output text is empty")
			}
		}
	}
}

func TestStructuredOutputExplicitText(t *testing.T) {
	// Request with text.format.type = "text" should not send response_format.
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
		"text": map[string]any{
			"format": map[string]any{
				"type": "text",
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

	// text.format should be echoed.
	if response.Text == nil || response.Text.Format == nil {
		t.Fatal("text.format not echoed")
	}
	if response.Text.Format.Type != "text" {
		t.Errorf("text.format.type = %q, want %q", response.Text.Format.Type, "text")
	}

	// Output should be free-form text (not JSON from response_format handler).
	if len(response.Output) > 0 {
		outputItem := response.Output[0]
		if outputItem.Message != nil && len(outputItem.Message.Output) > 0 {
			text := outputItem.Message.Output[0].Text
			if text == "" {
				t.Error("output text is empty")
			}
		}
	}
}

func TestNonStreamingIncompleteStatus(t *testing.T) {
	reqBody := map[string]any{
		"model": "mock-model",
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
	if resp.StatusCode != http.StatusOK {
		body := readBody(t, resp)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var response api.Response
	decodeJSON(t, resp, &response)

	if response.Status != api.ResponseStatusIncomplete {
		t.Errorf("status = %q, want %q", response.Status, api.ResponseStatusIncomplete)
	}

	if response.IncompleteDetails == nil {
		t.Error("incomplete_details is nil")
	} else if response.IncompleteDetails.Reason != "max_output_tokens" {
		t.Errorf("incomplete reason = %q, want 'max_output_tokens'", response.IncompleteDetails.Reason)
	}

	// Should still have output.
	if len(response.Output) == 0 {
		t.Error("output is empty for incomplete response")
	}
}
