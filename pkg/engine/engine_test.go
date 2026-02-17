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
	streamFn func(ctx context.Context, req *provider.ProviderRequest) (<-chan provider.ProviderEvent, error)
}

func (m *mockProvider) Name() string                        { return m.name }
func (m *mockProvider) Capabilities() provider.ProviderCapabilities { return m.caps }
func (m *mockProvider) Complete(_ context.Context, _ *provider.ProviderRequest) (*provider.ProviderResponse, error) {
	return m.response, m.err
}
func (m *mockProvider) Stream(ctx context.Context, req *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
	if m.streamFn != nil {
		return m.streamFn(ctx, req)
	}
	// Default: return error (no stream function configured).
	return nil, api.NewServerError("streaming not configured in mock")
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

func TestEngine_CreateResponse_StreamingBasic(t *testing.T) {
	mp := &mockProvider{
		name: "test",
		caps: provider.ProviderCapabilities{Streaming: true},
		streamFn: func(_ context.Context, _ *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
			ch := make(chan provider.ProviderEvent, 16)
			go func() {
				defer close(ch)
				// Emit a text delta.
				ch <- provider.ProviderEvent{
					Type:  provider.ProviderEventTextDelta,
					Delta: "",
				}
				ch <- provider.ProviderEvent{
					Type:  provider.ProviderEventTextDelta,
					Delta: "Hello",
				}
				ch <- provider.ProviderEvent{
					Type:  provider.ProviderEventTextDone,
					Delta: "",
				}
				ch <- provider.ProviderEvent{
					Type: provider.ProviderEventDone,
					Item: &api.Item{Status: api.ItemStatusCompleted},
					Usage: &api.Usage{
						InputTokens:  5,
						OutputTokens: 1,
						TotalTokens:  6,
					},
				}
			}()
			return ch, nil
		},
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
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should have received streaming events.
	if len(w.events) == 0 {
		t.Fatal("expected streaming events, got none")
	}

	// First event should be response.created.
	if w.events[0].Type != api.EventResponseCreated {
		t.Errorf("first event type = %q, want %q", w.events[0].Type, api.EventResponseCreated)
	}

	// Last event should be response.completed.
	last := w.events[len(w.events)-1]
	if last.Type != api.EventResponseCompleted {
		t.Errorf("last event type = %q, want %q", last.Type, api.EventResponseCompleted)
	}

	// The completed response should have usage.
	if last.Response == nil || last.Response.Usage == nil {
		t.Fatal("expected response with usage in completed event")
	}
	if last.Response.Usage.InputTokens != 5 {
		t.Errorf("usage input_tokens = %d, want 5", last.Response.Usage.InputTokens)
	}
}

func TestEngine_CreateResponse_StreamingProviderError(t *testing.T) {
	mp := &mockProvider{
		name: "test",
		caps: provider.ProviderCapabilities{Streaming: true},
		streamFn: func(_ context.Context, _ *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
			return nil, api.NewServerError("backend unavailable")
		},
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
		t.Fatal("expected error from provider")
	}
}

// T025: Comprehensive engine streaming integration tests.

func TestEngine_Streaming_FullEventSequence(t *testing.T) {
	// Verify the exact event sequence for a multi-token streaming response.
	mp := &mockProvider{
		name: "test",
		caps: provider.ProviderCapabilities{Streaming: true},
		streamFn: func(_ context.Context, _ *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
			ch := make(chan provider.ProviderEvent, 16)
			go func() {
				defer close(ch)
				// Role-only first chunk.
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDelta, Delta: ""}
				// Text deltas.
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDelta, Delta: "Hello"}
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDelta, Delta: " "}
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDelta, Delta: "world"}
				// Text done.
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDone}
				// Stream done.
				ch <- provider.ProviderEvent{
					Type:  provider.ProviderEventDone,
					Item:  &api.Item{Status: api.ItemStatusCompleted},
					Usage: &api.Usage{InputTokens: 10, OutputTokens: 3, TotalTokens: 13},
				}
			}()
			return ch, nil
		},
	}

	eng, err := New(mp, nil, Config{})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	req := &api.CreateResponseRequest{
		Model:  "test-model",
		Stream: true,
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
	if err := eng.CreateResponse(context.Background(), req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	// Expected event sequence:
	// 0: response.created
	// 1: response.in_progress
	// 2: response.output_item.added
	// 3: response.content_part.added
	// 4: response.output_text.delta "Hello"
	// 5: response.output_text.delta " "
	// 6: response.output_text.delta "world"
	// 7: response.output_text.done
	// 8: response.content_part.done
	// 9: response.output_item.done
	// 10: response.completed

	expectedTypes := []api.StreamEventType{
		api.EventResponseCreated,
		api.EventResponseInProgress,
		api.EventOutputItemAdded,
		api.EventContentPartAdded,
		api.EventOutputTextDelta,
		api.EventOutputTextDelta,
		api.EventOutputTextDelta,
		api.EventOutputTextDone,
		api.EventContentPartDone,
		api.EventOutputItemDone,
		api.EventResponseCompleted,
	}

	if len(w.events) != len(expectedTypes) {
		t.Fatalf("expected %d events, got %d", len(expectedTypes), len(w.events))
		for i, ev := range w.events {
			t.Logf("  event[%d]: %s", i, ev.Type)
		}
	}

	for i, wantType := range expectedTypes {
		if w.events[i].Type != wantType {
			t.Errorf("event[%d].Type = %q, want %q", i, w.events[i].Type, wantType)
		}
	}

	// Verify text deltas contain expected content.
	if w.events[4].Delta != "Hello" {
		t.Errorf("delta[4] = %q, want %q", w.events[4].Delta, "Hello")
	}
	if w.events[5].Delta != " " {
		t.Errorf("delta[5] = %q, want %q", w.events[5].Delta, " ")
	}
	if w.events[6].Delta != "world" {
		t.Errorf("delta[6] = %q, want %q", w.events[6].Delta, "world")
	}

	// Verify response.created has a valid response with resp_ prefix.
	createdResp := w.events[0].Response
	if createdResp == nil {
		t.Fatal("expected response in created event")
	}
	if !api.ValidateResponseID(createdResp.ID) {
		t.Errorf("expected valid response ID, got %q", createdResp.ID)
	}
	if createdResp.Status != api.ResponseStatusInProgress {
		t.Errorf("created response status = %q, want %q", createdResp.Status, api.ResponseStatusInProgress)
	}

	// Verify output_item.added has a valid item.
	addedItem := w.events[2].Item
	if addedItem == nil {
		t.Fatal("expected item in output_item.added event")
	}
	if !api.ValidateItemID(addedItem.ID) {
		t.Errorf("expected valid item ID, got %q", addedItem.ID)
	}
	if addedItem.Type != api.ItemTypeMessage {
		t.Errorf("item type = %q, want %q", addedItem.Type, api.ItemTypeMessage)
	}

	// Verify content_part.done contains accumulated text.
	partDone := w.events[8]
	if partDone.Part == nil {
		t.Fatal("expected part in content_part.done")
	}
	if partDone.Part.Text != "Hello world" {
		t.Errorf("part.done text = %q, want %q", partDone.Part.Text, "Hello world")
	}

	// Verify output_item.done has completed item with full output.
	itemDone := w.events[9]
	if itemDone.Item == nil {
		t.Fatal("expected item in output_item.done")
	}
	if itemDone.Item.Status != api.ItemStatusCompleted {
		t.Errorf("item done status = %q, want %q", itemDone.Item.Status, api.ItemStatusCompleted)
	}
	if itemDone.Item.Message == nil || len(itemDone.Item.Message.Output) == 0 {
		t.Fatal("expected message with output in item.done")
	}
	if itemDone.Item.Message.Output[0].Text != "Hello world" {
		t.Errorf("item done text = %q, want %q", itemDone.Item.Message.Output[0].Text, "Hello world")
	}

	// Verify response.completed has usage and correct status.
	completed := w.events[10]
	if completed.Response == nil {
		t.Fatal("expected response in completed event")
	}
	if completed.Response.Status != api.ResponseStatusCompleted {
		t.Errorf("completed status = %q, want %q", completed.Response.Status, api.ResponseStatusCompleted)
	}
	if completed.Response.Usage == nil {
		t.Fatal("expected usage in completed response")
	}
	if completed.Response.Usage.InputTokens != 10 {
		t.Errorf("usage input_tokens = %d, want 10", completed.Response.Usage.InputTokens)
	}
	if completed.Response.Usage.OutputTokens != 3 {
		t.Errorf("usage output_tokens = %d, want 3", completed.Response.Usage.OutputTokens)
	}

	// Verify sequence numbers are monotonically increasing.
	for i := 1; i < len(w.events); i++ {
		if w.events[i].SequenceNumber <= w.events[i-1].SequenceNumber {
			t.Errorf("sequence numbers not monotonic: event[%d]=%d, event[%d]=%d",
				i-1, w.events[i-1].SequenceNumber, i, w.events[i].SequenceNumber)
		}
	}

	// Verify no WriteResponse calls were made (streaming only uses WriteEvent).
	if w.writeRespCalls != 0 {
		t.Errorf("expected 0 WriteResponse calls, got %d", w.writeRespCalls)
	}
}

func TestEngine_Streaming_FinishReasonLength(t *testing.T) {
	// Verify that finish_reason=length produces incomplete status.
	mp := &mockProvider{
		name: "test",
		caps: provider.ProviderCapabilities{Streaming: true},
		streamFn: func(_ context.Context, _ *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
			ch := make(chan provider.ProviderEvent, 16)
			go func() {
				defer close(ch)
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDelta, Delta: ""}
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDelta, Delta: "truncated output"}
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDone}
				ch <- provider.ProviderEvent{
					Type: provider.ProviderEventDone,
					Item: &api.Item{Status: api.ItemStatusIncomplete},
				}
			}()
			return ch, nil
		},
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
	if err := eng.CreateResponse(context.Background(), req, w); err != nil {
		t.Fatalf("CreateResponse failed: %v", err)
	}

	// Last event should be response.completed with incomplete status.
	last := w.events[len(w.events)-1]
	if last.Type != api.EventResponseCompleted {
		t.Errorf("last event type = %q, want %q", last.Type, api.EventResponseCompleted)
	}
	if last.Response.Status != api.ResponseStatusIncomplete {
		t.Errorf("response status = %q, want %q", last.Response.Status, api.ResponseStatusIncomplete)
	}

	// Find the output_item.done event and verify item status is incomplete.
	for _, ev := range w.events {
		if ev.Type == api.EventOutputItemDone && ev.Item != nil {
			if ev.Item.Status != api.ItemStatusIncomplete {
				t.Errorf("item status = %q, want %q", ev.Item.Status, api.ItemStatusIncomplete)
			}
			break
		}
	}
}

