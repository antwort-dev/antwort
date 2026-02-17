package engine

import (
	"context"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/transport"
)

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	name     string
	caps     provider.ProviderCapabilities
	response *provider.ProviderResponse
	err      error
}

func (m *mockProvider) Name() string                        { return m.name }
func (m *mockProvider) Capabilities() provider.ProviderCapabilities { return m.caps }
func (m *mockProvider) Complete(_ context.Context, _ *provider.ProviderRequest) (*provider.ProviderResponse, error) {
	return m.response, m.err
}
func (m *mockProvider) Stream(_ context.Context, _ *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
	return nil, nil
}
func (m *mockProvider) ListModels(_ context.Context) ([]provider.ModelInfo, error) {
	return nil, nil
}
func (m *mockProvider) Close() error { return nil }

// mockResponseWriter captures WriteResponse calls for testing.
type mockResponseWriter struct {
	response       *api.Response
	events         []api.StreamEvent
	writeRespCalls int
	writeEvtCalls  int
}

func (w *mockResponseWriter) WriteResponse(_ context.Context, resp *api.Response) error {
	w.response = resp
	w.writeRespCalls++
	return nil
}

func (w *mockResponseWriter) WriteEvent(_ context.Context, event api.StreamEvent) error {
	w.events = append(w.events, event)
	w.writeEvtCalls++
	return nil
}

func (w *mockResponseWriter) Flush() error { return nil }

// Ensure mockResponseWriter implements transport.ResponseWriter.
var _ transport.ResponseWriter = (*mockResponseWriter)(nil)

func TestEngine_CreateResponse_NonStreaming(t *testing.T) {
	mp := &mockProvider{
		name: "test",
		caps: provider.ProviderCapabilities{
			Streaming:   true,
			ToolCalling: true,
		},
		response: &provider.ProviderResponse{
			Model:  "test-model-v1",
			Status: api.ResponseStatusCompleted,
			Items: []api.Item{
				{
					Type:   api.ItemTypeMessage,
					Status: api.ItemStatusCompleted,
					Message: &api.MessageData{
						Role: api.RoleAssistant,
						Output: []api.OutputContentPart{
							{Type: "output_text", Text: "Hello there!"},
						},
					},
				},
			},
			Usage: api.Usage{
				InputTokens:  10,
				OutputTokens: 5,
				TotalTokens:  15,
			},
		},
	}

	eng, err := New(mp, nil, Config{})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model: "test-model-v1",
		Input: []api.Item{
			{
				Type: api.ItemTypeMessage,
				Message: &api.MessageData{
					Role:    api.RoleUser,
					Content: []api.ContentPart{{Type: "input_text", Text: "Hi"}},
				},
			},
		},
	}

	w := &mockResponseWriter{}
	ctx := context.Background()

	if err := eng.CreateResponse(ctx, req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	// Verify WriteResponse was called exactly once.
	if w.writeRespCalls != 1 {
		t.Errorf("expected 1 WriteResponse call, got %d", w.writeRespCalls)
	}
	if w.writeEvtCalls != 0 {
		t.Errorf("expected 0 WriteEvent calls, got %d", w.writeEvtCalls)
	}

	resp := w.response
	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	// Verify response ID has resp_ prefix.
	if !api.ValidateResponseID(resp.ID) {
		t.Errorf("expected valid response ID with resp_ prefix, got %q", resp.ID)
	}

	// Verify object type.
	if resp.Object != "response" {
		t.Errorf("expected object %q, got %q", "response", resp.Object)
	}

	// Verify status.
	if resp.Status != api.ResponseStatusCompleted {
		t.Errorf("expected status %q, got %q", api.ResponseStatusCompleted, resp.Status)
	}

	// Verify model.
	if resp.Model != "test-model-v1" {
		t.Errorf("expected model %q, got %q", "test-model-v1", resp.Model)
	}

	// Verify output items.
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(resp.Output))
	}

	item := resp.Output[0]
	if !api.ValidateItemID(item.ID) {
		t.Errorf("expected valid item ID with item_ prefix, got %q", item.ID)
	}
	if item.Type != api.ItemTypeMessage {
		t.Errorf("expected item type %q, got %q", api.ItemTypeMessage, item.Type)
	}
	if item.Message == nil {
		t.Fatal("expected message data")
	}
	if item.Message.Role != api.RoleAssistant {
		t.Errorf("expected role %q, got %q", api.RoleAssistant, item.Message.Role)
	}
	if len(item.Message.Output) != 1 || item.Message.Output[0].Text != "Hello there!" {
		t.Errorf("expected output text %q, got %v", "Hello there!", item.Message.Output)
	}

	// Verify usage.
	if resp.Usage == nil {
		t.Fatal("expected usage")
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("expected input_tokens 10, got %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 5 {
		t.Errorf("expected output_tokens 5, got %d", resp.Usage.OutputTokens)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("expected total_tokens 15, got %d", resp.Usage.TotalTokens)
	}

	// Verify CreatedAt is populated.
	if resp.CreatedAt == 0 {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestEngine_CreateResponse_DefaultModel(t *testing.T) {
	mp := &mockProvider{
		name: "test",
		caps: provider.ProviderCapabilities{},
		response: &provider.ProviderResponse{
			Model:  "default-model",
			Status: api.ResponseStatusCompleted,
			Usage:  api.Usage{},
		},
	}

	eng, err := New(mp, nil, Config{DefaultModel: "default-model"})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		// Model intentionally omitted.
		Input: []api.Item{},
	}

	w := &mockResponseWriter{}
	if err := eng.CreateResponse(context.Background(), req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	if w.response.Model != "default-model" {
		t.Errorf("expected model %q, got %q", "default-model", w.response.Model)
	}
}

func TestEngine_CreateResponse_MissingModel(t *testing.T) {
	mp := &mockProvider{
		name: "test",
		caps: provider.ProviderCapabilities{},
	}

	eng, err := New(mp, nil, Config{})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Input: []api.Item{},
	}

	w := &mockResponseWriter{}
	err = eng.CreateResponse(context.Background(), req, w)
	if err == nil {
		t.Fatal("expected error for missing model")
	}

	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *api.APIError, got %T", err)
	}
	if apiErr.Type != api.ErrorTypeInvalidRequest {
		t.Errorf("expected error type %q, got %q", api.ErrorTypeInvalidRequest, apiErr.Type)
	}
	if apiErr.Param != "model" {
		t.Errorf("expected param %q, got %q", "model", apiErr.Param)
	}
}

