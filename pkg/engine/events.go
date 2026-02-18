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
	itemID      string // Current output item ID (for text message).
	outputIndex int    // Current output index (position in response output array).
	textStarted bool   // Whether text content has started for the current item.

	// Tool call tracking: maps tool call index to item ID and output index.
	toolCallItems map[int]*toolCallItemState
}

// toolCallItemState tracks a single tool call item during streaming.
type toolCallItemState struct {
	itemID      string
	outputIndex int
	started     bool // Whether the first delta has been emitted.
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

// mapToolCallDelta converts a ProviderEventToolCallDelta to StreamEvent(s).
// On the first delta for a tool call index, it emits output_item.added.
// Subsequent deltas emit function_call_arguments.delta.
func mapToolCallDelta(ev provider.ProviderEvent, state *streamState) []api.StreamEvent {
	if state.toolCallItems == nil {
		state.toolCallItems = make(map[int]*toolCallItemState)
	}

	tcState, exists := state.toolCallItems[ev.ToolCallIndex]
	if !exists {
		// First delta for this tool call: create a new item.
		state.outputIndex++
		tcState = &toolCallItemState{
			itemID:      api.NewItemID(),
			outputIndex: state.outputIndex,
		}
		state.toolCallItems[ev.ToolCallIndex] = tcState
	}

	var events []api.StreamEvent

	if !tcState.started {
		tcState.started = true
		// Emit output_item.added for the function_call item.
		events = append(events, api.StreamEvent{
			Type:           api.EventOutputItemAdded,
			SequenceNumber: state.nextSeq(),
			Item: &api.Item{
				ID:     tcState.itemID,
				Type:   api.ItemTypeFunctionCall,
				Status: api.ItemStatusInProgress,
				FunctionCall: &api.FunctionCallData{
					Name:   ev.FunctionName,
					CallID: ev.ToolCallID,
				},
			},
			OutputIndex: tcState.outputIndex,
		})
	}

	// Emit arguments delta (skip empty deltas).
	if ev.Delta != "" {
		events = append(events, api.StreamEvent{
			Type:           api.EventFunctionCallArgsDelta,
			SequenceNumber: state.nextSeq(),
			Delta:          ev.Delta,
			ItemID:         tcState.itemID,
			OutputIndex:    tcState.outputIndex,
		})
	}

	return events
}

// mapToolCallDone converts a ProviderEventToolCallDone to StreamEvent(s).
func mapToolCallDone(ev provider.ProviderEvent, state *streamState) []api.StreamEvent {
	if state.toolCallItems == nil {
		return nil
	}

	tcState, exists := state.toolCallItems[ev.ToolCallIndex]
	if !exists {
		return nil
	}

	var events []api.StreamEvent

	// Emit function_call_arguments.done with complete arguments.
	events = append(events, api.StreamEvent{
		Type:           api.EventFunctionCallArgsDone,
		SequenceNumber: state.nextSeq(),
		Delta:          ev.Delta, // Complete arguments string.
		ItemID:         tcState.itemID,
		OutputIndex:    tcState.outputIndex,
	})

	// Emit output_item.done with the complete function_call item.
	if ev.Item != nil {
		// Use the item ID we assigned, not the one from the provider.
		itemCopy := *ev.Item
		itemCopy.ID = tcState.itemID
		events = append(events, api.StreamEvent{
			Type:           api.EventOutputItemDone,
			SequenceNumber: state.nextSeq(),
			Item:           &itemCopy,
			OutputIndex:    tcState.outputIndex,
		})
	}

	return events
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
	case provider.ProviderEventToolCallDelta:
		return mapToolCallDelta(ev, state)
	case provider.ProviderEventToolCallDone:
		return mapToolCallDone(ev, state)
	case provider.ProviderEventDone:
		// Done events are handled by the engine to emit terminal lifecycle
		// events (content_part.done, item.done, response.completed).
		return nil
	case provider.ProviderEventError:
		// Error events are handled by the engine to emit response.failed.
		return nil
	default:
		// Reasoning events: not mapped yet.
		return nil
	}
}
