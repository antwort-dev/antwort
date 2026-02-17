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

func TestParseSSEStream_ToolCallWarning(t *testing.T) {
	// Tool call chunks should be logged as warning and skipped in Phase 4.
	sseData := `data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]
`
	events := collectEvents(t, sseData)

	// Tool call deltas should be skipped; we should only get
	// text done + done from the finish_reason chunk.
	for _, ev := range events {
		if ev.Type == provider.ProviderEventToolCallDelta || ev.Type == provider.ProviderEventToolCallDone {
			t.Errorf("unexpected tool call event in Phase 4: type=%d", ev.Type)
		}
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
