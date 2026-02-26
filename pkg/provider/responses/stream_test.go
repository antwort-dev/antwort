package responses

import (
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/provider"
)

func TestParseSSEStream_TextDelta(t *testing.T) {
	stream := `event: response.created
data: {}

event: response.output_text.delta
data: {"delta":"Hello"}

event: response.output_text.delta
data: {"delta":" world"}

event: response.output_text.done
data: {}

event: response.completed
data: {"response":{"id":"resp_1","status":"completed","model":"m","output":[],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}

`

	ch := make(chan provider.ProviderEvent, 32)
	go parseSSEStream(strings.NewReader(stream), ch)

	var events []provider.ProviderEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Expect: text delta "Hello", text delta " world", text done, done (with usage)
	if len(events) < 4 {
		t.Fatalf("expected at least 4 events, got %d", len(events))
	}

	if events[0].Type != provider.ProviderEventTextDelta || events[0].Delta != "Hello" {
		t.Errorf("event 0: type=%v delta=%q, want TextDelta 'Hello'", events[0].Type, events[0].Delta)
	}
	if events[1].Type != provider.ProviderEventTextDelta || events[1].Delta != " world" {
		t.Errorf("event 1: type=%v delta=%q, want TextDelta ' world'", events[1].Type, events[1].Delta)
	}
	if events[2].Type != provider.ProviderEventTextDone {
		t.Errorf("event 2: type=%v, want TextDone", events[2].Type)
	}
	if events[3].Type != provider.ProviderEventDone {
		t.Errorf("event 3: type=%v, want Done", events[3].Type)
	}
	if events[3].Usage == nil {
		t.Error("expected usage on Done event")
	} else if events[3].Usage.InputTokens != 10 {
		t.Errorf("usage input_tokens = %d, want 10", events[3].Usage.InputTokens)
	}
}

func TestParseSSEStream_FunctionCall(t *testing.T) {
	stream := `event: response.function_call_arguments.delta
data: {"delta":"{\"city\"","call_id":"call_1","name":"get_weather","output_index":0}

event: response.function_call_arguments.delta
data: {"delta":":\"Berlin\"}","call_id":"call_1","name":"get_weather","output_index":0}

event: response.function_call_arguments.done
data: {"arguments":"{\"city\":\"Berlin\"}","call_id":"call_1","name":"get_weather","output_index":0}

event: response.completed
data: {"response":{"id":"resp_2","status":"completed","model":"m","output":[]}}

`

	ch := make(chan provider.ProviderEvent, 32)
	go parseSSEStream(strings.NewReader(stream), ch)

	var events []provider.ProviderEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}

	// First two: tool call deltas
	if events[0].Type != provider.ProviderEventToolCallDelta {
		t.Errorf("event 0: type=%v, want ToolCallDelta", events[0].Type)
	}
	if events[0].ToolCallID != "call_1" {
		t.Errorf("event 0: call_id=%q, want 'call_1'", events[0].ToolCallID)
	}
	if events[0].FunctionName != "get_weather" {
		t.Errorf("event 0: function=%q, want 'get_weather'", events[0].FunctionName)
	}

	// Third: tool call done
	if events[2].Type != provider.ProviderEventToolCallDone {
		t.Errorf("event 2: type=%v, want ToolCallDone", events[2].Type)
	}
}

func TestParseSSEStream_UnknownEventSkipped(t *testing.T) {
	stream := `event: response.some_future_event
data: {"foo":"bar"}

event: response.output_text.delta
data: {"delta":"hi"}

event: response.completed
data: {"response":{"id":"resp_3","status":"completed","model":"m","output":[]}}

`

	ch := make(chan provider.ProviderEvent, 32)
	go parseSSEStream(strings.NewReader(stream), ch)

	var events []provider.ProviderEvent
	for ev := range ch {
		events = append(events, ev)
	}

	// Unknown event should be skipped, leaving: text delta, done
	if len(events) != 2 {
		t.Fatalf("expected 2 events (unknown skipped), got %d", len(events))
	}
	if events[0].Type != provider.ProviderEventTextDelta {
		t.Errorf("event 0: type=%v, want TextDelta", events[0].Type)
	}
}

func TestParseSSEStream_ErrorEvent(t *testing.T) {
	stream := `event: response.failed
data: {}

`

	ch := make(chan provider.ProviderEvent, 32)
	go parseSSEStream(strings.NewReader(stream), ch)

	var events []provider.ProviderEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != provider.ProviderEventError {
		t.Errorf("event type = %v, want Error", events[0].Type)
	}
}

func TestParseSSEStream_DoneSignal(t *testing.T) {
	stream := `data: [DONE]
`

	ch := make(chan provider.ProviderEvent, 32)
	go parseSSEStream(strings.NewReader(stream), ch)

	var events []provider.ProviderEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 1 || events[0].Type != provider.ProviderEventDone {
		t.Errorf("expected single Done event, got %v", events)
	}
}
