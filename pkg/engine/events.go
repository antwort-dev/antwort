package engine

import (
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

// streamState tracks the state of a streaming response being assembled.
// It holds the running sequence number and output item tracking needed
// to generate the correct OpenResponses event sequence.
type streamState struct {
	seq         int    // Next sequence number (monotonically increasing from 0).
	itemID      string // Current output item ID.
	outputIndex int    // Current output index (position in response output array).
	textStarted bool   // Whether text content has started for the current item.
}

// nextSeq returns the current sequence number and increments it.
func (s *streamState) nextSeq() int {
	n := s.seq
	s.seq++
	return n
}

// mapTextDelta converts a ProviderEventTextDelta to a StreamEvent.
// An empty delta signals a new message start (role-only chunk from the backend).
func mapTextDelta(ev provider.ProviderEvent, state *streamState) []api.StreamEvent {
	if ev.Delta == "" && !state.textStarted {
		// Empty delta = role-only first chunk. Don't emit a text delta event,
		// but mark text as started. The lifecycle events (item.added,
		// part.added) are emitted by the engine, not here.
		state.textStarted = true
		return nil
	}

	if ev.Delta == "" {
		return nil
	}

	state.textStarted = true
	return []api.StreamEvent{
		{
			Type:           api.EventOutputTextDelta,
			SequenceNumber: state.nextSeq(),
			Delta:          ev.Delta,
			ItemID:         state.itemID,
			OutputIndex:    state.outputIndex,
			ContentIndex:   0,
		},
	}
}

// mapTextDone converts a ProviderEventTextDone to StreamEvent(s).
func mapTextDone(ev provider.ProviderEvent, state *streamState) []api.StreamEvent {
	if !state.textStarted {
		return nil
	}

	return []api.StreamEvent{
		{
			Type:           api.EventOutputTextDone,
			SequenceNumber: state.nextSeq(),
			Delta:          ev.Delta, // Final accumulated text (may be empty).
			ItemID:         state.itemID,
			OutputIndex:    state.outputIndex,
			ContentIndex:   0,
		},
	}
}

// mapProviderEvent converts a ProviderEvent into zero or more StreamEvents.
// Lifecycle events (response.created, item.added, etc.) are NOT generated
// here; they are managed by the engine's streaming loop.
func mapProviderEvent(ev provider.ProviderEvent, state *streamState) []api.StreamEvent {
	switch ev.Type {
	case provider.ProviderEventTextDelta:
		return mapTextDelta(ev, state)
	case provider.ProviderEventTextDone:
		return mapTextDone(ev, state)
	case provider.ProviderEventDone:
		// Done events are handled by the engine to emit terminal lifecycle
		// events (content_part.done, item.done, response.completed).
		return nil
	case provider.ProviderEventError:
		// Error events are handled by the engine to emit response.failed.
		return nil
	default:
		// Tool call events, reasoning events: not mapped in Phase 4.
		return nil
	}
}
