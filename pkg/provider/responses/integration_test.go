package responses

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/provider"
)

// mockResponsesServer creates an httptest server that speaks the Responses API
// protocol. The handler function receives the decoded request and returns the
// response body to send back.
func mockResponsesServer(t *testing.T, handler func(req responsesRequest) (int, any)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle probe requests (startup validation).
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		var req responsesRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "decode: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Probe detection: model "_probe" is the startup validation.
		if req.Model == "_probe" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "probe"})
			return
		}

		status, resp := handler(req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(resp)
	}))
}

// mockStreamingServer creates a server that returns SSE events for streaming requests.
func mockStreamingServer(t *testing.T, handler func(req responsesRequest) string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		var req responsesRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "decode: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Probe detection.
		if req.Model == "_probe" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "probe"})
			return
		}

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			sseData := handler(req)
			fmt.Fprint(w, sseData)
			return
		}

		// Non-streaming fallback.
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(responsesResponse{
			ID:     "resp_test",
			Status: "completed",
			Model:  req.Model,
		})
	}))
}

// --- T015: Startup validation tests ---

func TestNew_ProbeSuccess(t *testing.T) {
	// Backend returns 400 for the probe (endpoint exists but rejects the request).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid model"}`))
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New() should succeed when endpoint returns 400, got: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_ProbeModelNotFound(t *testing.T) {
	// Backend returns 404 with a JSON API error (model not found).
	// This means the endpoint exists but the probe model doesn't.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"object":"error","message":"The model '_probe' does not exist.","type":"NotFoundError","code":404}`))
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New() should succeed when 404 is a model-not-found API error, got: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNew_ProbeNotFound(t *testing.T) {
	// Backend returns 404 with no body (endpoint does not exist).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := New(Config{BaseURL: srv.URL})
	if err == nil {
		t.Fatal("expected error when backend returns 404")
	}
	if !strings.Contains(err.Error(), "does not support the Responses API") {
		t.Errorf("error message should mention Responses API support, got: %v", err)
	}
}

func TestNew_ProbeUnreachable(t *testing.T) {
	// Backend is not reachable at all.
	_, err := New(Config{BaseURL: "http://127.0.0.1:1"})
	if err == nil {
		t.Fatal("expected error when backend is unreachable")
	}
	if !strings.Contains(err.Error(), "not reachable") {
		t.Errorf("error message should mention unreachable, got: %v", err)
	}
}

// --- T011/T012: Conversation history reconstruction (stateful enrichment) ---

func TestIntegration_ConversationHistory(t *testing.T) {
	// Verify that when the provider receives messages reconstructed from
	// conversation history, they are correctly translated to Responses API
	// input items.
	var capturedReq responsesRequest

	srv := mockResponsesServer(t, func(req responsesRequest) (int, any) {
		capturedReq = req
		return http.StatusOK, responsesResponse{
			ID:     "resp_002",
			Status: "completed",
			Model:  req.Model,
			Output: []responsesItem{
				{
					ID:   "item_001",
					Type: "message",
					Role: "assistant",
					Content: json.RawMessage(`[{"type":"output_text","text":"The answer is 4"}]`),
				},
			},
			Usage: &responsesUsage{InputTokens: 30, OutputTokens: 5, TotalTokens: 35},
		}
	})
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Simulate conversation history: the engine would have reconstructed
	// these messages from a previous_response_id chain.
	req := &provider.ProviderRequest{
		Model: "test-model",
		Messages: []provider.ProviderMessage{
			// History messages (from previous turn).
			{Role: "system", Content: "You are a calculator"},
			{Role: "user", Content: "What is 2+2?"},
			{
				Role: "assistant",
				ToolCalls: []provider.ProviderToolCall{
					{
						ID:   "call_calc",
						Type: "function",
						Function: provider.ProviderFunctionCall{
							Name:      "calculator",
							Arguments: `{"expr":"2+2"}`,
						},
					},
				},
			},
			{Role: "tool", Content: "4", ToolCallID: "call_calc"},
			// Current turn message.
			{Role: "user", Content: "Explain the answer"},
		},
	}

	resp, err := p.Complete(t.Context(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Verify the response was translated correctly.
	if resp.Model != "test-model" {
		t.Errorf("model = %q, want %q", resp.Model, "test-model")
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}

	// Verify the backend received the full conversation history as input.
	var inputItems []map[string]any
	if err := json.Unmarshal(capturedReq.Input, &inputItems); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}

	// Expected items: system, user, function_call, function_call_output, user
	if len(inputItems) != 5 {
		t.Fatalf("expected 5 input items, got %d", len(inputItems))
	}

	// Verify item types.
	expectedTypes := []string{"message", "message", "function_call", "function_call_output", "message"}
	for i, wantType := range expectedTypes {
		gotType, _ := inputItems[i]["type"].(string)
		if gotType != wantType {
			t.Errorf("input[%d].type = %q, want %q", i, gotType, wantType)
		}
	}

	// Verify the function_call item has the correct call_id and name.
	fcItem := inputItems[2]
	if callID, _ := fcItem["call_id"].(string); callID != "call_calc" {
		t.Errorf("function_call.call_id = %q, want %q", callID, "call_calc")
	}
	if name, _ := fcItem["name"].(string); name != "calculator" {
		t.Errorf("function_call.name = %q, want %q", name, "calculator")
	}

	// Verify the function_call_output has the correct call_id and output.
	fcoItem := inputItems[3]
	if callID, _ := fcoItem["call_id"].(string); callID != "call_calc" {
		t.Errorf("function_call_output.call_id = %q, want %q", callID, "call_calc")
	}
	if output, _ := fcoItem["output"].(string); output != "4" {
		t.Errorf("function_call_output.output = %q, want %q", output, "4")
	}

	// Verify store is always false.
	if capturedReq.Store != false {
		t.Error("store should always be false")
	}
}

func TestIntegration_ConversationHistory_Streaming(t *testing.T) {
	// Verify that conversation history works correctly with streaming.
	var capturedInputLen int

	srv := mockStreamingServer(t, func(req responsesRequest) string {
		var items []json.RawMessage
		json.Unmarshal(req.Input, &items)
		capturedInputLen = len(items)

		return `event: response.output_text.delta
data: {"delta":"Response"}

event: response.completed
data: {"response":{"id":"resp_s","status":"completed","model":"m","output":[],"usage":{"input_tokens":20,"output_tokens":1,"total_tokens":21}}}

`
	})
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Simulate history with 2 messages + 1 current.
	req := &provider.ProviderRequest{
		Model:  "m",
		Stream: true,
		Messages: []provider.ProviderMessage{
			{Role: "user", Content: "First message"},
			{Role: "assistant", Content: "First reply"},
			{Role: "user", Content: "Second message"},
		},
	}

	ch, err := p.Stream(t.Context(), req)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var events []provider.ProviderEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Verify backend received all 3 messages as input.
	if capturedInputLen != 3 {
		t.Errorf("expected 3 input items sent to backend, got %d", capturedInputLen)
	}

	// Verify we got text delta and done events.
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
	if events[0].Type != provider.ProviderEventTextDelta {
		t.Errorf("event[0].type = %v, want TextDelta", events[0].Type)
	}
}

// --- T013/T014: Tool call round-trip tests ---

func TestIntegration_ToolCallResponse(t *testing.T) {
	// Verify the provider correctly parses function_call items from the
	// backend response and maps them to ProviderResponse.Items.
	srv := mockResponsesServer(t, func(req responsesRequest) (int, any) {
		return http.StatusOK, responsesResponse{
			ID:     "resp_tc",
			Status: "completed",
			Model:  req.Model,
			Output: []responsesItem{
				{
					ID:        "item_fc1",
					Type:      "function_call",
					CallID:    "call_weather",
					Name:      "get_weather",
					Arguments: `{"city":"Berlin","units":"celsius"}`,
					Status:    "completed",
				},
			},
			Usage: &responsesUsage{InputTokens: 15, OutputTokens: 10, TotalTokens: 25},
		}
	})
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := &provider.ProviderRequest{
		Model: "tool-model",
		Messages: []provider.ProviderMessage{
			{Role: "user", Content: "What's the weather in Berlin?"},
		},
		Tools: []provider.ProviderTool{
			{
				Type: "function",
				Function: provider.ProviderFunctionDef{
					Name:        "get_weather",
					Description: "Get weather for a city",
					Parameters:  json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
				},
			},
		},
	}

	resp, err := p.Complete(t.Context(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Verify the function_call item was parsed correctly.
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}

	item := resp.Items[0]
	if item.Type != "function_call" {
		t.Errorf("item.type = %q, want %q", item.Type, "function_call")
	}
	if item.FunctionCall == nil {
		t.Fatal("function_call data is nil")
	}
	if item.FunctionCall.Name != "get_weather" {
		t.Errorf("name = %q, want %q", item.FunctionCall.Name, "get_weather")
	}
	if item.FunctionCall.CallID != "call_weather" {
		t.Errorf("call_id = %q, want %q", item.FunctionCall.CallID, "call_weather")
	}
	if item.FunctionCall.Arguments != `{"city":"Berlin","units":"celsius"}` {
		t.Errorf("arguments = %q", item.FunctionCall.Arguments)
	}
}

func TestIntegration_ToolCallRoundTrip(t *testing.T) {
	// Simulate a multi-turn tool call round trip:
	// Turn 1: User asks question, backend returns tool call
	// Turn 2: Tool result is sent back, backend returns final answer
	callCount := 0

	srv := mockResponsesServer(t, func(req responsesRequest) (int, any) {
		callCount++

		var inputItems []map[string]any
		json.Unmarshal(req.Input, &inputItems)

		if callCount == 1 {
			// First call: return a tool call.
			return http.StatusOK, responsesResponse{
				ID:     "resp_turn1",
				Status: "completed",
				Model:  req.Model,
				Output: []responsesItem{
					{
						ID:        "fc_1",
						Type:      "function_call",
						CallID:    "call_123",
						Name:      "code_interpreter",
						Arguments: `{"code":"print(2+2)"}`,
						Status:    "completed",
					},
				},
				Usage: &responsesUsage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
			}
		}

		// Second call: tool result should be in input, return final answer.
		// Verify the tool result is present.
		hasToolOutput := false
		for _, item := range inputItems {
			if itemType, _ := item["type"].(string); itemType == "function_call_output" {
				hasToolOutput = true
				if callID, _ := item["call_id"].(string); callID != "call_123" {
					t.Errorf("tool output call_id = %q, want %q", callID, "call_123")
				}
			}
		}
		if !hasToolOutput {
			t.Error("second call should include function_call_output in input")
		}

		return http.StatusOK, responsesResponse{
			ID:     "resp_turn2",
			Status: "completed",
			Model:  req.Model,
			Output: []responsesItem{
				{
					ID:   "msg_1",
					Type: "message",
					Role: "assistant",
					Content: json.RawMessage(`[{"type":"output_text","text":"The result is 4"}]`),
				},
			},
			Usage: &responsesUsage{InputTokens: 25, OutputTokens: 5, TotalTokens: 30},
		}
	})
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Turn 1: Send initial request.
	req1 := &provider.ProviderRequest{
		Model: "m",
		Messages: []provider.ProviderMessage{
			{Role: "user", Content: "What is 2+2?"},
		},
		Tools: []provider.ProviderTool{
			{
				Type: "function",
				Function: provider.ProviderFunctionDef{
					Name:        "code_interpreter",
					Description: "Run code",
					Parameters:  json.RawMessage(`{"type":"object"}`),
				},
			},
		},
	}

	resp1, err := p.Complete(t.Context(), req1)
	if err != nil {
		t.Fatalf("Turn 1 Complete: %v", err)
	}

	// Verify we got a tool call.
	if len(resp1.Items) != 1 || resp1.Items[0].Type != "function_call" {
		t.Fatalf("Turn 1: expected function_call item, got %v", resp1.Items)
	}

	fc := resp1.Items[0].FunctionCall
	if fc.Name != "code_interpreter" {
		t.Errorf("Turn 1: function name = %q, want %q", fc.Name, "code_interpreter")
	}

	// Turn 2: Send tool result back (simulating what the engine does).
	req2 := &provider.ProviderRequest{
		Model: "m",
		Messages: []provider.ProviderMessage{
			{Role: "user", Content: "What is 2+2?"},
			{
				Role: "assistant",
				ToolCalls: []provider.ProviderToolCall{
					{
						ID:   fc.CallID,
						Type: "function",
						Function: provider.ProviderFunctionCall{
							Name:      fc.Name,
							Arguments: fc.Arguments,
						},
					},
				},
			},
			{Role: "tool", Content: "4", ToolCallID: fc.CallID},
		},
		Tools: req1.Tools,
	}

	resp2, err := p.Complete(t.Context(), req2)
	if err != nil {
		t.Fatalf("Turn 2 Complete: %v", err)
	}

	// Verify we got a message response.
	if len(resp2.Items) != 1 || resp2.Items[0].Type != "message" {
		t.Fatalf("Turn 2: expected message item, got %v", resp2.Items)
	}
	if resp2.Items[0].Message == nil || len(resp2.Items[0].Message.Output) == 0 {
		t.Fatal("Turn 2: expected message with output")
	}
	if resp2.Items[0].Message.Output[0].Text != "The result is 4" {
		t.Errorf("Turn 2: text = %q, want %q", resp2.Items[0].Message.Output[0].Text, "The result is 4")
	}

	if callCount != 2 {
		t.Errorf("expected 2 backend calls, got %d", callCount)
	}
}

func TestIntegration_ToolCall_Streaming(t *testing.T) {
	// Verify tool call events are correctly parsed from a streaming response.
	srv := mockStreamingServer(t, func(req responsesRequest) string {
		return `event: response.function_call_arguments.delta
data: {"delta":"{\"code\"","call_id":"call_ci","name":"code_interpreter","output_index":0}

event: response.function_call_arguments.delta
data: {"delta":":\"print(42)\"}","call_id":"call_ci","name":"code_interpreter","output_index":0}

event: response.function_call_arguments.done
data: {"arguments":"{\"code\":\"print(42)\"}","call_id":"call_ci","name":"code_interpreter","output_index":0}

event: response.completed
data: {"response":{"id":"resp_stream_tc","status":"completed","model":"m","output":[],"usage":{"input_tokens":10,"output_tokens":8,"total_tokens":18}}}

`
	})
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := &provider.ProviderRequest{
		Model:  "m",
		Stream: true,
		Messages: []provider.ProviderMessage{
			{Role: "user", Content: "Run some code"},
		},
	}

	ch, err := p.Stream(t.Context(), req)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var events []provider.ProviderEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Expected: 2 tool call deltas, 1 tool call done, 1 done
	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(events))
	}

	// Verify tool call delta events.
	if events[0].Type != provider.ProviderEventToolCallDelta {
		t.Errorf("event[0].type = %v, want ToolCallDelta", events[0].Type)
	}
	if events[0].ToolCallID != "call_ci" {
		t.Errorf("event[0].call_id = %q, want %q", events[0].ToolCallID, "call_ci")
	}
	if events[0].FunctionName != "code_interpreter" {
		t.Errorf("event[0].function = %q, want %q", events[0].FunctionName, "code_interpreter")
	}

	// Verify tool call done.
	if events[2].Type != provider.ProviderEventToolCallDone {
		t.Errorf("event[2].type = %v, want ToolCallDone", events[2].Type)
	}

	// Verify done with usage.
	lastEvent := events[len(events)-1]
	if lastEvent.Type != provider.ProviderEventDone {
		t.Errorf("last event type = %v, want Done", lastEvent.Type)
	}
	if lastEvent.Usage == nil || lastEvent.Usage.InputTokens != 10 {
		t.Errorf("done event usage = %+v, want input_tokens=10", lastEvent.Usage)
	}
}

// --- T015: API key forwarding ---

func TestIntegration_APIKeyForwarded(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusBadRequest) // probe accepted
		w.Write([]byte(`{"error":"test"}`))
	}))
	defer srv.Close()

	_, err := New(Config{BaseURL: srv.URL, APIKey: "test-key-123"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if capturedAuth != "Bearer test-key-123" {
		t.Errorf("auth = %q, want %q", capturedAuth, "Bearer test-key-123")
	}
}
