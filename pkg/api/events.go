package api

import "strings"

// StreamEventType identifies the type of a streaming event.
type StreamEventType string

// Delta events are emitted during streaming to convey incremental content.
const (
	EventOutputItemAdded       StreamEventType = "response.output_item.added"
	EventContentPartAdded      StreamEventType = "response.content_part.added"
	EventOutputTextDelta       StreamEventType = "response.output_text.delta"
	EventOutputTextDone        StreamEventType = "response.output_text.done"
	EventFunctionCallArgsDelta StreamEventType = "response.function_call_arguments.delta"
	EventFunctionCallArgsDone  StreamEventType = "response.function_call_arguments.done"
	EventContentPartDone       StreamEventType = "response.content_part.done"
	EventOutputItemDone        StreamEventType = "response.output_item.done"
)

// State machine events track the lifecycle of a response.
const (
	EventResponseCreated    StreamEventType = "response.created"
	EventResponseQueued     StreamEventType = "response.queued"
	EventResponseInProgress StreamEventType = "response.in_progress"
	EventResponseCompleted  StreamEventType = "response.completed"
	EventResponseFailed         StreamEventType = "response.failed"
	EventResponseCancelled      StreamEventType = "response.cancelled"
	EventResponseRequiresAction StreamEventType = "response.requires_action"
)

// StreamEvent represents a single server-sent event in a streaming response.
type StreamEvent struct {
	Type           StreamEventType    `json:"type"`
	SequenceNumber int                `json:"sequence_number"`
	Response       *Response          `json:"response,omitempty"`
	Item           *Item              `json:"item,omitempty"`
	Part           *OutputContentPart `json:"part,omitempty"`
	Delta          string             `json:"delta,omitempty"`
	ItemID         string             `json:"item_id,omitempty"`
	OutputIndex    int                `json:"output_index,omitempty"`
	ContentIndex   int                `json:"content_index,omitempty"`
}

// IsExtensionEvent returns true if the event type follows the "provider:event_type"
// pattern used for provider-specific extension events.
func IsExtensionEvent(t StreamEventType) bool {
	return strings.Contains(string(t), ":")
}
