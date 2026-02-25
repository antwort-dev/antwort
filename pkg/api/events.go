package api

import (
	"encoding/json"
	"strings"
)

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
	EventReasoningDelta        StreamEventType = "response.reasoning.delta"
	EventReasoningDone         StreamEventType = "response.reasoning.done"
	EventResponseIncomplete    StreamEventType = "response.incomplete"
	EventError                 StreamEventType = "error"
	EventRefusalDelta          StreamEventType = "response.refusal.delta"
	EventRefusalDone           StreamEventType = "response.refusal.done"

	// Tool lifecycle events emitted during agentic loop tool execution.
	EventMCPCallInProgress         StreamEventType = "response.mcp_call.in_progress"
	EventMCPCallCompleted          StreamEventType = "response.mcp_call.completed"
	EventMCPCallFailed             StreamEventType = "response.mcp_call.failed"
	EventFileSearchCallInProgress  StreamEventType = "response.file_search_call.in_progress"
	EventFileSearchCallSearching   StreamEventType = "response.file_search_call.searching"
	EventFileSearchCallCompleted   StreamEventType = "response.file_search_call.completed"
	EventWebSearchCallInProgress   StreamEventType = "response.web_search_call.in_progress"
	EventWebSearchCallSearching    StreamEventType = "response.web_search_call.searching"
	EventWebSearchCallCompleted        StreamEventType = "response.web_search_call.completed"
	EventCodeInterpreterInProgress     StreamEventType = "response.code_interpreter_call.in_progress"
	EventCodeInterpreterInterpreting   StreamEventType = "response.code_interpreter_call.interpreting"
	EventCodeInterpreterCompleted      StreamEventType = "response.code_interpreter_call.completed"
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
// Fields are included or omitted based on the event type via custom marshaling.
type StreamEvent struct {
	Type           StreamEventType    `json:"-"`
	SequenceNumber int                `json:"-"`
	Response       *Response          `json:"-"`
	Item           *Item              `json:"-"`
	Part           *OutputContentPart `json:"-"`
	Delta          string             `json:"-"`
	ItemID         string             `json:"-"`
	OutputIndex    int                `json:"-"`
	ContentIndex   int                `json:"-"`
}

