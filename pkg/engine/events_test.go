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

func TestMapToolCallDelta_Ignored(t *testing.T) {
	state := &streamState{}

	ev := provider.ProviderEvent{
		Type: provider.ProviderEventToolCallDelta,
	}

	events := mapProviderEvent(ev, state)
	if len(events) != 0 {
		t.Fatalf("expected 0 events for tool call delta (Phase 5), got %d", len(events))
	}
}
