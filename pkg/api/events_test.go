package api

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStreamEventDeltaRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		event StreamEvent
	}{
		{
			name: "output_text_delta",
			event: StreamEvent{
				Type:           EventOutputTextDelta,
				Delta:          "Hello ",
				ItemID:         "item_001",
				OutputIndex:    0,
				ContentIndex:   1,
				SequenceNumber: 5,
			},
		},
		{
			name: "function_call_args_delta",
			event: StreamEvent{
				Type:           EventFunctionCallArgsDelta,
				Delta:          `{"loc`,
				ItemID:         "item_002",
				OutputIndex:    1,
				ContentIndex:   0,
				SequenceNumber: 12,
			},
		},
		{
			name: "output_text_done",
			event: StreamEvent{
				Type:           EventOutputTextDone,
				Delta:          "complete text",
				ItemID:         "item_003",
				OutputIndex:    0,
				ContentIndex:   0,
				SequenceNumber: 20,
			},
		},
		{
			name: "function_call_args_done",
			event: StreamEvent{
				Type:           EventFunctionCallArgsDone,
				Delta:          `{"location":"NYC"}`,
				ItemID:         "item_004",
				OutputIndex:    2,
				ContentIndex:   0,
				SequenceNumber: 30,
			},
		},
		{
			name: "content_part_done",
			event: StreamEvent{
				Type:           EventContentPartDone,
				ItemID:         "item_005",
				OutputIndex:    0,
				ContentIndex:   0,
				SequenceNumber: 40,
			},
		},
		{
			name: "output_item_done",
			event: StreamEvent{
				Type:           EventOutputItemDone,
				ItemID:         "item_006",
				OutputIndex:    0,
				SequenceNumber: 50,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}

			var got StreamEvent
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}

			if !reflect.DeepEqual(tt.event, got) {
				t.Errorf("round-trip mismatch\nwant: %+v\ngot:  %+v", tt.event, got)
			}
		})
	}
}

func TestStreamEventStateRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		event StreamEvent
	}{
		{
			name: "response_created",
			event: StreamEvent{
				Type:           EventResponseCreated,
				SequenceNumber: 0,
				Response: &Response{
					ID:        "resp_001",
					Object:    "response",
					Status:    ResponseStatusInProgress,
					Model:     "test-model",
					CreatedAt: 1700000000,
				},
			},
		},
		{
			name: "response_completed",
			event: StreamEvent{
				Type:           EventResponseCompleted,
				SequenceNumber: 100,
				Response: &Response{
					ID:        "resp_002",
					Object:    "response",
					Status:    ResponseStatusCompleted,
					Model:     "test-model",
					Output:    []Item{},
					CreatedAt: 1700000001,
					Usage: &Usage{
						InputTokens:  10,
						OutputTokens: 25,
						TotalTokens:  35,
					},
				},
			},
		},
		{
			name: "response_failed_with_error",
			event: StreamEvent{
				Type:           EventResponseFailed,
				SequenceNumber: 50,
				Response: &Response{
					ID:        "resp_003",
					Object:    "response",
					Status:    ResponseStatusFailed,
					Model:     "test-model",
					CreatedAt: 1700000002,
					Error: &APIError{
						Type:    ErrorTypeServerError,
						Message: "internal failure",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}

			var got StreamEvent
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}

			if !reflect.DeepEqual(tt.event, got) {
				t.Errorf("round-trip mismatch\nwant: %+v\ngot:  %+v", tt.event, got)
			}
		})
	}
}

func TestStreamEventItemAdded(t *testing.T) {
	event := StreamEvent{
		Type:           EventOutputItemAdded,
		SequenceNumber: 3,
		OutputIndex:    0,
		Item: &Item{
			ID:     "item_010",
			Type:   ItemTypeMessage,
			Status: ItemStatusInProgress,
			Message: &MessageData{
				Role: RoleAssistant,
			},
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var got StreamEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(event, got) {
		t.Errorf("round-trip mismatch\nwant: %+v\ngot:  %+v", event, got)
	}

	if got.Item == nil {
		t.Fatal("expected Item to be non-nil")
	}
	if got.Item.ID != "item_010" {
		t.Errorf("expected Item.ID = %q, got %q", "item_010", got.Item.ID)
	}
	if got.Item.Type != ItemTypeMessage {
		t.Errorf("expected Item.Type = %q, got %q", ItemTypeMessage, got.Item.Type)
	}
}

func TestStreamEventContentPartAdded(t *testing.T) {
	event := StreamEvent{
		Type:           EventContentPartAdded,
		SequenceNumber: 7,
		ItemID:         "item_020",
		OutputIndex:    0,
		ContentIndex:   0,
		Part: &OutputContentPart{
			Type: "output_text",
			Text: "",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var got StreamEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if !reflect.DeepEqual(event, got) {
		t.Errorf("round-trip mismatch\nwant: %+v\ngot:  %+v", event, got)
	}

	if got.Part == nil {
		t.Fatal("expected Part to be non-nil")
	}
	if got.Part.Type != "output_text" {
		t.Errorf("expected Part.Type = %q, got %q", "output_text", got.Part.Type)
	}
}

func TestSequenceNumberSerialization(t *testing.T) {
	tests := []struct {
		name     string
		seqNum   int
		expected string
	}{
		{
			name:     "zero_value",
			seqNum:   0,
			expected: `"sequence_number":0`,
		},
		{
			name:     "positive_value",
			seqNum:   42,
			expected: `"sequence_number":42`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := StreamEvent{
				Type:           EventResponseCreated,
				SequenceNumber: tt.seqNum,
			}

			data, err := json.Marshal(event)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}

			jsonStr := string(data)
			if !containsSubstring(jsonStr, tt.expected) {
				t.Errorf("expected JSON to contain %q, got: %s", tt.expected, jsonStr)
			}
		})
	}
}

func TestIsExtensionEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    StreamEventType
		expected bool
	}{
		{
			name:     "standard_response_created",
			event:    EventResponseCreated,
			expected: false,
		},
		{
			name:     "standard_output_text_delta",
			event:    EventOutputTextDelta,
			expected: false,
		},
		{
			name:     "standard_response_completed",
			event:    EventResponseCompleted,
			expected: false,
		},
		{
			name:     "standard_output_item_added",
			event:    EventOutputItemAdded,
			expected: false,
		},
		{
			name:     "extension_acme_custom",
			event:    StreamEventType("acme:custom_event"),
			expected: true,
		},
		{
			name:     "extension_provider_metric",
			event:    StreamEventType("provider:metrics.update"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsExtensionEvent(tt.event)
			if got != tt.expected {
				t.Errorf("IsExtensionEvent(%q) = %v, want %v", tt.event, got, tt.expected)
			}
		})
	}
}

func TestStreamEventOmitEmpty(t *testing.T) {
	event := StreamEvent{
		Type:           EventResponseCreated,
		SequenceNumber: 1,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map error: %v", err)
	}

	omittedFields := []string{"response", "item", "part", "delta", "item_id"}
	for _, field := range omittedFields {
		if _, exists := raw[field]; exists {
			t.Errorf("expected field %q to be absent from JSON, but it was present", field)
		}
	}

	requiredFields := []string{"type", "sequence_number"}
	for _, field := range requiredFields {
		if _, exists := raw[field]; !exists {
			t.Errorf("expected field %q to be present in JSON, but it was absent", field)
		}
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