// MarshalJSON serializes a StreamEvent with the correct fields for each event type.
func (e StreamEvent) MarshalJSON() ([]byte, error) {
	switch e.Type {
	case EventResponseCreated, EventResponseQueued, EventResponseInProgress,
		EventResponseCompleted, EventResponseFailed, EventResponseCancelled,
		EventResponseRequiresAction, EventResponseIncomplete:
		// Lifecycle events: type + sequence_number + response.
		return json.Marshal(struct {
			Type           StreamEventType `json:"type"`
			SequenceNumber int             `json:"sequence_number"`
			Response       *Response       `json:"response,omitempty"`
		}{e.Type, e.SequenceNumber, e.Response})

	case EventOutputItemAdded, EventOutputItemDone:
		// Item events: type + sequence_number + output_index + item.
		return json.Marshal(struct {
			Type           StreamEventType `json:"type"`
			SequenceNumber int             `json:"sequence_number"`
			OutputIndex    int             `json:"output_index"`
			Item           *Item           `json:"item,omitempty"`
		}{e.Type, e.SequenceNumber, e.OutputIndex, e.Item})

	case EventContentPartAdded, EventContentPartDone:
		// Content part events: type + seq + item_id + output_index + content_index + part.
		return json.Marshal(struct {
			Type           StreamEventType    `json:"type"`
			SequenceNumber int                `json:"sequence_number"`
			ItemID         string             `json:"item_id"`
			OutputIndex    int                `json:"output_index"`
			ContentIndex   int                `json:"content_index"`
			Part           *OutputContentPart `json:"part,omitempty"`
		}{e.Type, e.SequenceNumber, e.ItemID, e.OutputIndex, e.ContentIndex, e.Part})

	case EventOutputTextDelta:
		// Text delta: type + seq + item_id + output_index + content_index + delta + logprobs.
		return json.Marshal(struct {
			Type           StreamEventType  `json:"type"`
			SequenceNumber int              `json:"sequence_number"`
			ItemID         string           `json:"item_id"`
			OutputIndex    int              `json:"output_index"`
			ContentIndex   int              `json:"content_index"`
			Delta          string           `json:"delta"`
			Logprobs       []TokenLogprob   `json:"logprobs"`
		}{e.Type, e.SequenceNumber, e.ItemID, e.OutputIndex, e.ContentIndex, e.Delta, []TokenLogprob{}})

	case EventOutputTextDone:
		// Text done: type + seq + item_id + output_index + content_index + text + logprobs.
		return json.Marshal(struct {
			Type           StreamEventType  `json:"type"`
			SequenceNumber int              `json:"sequence_number"`
			ItemID         string           `json:"item_id"`
			OutputIndex    int              `json:"output_index"`
			ContentIndex   int              `json:"content_index"`
			Text           string           `json:"text"`
			Logprobs       []TokenLogprob   `json:"logprobs"`
		}{e.Type, e.SequenceNumber, e.ItemID, e.OutputIndex, e.ContentIndex, e.Delta, []TokenLogprob{}})

	case EventFunctionCallArgsDelta:
		// Function call args delta: type + seq + item_id + output_index + delta.
		return json.Marshal(struct {
			Type           StreamEventType `json:"type"`
			SequenceNumber int             `json:"sequence_number"`
			ItemID         string          `json:"item_id"`
			OutputIndex    int             `json:"output_index"`
			Delta          string          `json:"delta"`
		}{e.Type, e.SequenceNumber, e.ItemID, e.OutputIndex, e.Delta})

	case EventFunctionCallArgsDone:
		// Function call args done: type + seq + item_id + output_index + arguments.
		return json.Marshal(struct {
			Type           StreamEventType `json:"type"`
			SequenceNumber int             `json:"sequence_number"`
			ItemID         string          `json:"item_id"`
			OutputIndex    int             `json:"output_index"`
			Arguments      string          `json:"arguments"`
		}{e.Type, e.SequenceNumber, e.ItemID, e.OutputIndex, e.Delta})

	case EventReasoningDelta:
		// Reasoning delta: type + seq + item_id + output_index + content_index + delta.
		return json.Marshal(struct {
			Type           StreamEventType `json:"type"`
			SequenceNumber int             `json:"sequence_number"`
			ItemID         string          `json:"item_id"`
			OutputIndex    int             `json:"output_index"`
			ContentIndex   int             `json:"content_index"`
			Delta          string          `json:"delta"`
		}{e.Type, e.SequenceNumber, e.ItemID, e.OutputIndex, e.ContentIndex, e.Delta})

	case EventReasoningDone:
		// Reasoning done: type + seq + item_id + output_index + content_index.
		return json.Marshal(struct {
			Type           StreamEventType `json:"type"`
			SequenceNumber int             `json:"sequence_number"`
			ItemID         string          `json:"item_id"`
			OutputIndex    int             `json:"output_index"`
			ContentIndex   int             `json:"content_index"`
		}{e.Type, e.SequenceNumber, e.ItemID, e.OutputIndex, e.ContentIndex})

	case EventError:
		// Error event: type + error fields (no response wrapper).
		return json.Marshal(struct {
			Type           StreamEventType `json:"type"`
			SequenceNumber int             `json:"sequence_number"`
			Error          *APIError       `json:"error,omitempty"`
		}{e.Type, e.SequenceNumber, extractError(e.Response)})

	case EventRefusalDelta:
		// Refusal delta: type + seq + item_id + output_index + content_index + delta.
		return json.Marshal(struct {
			Type           StreamEventType `json:"type"`
			SequenceNumber int             `json:"sequence_number"`
			ItemID         string          `json:"item_id"`
			OutputIndex    int             `json:"output_index"`
			ContentIndex   int             `json:"content_index"`
			Delta          string          `json:"delta"`
		}{e.Type, e.SequenceNumber, e.ItemID, e.OutputIndex, e.ContentIndex, e.Delta})

	case EventRefusalDone:
		// Refusal done: type + seq + item_id + output_index + content_index.
		return json.Marshal(struct {
			Type           StreamEventType `json:"type"`
			SequenceNumber int             `json:"sequence_number"`
			ItemID         string          `json:"item_id"`
			OutputIndex    int             `json:"output_index"`
			ContentIndex   int             `json:"content_index"`
		}{e.Type, e.SequenceNumber, e.ItemID, e.OutputIndex, e.ContentIndex})

	case EventMCPCallInProgress, EventMCPCallCompleted, EventMCPCallFailed,
		EventFileSearchCallInProgress, EventFileSearchCallSearching, EventFileSearchCallCompleted,
		EventWebSearchCallInProgress, EventWebSearchCallSearching, EventWebSearchCallCompleted,
		EventCodeInterpreterInProgress, EventCodeInterpreterInterpreting, EventCodeInterpreterCompleted:
		// Tool lifecycle events: type + seq + item_id + output_index.
		return json.Marshal(struct {
			Type           StreamEventType `json:"type"`
			SequenceNumber int             `json:"sequence_number"`
			ItemID         string          `json:"item_id"`
			OutputIndex    int             `json:"output_index"`
		}{e.Type, e.SequenceNumber, e.ItemID, e.OutputIndex})

	default:
		// Fallback: include all non-zero fields.
		return json.Marshal(struct {
			Type           StreamEventType    `json:"type"`
			SequenceNumber int                `json:"sequence_number"`
			Response       *Response          `json:"response,omitempty"`
			Item           *Item              `json:"item,omitempty"`
			Part           *OutputContentPart `json:"part,omitempty"`
			Delta          string             `json:"delta,omitempty"`
			ItemID         string             `json:"item_id,omitempty"`
			OutputIndex    int                `json:"output_index,omitempty"`
			ContentIndex   int                `json:"content_index,omitempty"`
		}{e.Type, e.SequenceNumber, e.Response, e.Item, e.Part, e.Delta, e.ItemID, e.OutputIndex, e.ContentIndex})
	}
}

