package engine

import (
	"testing"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

func TestMapTextDelta_Content(t *testing.T) {
	state := &streamState{
		itemID:      "item_1",
		outputIndex: 0,
		textStarted: true,
	}

	ev := provider.ProviderEvent{
		Type:  provider.ProviderEventTextDelta,
		Delta: "Hello",
	}

	events := mapProviderEvent(ev, state)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.Type != api.EventOutputTextDelta {
		t.Errorf("type = %q, want %q", got.Type, api.EventOutputTextDelta)
	}
	if got.Delta != "Hello" {
		t.Errorf("delta = %q, want %q", got.Delta, "Hello")
	}
	if got.ItemID != "item_1" {
		t.Errorf("item_id = %q, want %q", got.ItemID, "item_1")
	}
	if got.OutputIndex != 0 {
		t.Errorf("output_index = %d, want 0", got.OutputIndex)
	}
	if got.ContentIndex != 0 {
		t.Errorf("content_index = %d, want 0", got.ContentIndex)
	}
}

func TestMapTextDelta_EmptyDelta_FirstChunk(t *testing.T) {
	state := &streamState{
		itemID:      "item_1",
		outputIndex: 0,
		textStarted: false,
	}

	ev := provider.ProviderEvent{
		Type:  provider.ProviderEventTextDelta,
		Delta: "",
	}

	events := mapProviderEvent(ev, state)
	if len(events) != 0 {
		t.Fatalf("expected 0 events for role-only first chunk, got %d", len(events))
	}

	// But textStarted should be set.
	if !state.textStarted {
		t.Error("expected textStarted to be true after role-only chunk")
	}
}

func TestMapTextDelta_EmptyDelta_AfterText(t *testing.T) {
	state := &streamState{
		itemID:      "item_1",
		outputIndex: 0,
		textStarted: true,
	}

	ev := provider.ProviderEvent{
		Type:  provider.ProviderEventTextDelta,
		Delta: "",
	}

	events := mapProviderEvent(ev, state)
	if len(events) != 0 {
		t.Fatalf("expected 0 events for empty delta after text started, got %d", len(events))
	}
}

func TestMapTextDone(t *testing.T) {
	state := &streamState{
		itemID:      "item_1",
		outputIndex: 0,
		textStarted: true,
	}

	ev := provider.ProviderEvent{
		Type:  provider.ProviderEventTextDone,
		Delta: "full text",
	}

	events := mapProviderEvent(ev, state)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.Type != api.EventOutputTextDone {
		t.Errorf("type = %q, want %q", got.Type, api.EventOutputTextDone)
	}
	if got.Delta != "full text" {
		t.Errorf("delta = %q, want %q", got.Delta, "full text")
	}
}

func TestMapTextDone_NoTextStarted(t *testing.T) {
	state := &streamState{
		itemID:      "item_1",
		outputIndex: 0,
		textStarted: false,
	}

	ev := provider.ProviderEvent{
		Type: provider.ProviderEventTextDone,
	}

	events := mapProviderEvent(ev, state)
	if len(events) != 0 {
		t.Fatalf("expected 0 events when no text was started, got %d", len(events))
	}
}

func TestSequenceNumbers_Monotonic(t *testing.T) {
	state := &streamState{
		itemID:      "item_1",
		outputIndex: 0,
		textStarted: true,
	}

	deltas := []string{"Hello", " ", "world"}
	var seqs []int

	for _, d := range deltas {
		ev := provider.ProviderEvent{
			Type:  provider.ProviderEventTextDelta,
			Delta: d,
		}
		events := mapProviderEvent(ev, state)
		for _, e := range events {
			seqs = append(seqs, e.SequenceNumber)
		}
	}

	if len(seqs) != 3 {
		t.Fatalf("expected 3 sequence numbers, got %d", len(seqs))
	}

	for i := 1; i < len(seqs); i++ {
		if seqs[i] != seqs[i-1]+1 {
			t.Errorf("sequence numbers not monotonic: %v", seqs)
			break
		}
	}
}

func TestMapDoneEvent_NoOutput(t *testing.T) {
	state := &streamState{}

	ev := provider.ProviderEvent{
		Type: provider.ProviderEventDone,
	}

	events := mapProviderEvent(ev, state)
	if len(events) != 0 {
		t.Fatalf("expected 0 events for done (handled by engine), got %d", len(events))
	}
}

func TestMapErrorEvent_NoOutput(t *testing.T) {
	state := &streamState{}

	ev := provider.ProviderEvent{
		Type: provider.ProviderEventError,
	}

	events := mapProviderEvent(ev, state)
	if len(events) != 0 {
		t.Fatalf("expected 0 events for error (handled by engine), got %d", len(events))
	}
}

func TestMapToolCallDelta_FirstDelta(t *testing.T) {
	state := &streamState{}

	ev := provider.ProviderEvent{
		Type:          provider.ProviderEventToolCallDelta,
		ToolCallIndex: 0,
		ToolCallID:    "call_1",
		FunctionName:  "get_weather",
		Delta:         `{"city":`,
	}

	events := mapProviderEvent(ev, state)

	// First delta should produce output_item.added + arguments delta.
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Type != api.EventOutputItemAdded {
		t.Errorf("event[0] type = %q, want %q", events[0].Type, api.EventOutputItemAdded)
	}
	if events[0].Item == nil || events[0].Item.Type != api.ItemTypeFunctionCall {
		t.Error("expected function_call item in output_item.added")
	}

	if events[1].Type != api.EventFunctionCallArgsDelta {
		t.Errorf("event[1] type = %q, want %q", events[1].Type, api.EventFunctionCallArgsDelta)
	}
	if events[1].Delta != `{"city":` {
		t.Errorf("event[1] delta = %q, want %q", events[1].Delta, `{"city":`)
	}
}

func TestMapToolCallDelta_ContinuationDelta(t *testing.T) {
	state := &streamState{
		toolCallItems: map[int]*toolCallItemState{
			0: {itemID: "item_1", outputIndex: 1, started: true},
		},
	}

	ev := provider.ProviderEvent{
		Type:          provider.ProviderEventToolCallDelta,
		ToolCallIndex: 0,
		ToolCallID:    "call_1",
		Delta:         `"Berlin"}`,
	}

	events := mapProviderEvent(ev, state)

	// Continuation delta should produce only arguments delta (no item.added).
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Type != api.EventFunctionCallArgsDelta {
		t.Errorf("type = %q, want %q", events[0].Type, api.EventFunctionCallArgsDelta)
	}
}

func TestMapToolCallDone(t *testing.T) {
	state := &streamState{
		toolCallItems: map[int]*toolCallItemState{
			0: {itemID: "item_1", outputIndex: 1, started: true},
		},
	}

	ev := provider.ProviderEvent{
		Type:          provider.ProviderEventToolCallDone,
		ToolCallIndex: 0,
		ToolCallID:    "call_1",
		FunctionName:  "get_weather",
		Delta:         `{"city":"Berlin"}`,
		Item: &api.Item{
			Type:   api.ItemTypeFunctionCall,
			Status: api.ItemStatusCompleted,
			FunctionCall: &api.FunctionCallData{
				Name:      "get_weather",
				CallID:    "call_1",
				Arguments: `{"city":"Berlin"}`,
			},
		},
	}

	events := mapProviderEvent(ev, state)

	// Should produce args done + item done.
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Type != api.EventFunctionCallArgsDone {
		t.Errorf("event[0] type = %q, want %q", events[0].Type, api.EventFunctionCallArgsDone)
	}

	if events[1].Type != api.EventOutputItemDone {
		t.Errorf("event[1] type = %q, want %q", events[1].Type, api.EventOutputItemDone)
	}
	if events[1].Item == nil || events[1].Item.FunctionCall == nil {
		t.Fatal("expected function_call data in item done")
	}
	if events[1].Item.ID != "item_1" {
		t.Errorf("item ID = %q, want %q (should use engine-assigned ID)", events[1].Item.ID, "item_1")
	}
}
