package engine

import (
	"context"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/transport"
)

// mockStore implements transport.ResponseStore for testing.
type mockStore struct {
	responses map[string]*api.Response
}

var _ transport.ResponseStore = (*mockStore)(nil)

func (s *mockStore) SaveResponse(_ context.Context, resp *api.Response) error {
	if s.responses == nil {
		s.responses = make(map[string]*api.Response)
	}
	s.responses[resp.ID] = resp
	return nil
}

func (s *mockStore) GetResponse(_ context.Context, id string) (*api.Response, error) {
	if resp, ok := s.responses[id]; ok {
		return resp, nil
	}
	return nil, api.NewNotFoundError("response " + id + " not found")
}

func (s *mockStore) GetResponseForChain(_ context.Context, id string) (*api.Response, error) {
	return s.GetResponse(context.Background(), id)
}

func (s *mockStore) DeleteResponse(_ context.Context, id string) error {
	return nil
}

func (s *mockStore) HealthCheck(_ context.Context) error { return nil }
func (s *mockStore) Close() error                        { return nil }

func TestLoadConversationHistory_ChainOfThree(t *testing.T) {
	respA := "resp_A"
	respB := "resp_B"
	store := &mockStore{
		responses: map[string]*api.Response{
			"resp_A": {
				ID:     "resp_A",
				Input:  []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Hello"}}}}},
				Output: []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleAssistant, Output: []api.OutputContentPart{{Type: "output_text", Text: "Hi there!"}}}}},
			},
			"resp_B": {
				ID:                 "resp_B",
				PreviousResponseID: &respA,
				Input:              []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "How are you?"}}}}},
				Output:             []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleAssistant, Output: []api.OutputContentPart{{Type: "output_text", Text: "I am fine."}}}}},
			},
			"resp_C": {
				ID:                 "resp_C",
				PreviousResponseID: &respB,
				Input:              []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Goodbye"}}}}},
				Output:             []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleAssistant, Output: []api.OutputContentPart{{Type: "output_text", Text: "See you!"}}}}},
			},
		},
	}

	msgs, err := loadConversationHistory(context.Background(), store, "resp_C")
	if err != nil {
		t.Fatalf("loadConversationHistory failed: %v", err)
	}

	// Should have 6 messages: A.input, A.output, B.input, B.output, C.input, C.output.
	if len(msgs) != 6 {
		t.Fatalf("expected 6 messages, got %d", len(msgs))
	}

	// Verify chronological order.
	expected := []struct {
		role    string
		content string
	}{
		{"user", "Hello"},
		{"assistant", "Hi there!"},
		{"user", "How are you?"},
		{"assistant", "I am fine."},
		{"user", "Goodbye"},
		{"assistant", "See you!"},
	}

	for i, exp := range expected {
		if msgs[i].Role != exp.role {
			t.Errorf("msg[%d] role = %q, want %q", i, msgs[i].Role, exp.role)
		}
		content, _ := msgs[i].Content.(string)
		if content != exp.content {
			t.Errorf("msg[%d] content = %q, want %q", i, content, exp.content)
		}
	}
}

func TestLoadConversationHistory_CycleDetection(t *testing.T) {
	respA := "resp_A"
	respB := "resp_B"
	store := &mockStore{
		responses: map[string]*api.Response{
			"resp_A": {ID: "resp_A", PreviousResponseID: &respB},
			"resp_B": {ID: "resp_B", PreviousResponseID: &respA},
		},
	}

	_, err := loadConversationHistory(context.Background(), store, "resp_A")
	if err == nil {
		t.Fatal("expected error for cycle")
	}

	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *api.APIError, got %T", err)
	}
	if apiErr.Type != api.ErrorTypeInvalidRequest {
		t.Errorf("error type = %q, want %q", apiErr.Type, api.ErrorTypeInvalidRequest)
	}
}