func TestEngine_Streaming_StreamError(t *testing.T) {
	// Verify that a ProviderEventError during streaming emits response.failed.
	mp := &mockProvider{
		name: "test",
		caps: provider.ProviderCapabilities{Streaming: true},
		streamFn: func(_ context.Context, _ *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
			ch := make(chan provider.ProviderEvent, 16)
			go func() {
				defer close(ch)
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDelta, Delta: ""}
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDelta, Delta: "partial"}
				// Error mid-stream.
				ch <- provider.ProviderEvent{
					Type: provider.ProviderEventError,
					Err:  api.NewServerError("backend connection lost"),
				}
			}()
			return ch, nil
		},
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
	// The engine writes the failed event and returns nil (error was communicated
	// via the event stream).
	if err != nil {
		t.Fatalf("expected nil error (error communicated via events), got %v", err)
	}

	// Find the response.failed event.
	var failedEvent *api.StreamEvent
	for i := range w.events {
		if w.events[i].Type == api.EventResponseFailed {
			failedEvent = &w.events[i]
			break
		}
	}

	if failedEvent == nil {
		t.Fatal("expected response.failed event")
	}
	if failedEvent.Response == nil {
		t.Fatal("expected response in failed event")
	}
	if failedEvent.Response.Status != api.ResponseStatusFailed {
		t.Errorf("response status = %q, want %q", failedEvent.Response.Status, api.ResponseStatusFailed)
	}
	if failedEvent.Response.Error == nil {
		t.Fatal("expected error in failed response")
	}
}