// UnmarshalJSON deserializes a StreamEvent.
func (e *StreamEvent) UnmarshalJSON(data []byte) error {
	// Parse all possible fields.
	var raw struct {
		Type           StreamEventType    `json:"type"`
		SequenceNumber int                `json:"sequence_number"`
		Response       *Response          `json:"response"`
		Item           *Item              `json:"item"`
		Part           *OutputContentPart `json:"part"`
		Delta          string             `json:"delta"`
		Text           string             `json:"text"`
		Arguments      string             `json:"arguments"`
		ItemID         string             `json:"item_id"`
		OutputIndex    int                `json:"output_index"`
		ContentIndex   int                `json:"content_index"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.Type = raw.Type
	e.SequenceNumber = raw.SequenceNumber
	e.Response = raw.Response
	e.Item = raw.Item
	e.Part = raw.Part
	e.ItemID = raw.ItemID
	e.OutputIndex = raw.OutputIndex
	e.ContentIndex = raw.ContentIndex

	// Delta can come from delta, text, or arguments depending on event type.
	if raw.Delta != "" {
		e.Delta = raw.Delta
	} else if raw.Text != "" {
		e.Delta = raw.Text
	} else if raw.Arguments != "" {
		e.Delta = raw.Arguments
	}

	return nil
}

// extractError pulls the APIError from a Response, or returns nil.
func extractError(r *Response) *APIError {
	if r != nil {
		return r.Error
	}
	return nil
}

// IsExtensionEvent returns true if the event type follows the "provider:event_type"
// pattern used for provider-specific extension events.
func IsExtensionEvent(t StreamEventType) bool {
	return strings.Contains(string(t), ":")
}
