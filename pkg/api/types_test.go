package api

import (
	"encoding/json"
	"reflect"
"testing"
)

// roundTrip marshals v to JSON, then unmarshals back into a new value of the
// same type and returns it. It fails the test on any error.
func roundTrip[T any](t *testing.T, v T) T {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	var got T
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal error: %v\nJSON: %s", err, data)
	}
	return got
}

// assertDeepEqual fails the test if got and want are not deeply equal.
func assertDeepEqual(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

// ---------------------------------------------------------------------------
// TestItemRoundTrip
// ---------------------------------------------------------------------------

func TestItemRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		item Item
	}{
		{
			name: "message/user with ContentPart input",
			item: Item{
				ID:     "item-001",
				Type:   ItemTypeMessage,
				Status: ItemStatusCompleted,
				Message: &MessageData{
					Role: RoleUser,
					Content: []ContentPart{
						{Type: "input_text", Text: "Hello, world!"},
					},
				},
			},
		},
		{
			name: "message/assistant with OutputContentPart including annotations and logprobs",
			item: Item{
				ID:     "item-002",
				Type:   ItemTypeMessage,
				Status: ItemStatusCompleted,
				Message: &MessageData{
					Role: RoleAssistant,
					Output: []OutputContentPart{
						{
							Type: "output_text",
							Text: "Here is the answer.",
							Annotations: []Annotation{
								{
									Type:       "url_citation",
									Text:       "source",
									StartIndex: 0,
									EndIndex:   6,
								},
							},
							Logprobs: []TokenLogprob{
								{
									Token:   "Here",
									Logprob: -0.123,
									TopLogprobs: []TopLogprob{
										{Token: "Here", Logprob: -0.123},
										{Token: "The", Logprob: -1.5},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "function_call",
			item: Item{
				ID:     "item-003",
				Type:   ItemTypeFunctionCall,
				Status: ItemStatusCompleted,
				FunctionCall: &FunctionCallData{
					Name:      "get_weather",
					CallID:    "call_abc123",
					Arguments: `{"location":"Berlin"}`,
				},
			},
		},
		{
			name: "function_call_output",
			item: Item{
				ID:     "item-004",
				Type:   ItemTypeFunctionCallOutput,
				Status: ItemStatusCompleted,
				FunctionCallOutput: &FunctionCallOutputData{
					CallID: "call_abc123",
					Output: `{"temp":20,"unit":"celsius"}`,
				},
			},
		},
		{
			name: "reasoning",
			item: Item{
				ID:     "item-005",
				Type:   ItemTypeReasoning,
				Status: ItemStatusCompleted,
				Reasoning: &ReasoningData{
					Content:          "Let me think about this...",
					EncryptedContent: "enc-abc123",
					Summary:          "Considered options A and B",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := roundTrip(t, tc.item)
			assertDeepEqual(t, got, tc.item)
		})
	}
}

// ---------------------------------------------------------------------------
// TestContentPartRoundTrip
// ---------------------------------------------------------------------------

func TestContentPartRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		part ContentPart
	}{
		{
			name: "input_text",
			part: ContentPart{Type: "input_text", Text: "Some user text"},
		},
		{
			name: "input_image with url",
			part: ContentPart{Type: "input_image", URL: "https://example.com/image.png"},
		},
		{
			name: "input_audio with data and media_type",
			part: ContentPart{
				Type:      "input_audio",
				Data:      "base64encodedaudiodata==",
				MediaType: "audio/wav",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := roundTrip(t, tc.part)
			assertDeepEqual(t, got, tc.part)
		})
	}
}

// ---------------------------------------------------------------------------
// TestOutputContentPartRoundTrip
// ---------------------------------------------------------------------------

func TestOutputContentPartRoundTrip(t *testing.T) {
	part := OutputContentPart{
		Type: "output_text",
		Text: "The capital of France is Paris.",
		Annotations: []Annotation{
			{
				Type:       "url_citation",
				Text:       "Wikipedia",
				StartIndex: 27,
				EndIndex:   32,
			},
		},
		Logprobs: []TokenLogprob{
			{
				Token:   "The",
				Logprob: -0.05,
				TopLogprobs: []TopLogprob{
					{Token: "The", Logprob: -0.05},
					{Token: "A", Logprob: -3.2},
				},
			},
			{
				Token:   " capital",
				Logprob: -0.12,
			},
		},
	}

	got := roundTrip(t, part)
	assertDeepEqual(t, got, part)
}

// ---------------------------------------------------------------------------
// TestToolChoiceRoundTrip
// ---------------------------------------------------------------------------

func TestToolChoiceRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		choice ToolChoice
	}{
		{name: "auto", choice: ToolChoiceAuto},
		{name: "required", choice: ToolChoiceRequired},
		{name: "none", choice: ToolChoiceNone},
		{
			name:   "function object",
			choice: NewToolChoiceFunction("get_weather"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := roundTrip(t, tc.choice)
			assertDeepEqual(t, got, tc.choice)
		})
	}
}

// ---------------------------------------------------------------------------
// TestCreateResponseRequestRoundTrip
// ---------------------------------------------------------------------------

func TestCreateResponseRequestRoundTrip(t *testing.T) {
	tc := ToolChoiceRequired
	req := CreateResponseRequest{
		Model: "gpt-4o",
		Input: []Item{
			{
				ID:   "msg-1",
				Type: ItemTypeMessage,
				Message: &MessageData{
					Role:    RoleUser,
					Content: []ContentPart{{Type: "input_text", Text: "Hi"}},
				},
			},
		},
		Instructions: "Be concise.",
		Tools: []ToolDefinition{
			{
				Type:        "function",
				Name:        "get_weather",
				Description: "Get current weather",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`),
			},
		},
		ToolChoice:         &tc,
		AllowedTools:       []string{"get_weather"},
		Store:              boolPtr(true),
		Stream:             true,
		PreviousResponseID: "resp-prev-001",
		Truncation:         "auto",
		ServiceTier:        "default",
		MaxOutputTokens:    intPtr(1024),
		Temperature:        float64Ptr(0.7),
		TopP:               float64Ptr(0.9),
		Extensions: map[string]json.RawMessage{
			"acme:telemetry": json.RawMessage(`{"trace_id":"abc"}`),
		},
	}

	got := roundTrip(t, req)
	assertDeepEqual(t, got, req)
}

// ---------------------------------------------------------------------------
// TestResponseRoundTrip
// ---------------------------------------------------------------------------

func TestResponseRoundTrip(t *testing.T) {
	prevID := "resp-prev-000"
	resp := Response{
		ID:     "resp-001",
		Object: "response",
		Status: ResponseStatusCompleted,
		Output: []Item{
			{
				ID:     "item-out-1",
				Type:   ItemTypeMessage,
				Status: ItemStatusCompleted,
				Message: &MessageData{
					Role: RoleAssistant,
					Output: []OutputContentPart{
						{Type: "output_text", Text: "Hello!"},
					},
				},
			},
		},
		Model: "gpt-4o",
		Usage: &Usage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
		Error: &APIError{
			Type:    ErrorTypeServerError,
			Code:    "internal",
			Param:   "input",
			Message: "something went wrong",
		},
		PreviousResponseID: &prevID,
		CreatedAt:          1700000000,
		Extensions: map[string]json.RawMessage{
			"acme:metrics": json.RawMessage(`{"latency_ms":42}`),
		},
	}

	got := roundTrip(t, resp)
	assertDeepEqual(t, got, resp)
}

// ---------------------------------------------------------------------------
// TestOmitEmptyBehavior
// ---------------------------------------------------------------------------

func TestOmitEmptyBehavior(t *testing.T) {
	// An Item with only required fields set (Type). All optional pointer/slice
	// fields should be omitted from the JSON output.
	item := Item{
		Type: ItemTypeMessage,
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	jsonStr := string(data)

	// Check that optional keys are absent by looking for them as JSON object keys.
	// We unmarshal to a map and check keys directly rather than doing substring
	// matching, which would false-positive on the type value "message".
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal into map error: %v", err)
	}

	// Keys that use omitempty and should be absent when zero-valued.
	absentKeys := []string{
		"message",
		"function_call",
		"function_call_output",
		"reasoning",
		"extension",
	}

	for _, key := range absentKeys {
		if _, ok := m[key]; ok {
			t.Errorf("expected key %q to be absent from JSON, got: %s", key, jsonStr)
		}
	}

	// id and status are always present per OpenAPI contract (no omitempty).
	requiredKeys := []string{"type", "id", "status"}
	for _, key := range requiredKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("expected required key %q to be present in JSON, got: %s", key, jsonStr)
		}
	}

	if len(m) != 3 {
		t.Errorf("expected 3 keys in JSON, got %d: %v", len(m), m)
	}
}

// ---------------------------------------------------------------------------
// TestIsExtensionType
// ---------------------------------------------------------------------------

func TestIsExtensionType(t *testing.T) {
	tests := []struct {
		name     string
		itemType ItemType
		want     bool
	}{
		{name: "message is not extension", itemType: ItemTypeMessage, want: false},
		{name: "function_call is not extension", itemType: ItemTypeFunctionCall, want: false},
		{name: "function_call_output is not extension", itemType: ItemTypeFunctionCallOutput, want: false},
		{name: "reasoning is not extension", itemType: ItemTypeReasoning, want: false},
		{name: "acme:telemetry is extension", itemType: "acme:telemetry", want: true},
		{name: "vendor:custom_type is extension", itemType: "vendor:custom_type", want: true},
		{name: "x:y is extension", itemType: "x:y", want: true},
		{name: "empty string is not extension", itemType: "", want: false},
		{name: "no colon is not extension", itemType: "custom", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsExtensionType(tc.itemType)
			if got != tc.want {
				t.Errorf("IsExtensionType(%q) = %v, want %v", tc.itemType, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Extension round-trip tests (T022)
// ---------------------------------------------------------------------------

func TestExtensionItemRoundTrip(t *testing.T) {
	extData := json.RawMessage(`{"trace_id":"abc123","duration_ms":42}`)
	item := Item{
		ID:        "item_ext001ext001ext001ext0",
		Type:      "acme:telemetry_chunk",
		Status:    ItemStatusCompleted,
		Extension: extData,
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var got Item
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if string(got.Extension) != string(extData) {
		t.Errorf("Extension data lost: got %s, want %s", string(got.Extension), string(extData))
	}
	if got.Type != "acme:telemetry_chunk" {
		t.Errorf("Type = %q, want %q", got.Type, "acme:telemetry_chunk")
	}
}

func TestRequestExtensionsRoundTrip(t *testing.T) {
	req := CreateResponseRequest{
		Model: "test-model",
		Input: []Item{{Type: ItemTypeMessage, Message: &MessageData{Role: RoleUser}}},
		Extensions: map[string]json.RawMessage{
			"acme:config": json.RawMessage(`{"mode":"fast","retries":3}`),
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var got CreateResponseRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if string(got.Extensions["acme:config"]) != `{"mode":"fast","retries":3}` {
		t.Errorf("Extensions lost: got %s", string(got.Extensions["acme:config"]))
	}
}

func TestResponseExtensionsRoundTrip(t *testing.T) {
	resp := Response{
		ID:        "resp_test001test001test001te",
		Object:    "response",
		Status:    ResponseStatusCompleted,
		Model:     "test-model",
		CreatedAt: 1700000000,
		Extensions: map[string]json.RawMessage{
			"acme:metrics": json.RawMessage(`{"latency_ms":150}`),
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var got Response
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if string(got.Extensions["acme:metrics"]) != `{"latency_ms":150}` {
		t.Errorf("Extensions lost: got %s", string(got.Extensions["acme:metrics"]))
	}
}