func TestLoadConversationHistory_NilStore(t *testing.T) {
	_, err := loadConversationHistory(context.Background(), nil, "resp_1")
	if err == nil {
		t.Fatal("expected error for nil store")
	}

	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *api.APIError, got %T", err)
	}
	if apiErr.Type != api.ErrorTypeInvalidRequest {
		t.Errorf("error type = %q, want %q", apiErr.Type, api.ErrorTypeInvalidRequest)
	}
}

func TestLoadConversationHistory_NotFound(t *testing.T) {
	store := &mockStore{responses: map[string]*api.Response{}}

	_, err := loadConversationHistory(context.Background(), store, "resp_missing")
	if err == nil {
		t.Fatal("expected error for missing response")
	}
}

func TestLoadConversationHistory_SingleResponse(t *testing.T) {
	store := &mockStore{
		responses: map[string]*api.Response{
			"resp_1": {
				ID:     "resp_1",
				Input:  []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Hi"}}}}},
				Output: []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleAssistant, Output: []api.OutputContentPart{{Type: "output_text", Text: "Hello"}}}}},
			},
		},
	}

	msgs, err := loadConversationHistory(context.Background(), store, "resp_1")
	if err != nil {
		t.Fatalf("loadConversationHistory failed: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestLoadConversationHistory_FunctionCallItems(t *testing.T) {
	resp1 := "resp_1"
	store := &mockStore{
		responses: map[string]*api.Response{
			"resp_1": {
				ID:    "resp_1",
				Input: []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Weather?"}}}}},
				Output: []api.Item{
					{Type: api.ItemTypeFunctionCall, FunctionCall: &api.FunctionCallData{Name: "get_weather", CallID: "call_1", Arguments: `{"city":"Berlin"}`}},
				},
			},
			"resp_2": {
				ID:                 "resp_2",
				PreviousResponseID: &resp1,
				Input: []api.Item{
					{Type: api.ItemTypeFunctionCallOutput, FunctionCallOutput: &api.FunctionCallOutputData{CallID: "call_1", Output: "22°C, sunny"}},
				},
				Output: []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleAssistant, Output: []api.OutputContentPart{{Type: "output_text", Text: "It's 22°C and sunny in Berlin."}}}}},
			},
		},
	}

	msgs, err := loadConversationHistory(context.Background(), store, "resp_2")
	if err != nil {
		t.Fatalf("loadConversationHistory failed: %v", err)
	}

	// Should have: user msg, assistant tool call, tool result, assistant response.
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}

	// Verify tool call message.
	if msgs[1].Role != "assistant" || len(msgs[1].ToolCalls) != 1 {
		t.Errorf("expected assistant message with 1 tool call, got role=%q, tool_calls=%d", msgs[1].Role, len(msgs[1].ToolCalls))
	}
	if msgs[1].ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("tool call name = %q, want %q", msgs[1].ToolCalls[0].Function.Name, "get_weather")
	}

	// Verify tool result message.
	if msgs[2].Role != "tool" {
		t.Errorf("expected tool role, got %q", msgs[2].Role)
	}
	if msgs[2].ToolCallID != "call_1" {
		t.Errorf("tool_call_id = %q, want %q", msgs[2].ToolCallID, "call_1")
	}
}

func TestLoadConversationHistory_ReasoningSkipped(t *testing.T) {
	store := &mockStore{
		responses: map[string]*api.Response{
			"resp_1": {
				ID:    "resp_1",
				Input: []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "Think"}}}}},
				Output: []api.Item{
					{Type: api.ItemTypeReasoning, Reasoning: &api.ReasoningData{Content: "thinking..."}},
					{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleAssistant, Output: []api.OutputContentPart{{Type: "output_text", Text: "Done"}}}},
				},
			},
		},
	}

	msgs, err := loadConversationHistory(context.Background(), store, "resp_1")
	if err != nil {
		t.Fatalf("loadConversationHistory failed: %v", err)
	}

	// Reasoning items should be skipped.
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (reasoning skipped), got %d", len(msgs))
	}
}