func TestEngine_Streaming_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	mp := &mockProvider{
		name: "test",
		caps: provider.ProviderCapabilities{Streaming: true},
		streamFn: func(_ context.Context, _ *provider.ProviderRequest) (<-chan provider.ProviderEvent, error) {
			ch := make(chan provider.ProviderEvent, 16)
			go func() {
				defer close(ch)
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDelta, Delta: ""}
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDelta, Delta: "start"}
				// Cancel the context after sending some events.
				cancel()
				// These events may or may not be consumed depending on timing.
				ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDelta, Delta: "after-cancel"}
			}()
			return ch, nil
		},
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
	err = eng.CreateResponse(ctx, req, w)
	// The engine should handle cancellation gracefully.
	if err != nil {
		t.Fatalf("expected nil error after cancellation, got %v", err)
	}

	// Should have a response.cancelled event.
	var cancelledFound bool
	for _, ev := range w.events {
		if ev.Type == api.EventResponseCancelled {
			cancelledFound = true
			if ev.Response == nil {
				t.Fatal("expected response in cancelled event")
			}
			if ev.Response.Status != api.ResponseStatusCancelled {
				t.Errorf("cancelled response status = %q, want %q", ev.Response.Status, api.ResponseStatusCancelled)
			}
		}
	}

	if !cancelledFound {
		t.Error("expected response.cancelled event, not found")
	}
}
