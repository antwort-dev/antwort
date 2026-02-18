package vllm

import (
	"context"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

// collectEvents runs parseSSEStream and returns all events.
func collectEvents(t *testing.T, sseData string) []provider.ProviderEvent {
	t.Helper()
	ch := make(chan provider.ProviderEvent, 64)
	ctx := context.Background()

	go func() {
		defer close(ch)
		parseSSEStream(ctx, strings.NewReader(sseData), ch)
	}()

	var events []provider.ProviderEvent
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func TestParseSSEStream_TextDeltas(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	events := collectEvents(t, sseData)

	// Expected sequence: role delta (empty), "Hello" delta, " world" delta,
	// text done, stream done.
	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d: %+v", len(events), events)
	}

	// First event: role-only chunk produces empty text delta.
	assertEventType(t, events[0], provider.ProviderEventTextDelta, "")

	// Text deltas.
	assertEventType(t, events[1], provider.ProviderEventTextDelta, "Hello")
	assertEventType(t, events[2], provider.ProviderEventTextDelta, " world")

	// Text done (from finish_reason).
	assertEventType(t, events[3], provider.ProviderEventTextDone, "")

	// Stream done.
	assertEventType(t, events[4], provider.ProviderEventDone, "")
}

func TestParseSSEStream_DoneSentinel(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	events := collectEvents(t, sseData)

	// Last event should be done (from finish_reason).
	lastIdx := len(events) - 1
	if lastIdx < 0 {
		t.Fatal("no events received")
	}
	if events[lastIdx].Type != provider.ProviderEventDone {
		t.Errorf("last event type = %d, want ProviderEventDone", events[lastIdx].Type)
	}
}

func TestParseSSEStream_MalformedChunk(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":null}]}

data: {this is not valid json}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	events := collectEvents(t, sseData)

	// Should have: "Hi" delta, "!" delta (malformed skipped), text done, done.
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events (malformed should be skipped), got %d", len(events))
	}

	// Verify text deltas are present (malformed was skipped).
	var textDeltas []string
	for _, ev := range events {
		if ev.Type == provider.ProviderEventTextDelta && ev.Delta != "" {
			textDeltas = append(textDeltas, ev.Delta)
		}
	}
	if len(textDeltas) != 2 {
		t.Errorf("expected 2 text deltas, got %d: %v", len(textDeltas), textDeltas)
	}
}

func TestParseSSEStream_UsageInFinalChunk(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}

data: [DONE]
`
	events := collectEvents(t, sseData)

	// Find the done event and check usage.
	var doneEvent *provider.ProviderEvent
	for i := range events {
		if events[i].Type == provider.ProviderEventDone && events[i].Usage != nil {
			doneEvent = &events[i]
			break
		}
	}

	if doneEvent == nil {
		t.Fatal("no done event with usage found")
	}

	if doneEvent.Usage.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", doneEvent.Usage.InputTokens)
	}
	if doneEvent.Usage.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", doneEvent.Usage.OutputTokens)
	}
	if doneEvent.Usage.TotalTokens != 15 {
		t.Errorf("TotalTokens = %d, want 15", doneEvent.Usage.TotalTokens)
	}
}

func TestParseSSEStream_UsageOnlyChunk(t *testing.T) {
	// Some backends send a separate usage-only chunk after finish_reason
	// when stream_options.include_usage is true.
	sseData := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[],"usage":{"prompt_tokens":8,"completion_tokens":2,"total_tokens":10}}

data: [DONE]
`
	events := collectEvents(t, sseData)

	// Find a done event with usage from the usage-only chunk.
	var usageEvent *provider.ProviderEvent
	for i := range events {
		if events[i].Type == provider.ProviderEventDone && events[i].Usage != nil {
			usageEvent = &events[i]
		}
	}

	if usageEvent == nil {
		t.Fatal("no done event with usage found")
	}

	if usageEvent.Usage.InputTokens != 8 {
		t.Errorf("InputTokens = %d, want 8", usageEvent.Usage.InputTokens)
	}
}

func TestParseSSEStream_FinishReasonLength(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"truncated"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"length"}]}

data: [DONE]
`
	events := collectEvents(t, sseData)

	// The done event should carry an item with incomplete status.
	var doneEvent *provider.ProviderEvent
	for i := range events {
		if events[i].Type == provider.ProviderEventDone {
			doneEvent = &events[i]
			break
		}
	}

	if doneEvent == nil {
		t.Fatal("no done event found")
	}
	if doneEvent.Item == nil {
		t.Fatal("done event has no item")
	}
	if doneEvent.Item.Status != api.ItemStatusIncomplete {
		t.Errorf("item status = %q, want %q", doneEvent.Item.Status, api.ItemStatusIncomplete)
	}
}

func TestParseSSEStream_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan provider.ProviderEvent, 64)

	// Create SSE data with many chunks.
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString(`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"x"},"finish_reason":null}]}`)
		sb.WriteString("\n\n")
	}
	sb.WriteString("data: [DONE]\n")

	// Cancel immediately.
	cancel()

	go func() {
		defer close(ch)
		parseSSEStream(ctx, strings.NewReader(sb.String()), ch)
	}()

	var count int
	for range ch {
		count++
	}

	// Should have very few events (cancelled before reading all).
	if count >= 100 {
		t.Errorf("expected fewer than 100 events after cancellation, got %d", count)
	}
}

func TestParseSSEStream_EmptyStream(t *testing.T) {
	sseData := `data: [DONE]
`
	events := collectEvents(t, sseData)

	// No events expected for an empty stream that immediately sends DONE.
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d: %+v", len(events), events)
	}
}

func TestParseSSEStream_SSECommentsIgnored(t *testing.T) {
	// SSE spec allows comments starting with ":" and empty lines.
	sseData := `: this is a comment