func TestEngine_CreateResponse_ToolCallOutput(t *testing.T) {
	mp := &mockProvider{
		name: "test",
		caps: provider.ProviderCapabilities{ToolCalling: true},
		response: &provider.ProviderResponse{
			Model:  "m",
			Status: api.ResponseStatusCompleted,
			Items: []api.Item{
				{
					Type:   api.ItemTypeFunctionCall,
					Status: api.ItemStatusCompleted,
					FunctionCall: &api.FunctionCallData{
						Name:      "get_weather",
						CallID:    "call_1",
						Arguments: `{"city":"Berlin"}`,
					},
				},
			},
			Usage: api.Usage{InputTokens: 20, OutputTokens: 15, TotalTokens: 35},
		},
	}

	eng, err := New(mp, nil, Config{})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{
			{
				Type: api.ItemTypeMessage,
				Message: &api.MessageData{
					Role:    api.RoleUser,
					Content: []api.ContentPart{{Type: "input_text", Text: "What's the weather?"}},
				},
			},
		},
	}

	w := &mockResponseWriter{}
	if err := eng.CreateResponse(context.Background(), req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	if len(w.response.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(w.response.Output))
	}

	item := w.response.Output[0]
	if item.Type != api.ItemTypeFunctionCall {
		t.Errorf("expected type %q, got %q", api.ItemTypeFunctionCall, item.Type)
	}
	if item.FunctionCall.Name != "get_weather" {
		t.Errorf("expected function name %q, got %q", "get_weather", item.FunctionCall.Name)
	}
	if !api.ValidateItemID(item.ID) {
		t.Errorf("expected valid item ID, got %q", item.ID)
	}
}

func TestEngine_CreateResponse_ProviderError(t *testing.T) {
	mp := &mockProvider{
		name: "test",
		caps: provider.ProviderCapabilities{},
		err:  api.NewServerError("backend unavailable"),
	}

	eng, err := New(mp, nil, Config{})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model: "m",
		Input: []api.Item{},
	}

	w := &mockResponseWriter{}
	err = eng.CreateResponse(context.Background(), req, w)
	if err == nil {
		t.Fatal("expected error from provider")
	}

	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *api.APIError, got %T", err)
	}
	if apiErr.Type != api.ErrorTypeServerError {
		t.Errorf("expected error type %q, got %q", api.ErrorTypeServerError, apiErr.Type)
	}
}

func TestEngine_New_NilProvider(t *testing.T) {
	_, err := New(nil, nil, Config{})
	if err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestEngine_CreateResponse_StreamingDeferred(t *testing.T) {
	mp := &mockProvider{
		name: "test",
		caps: provider.ProviderCapabilities{Streaming: true},
	}

	eng, err := New(mp, nil, Config{})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model:  "m",
		Stream: true,
		Input:  []api.Item{},
	}

	w := &mockResponseWriter{}
	err = eng.CreateResponse(context.Background(), req, w)
	if err == nil {
		t.Fatal("expected error for streaming (not yet implemented)")
	}
}