: keep-alive

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	events := collectEvents(t, sseData)

	// Comments should be ignored; we should get: text delta, text done, done.
	var textDeltas int
	for _, ev := range events {
		if ev.Type == provider.ProviderEventTextDelta && ev.Delta != "" {
			textDeltas++
		}
	}
	if textDeltas != 1 {
		t.Errorf("expected 1 text delta, got %d", textDeltas)
	}
}

func TestParseSSEStream_SingleToolCall(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"Berlin\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	events := collectEvents(t, sseData)

	// Should have: tool call deltas, tool call done, text done, stream done.
	var toolCallDeltas []provider.ProviderEvent
	var toolCallDones []provider.ProviderEvent
	for _, ev := range events {
		if ev.Type == provider.ProviderEventToolCallDelta {
			toolCallDeltas = append(toolCallDeltas, ev)
		}
		if ev.Type == provider.ProviderEventToolCallDone {
			toolCallDones = append(toolCallDones, ev)
		}
	}

	if len(toolCallDeltas) != 3 {
		t.Fatalf("expected 3 tool call deltas, got %d", len(toolCallDeltas))
	}

	// First delta should have function name.
	if toolCallDeltas[0].FunctionName != "get_weather" {
		t.Errorf("first delta function name = %q, want %q", toolCallDeltas[0].FunctionName, "get_weather")
	}
	if toolCallDeltas[0].ToolCallID != "call_1" {
		t.Errorf("first delta tool call ID = %q, want %q", toolCallDeltas[0].ToolCallID, "call_1")
	}

	// Should have exactly 1 tool call done.
	if len(toolCallDones) != 1 {
		t.Fatalf("expected 1 tool call done, got %d", len(toolCallDones))
	}

	// Tool call done should have complete assembled arguments.
	done := toolCallDones[0]
	if done.Item == nil {
		t.Fatal("tool call done has no item")
	}
	if done.Item.FunctionCall == nil {
		t.Fatal("tool call done item has no function call data")
	}
	if done.Item.FunctionCall.Name != "get_weather" {
		t.Errorf("function name = %q, want %q", done.Item.FunctionCall.Name, "get_weather")
	}
	if done.Item.FunctionCall.CallID != "call_1" {
		t.Errorf("call ID = %q, want %q", done.Item.FunctionCall.CallID, "call_1")
	}
	expectedArgs := `{"city":"Berlin"}`
	if done.Item.FunctionCall.Arguments != expectedArgs {
		t.Errorf("arguments = %q, want %q", done.Item.FunctionCall.Arguments, expectedArgs)
	}
}

func TestParseSSEStream_MultipleToolCalls(t *testing.T) {
	// Two tool calls in the same response (parallel tool calls).
	sseData := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Berlin\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_2","type":"function","function":{"name":"get_time","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"{\"tz\":\"CET\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	events := collectEvents(t, sseData)

	// Should have 2 tool call done events.
	var toolCallDones []provider.ProviderEvent
	for _, ev := range events {
		if ev.Type == provider.ProviderEventToolCallDone {
			toolCallDones = append(toolCallDones, ev)
		}
	}

	if len(toolCallDones) != 2 {
		t.Fatalf("expected 2 tool call dones, got %d", len(toolCallDones))
	}

	// Verify both tool calls have correct data (order may vary due to map iteration).
	names := map[string]string{}
	for _, done := range toolCallDones {
		if done.Item != nil && done.Item.FunctionCall != nil {
			names[done.Item.FunctionCall.Name] = done.Item.FunctionCall.Arguments
		}
	}

	if args, ok := names["get_weather"]; !ok {
		t.Error("missing get_weather tool call")
	} else if args != `{"city":"Berlin"}` {
		t.Errorf("get_weather args = %q, want %q", args, `{"city":"Berlin"}`)
	}

	if args, ok := names["get_time"]; !ok {
		t.Error("missing get_time tool call")
	} else if args != `{"tz":"CET"}` {
		t.Errorf("get_time args = %q, want %q", args, `{"tz":"CET"}`)
	}
}

func TestParseSSEStream_ToolCallIncrementalArgs(t *testing.T) {
	// Arguments split across 5 chunks.
	sseData := `data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_x","type":"function","function":{"name":"search","arguments":""}}]},"finish_reason":null}]}

data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"q"}}]},"finish_reason":null}]}

data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"uery"}}]},"finish_reason":null}]}

data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\":\""}}]},"finish_reason":null}]}

data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"hello"}}]},"finish_reason":null}]}

data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"}"}}]},"finish_reason":null}]}

data: {"id":"1","object":"chat.completion.chunk","model":"m","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	events := collectEvents(t, sseData)

	// Find the tool call done event.
	var done *provider.ProviderEvent
	for i := range events {
		if events[i].Type == provider.ProviderEventToolCallDone {
			done = &events[i]
			break
		}
	}

	if done == nil {
		t.Fatal("no tool call done event found")
	}

	expectedArgs := `{"query":"hello"}`
	if done.Item.FunctionCall.Arguments != expectedArgs {
		t.Errorf("assembled arguments = %q, want %q", done.Item.FunctionCall.Arguments, expectedArgs)
	}
}

// assertEventType checks that an event has the expected type and delta.
func assertEventType(t *testing.T, ev provider.ProviderEvent, wantType provider.ProviderEventType, wantDelta string) {
	t.Helper()
	if ev.Type != wantType {
		t.Errorf("event type = %d, want %d", ev.Type, wantType)
	}
	if ev.Delta != wantDelta {
		t.Errorf("event delta = %q, want %q", ev.Delta, wantDelta)
	}
}
