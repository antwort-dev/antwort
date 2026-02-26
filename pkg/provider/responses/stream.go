package responses

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

// parseSSEStream reads Responses API SSE events from the reader and maps them
// to ProviderEvent values sent to the channel. The channel is closed when the
// stream ends (response.completed/failed) or an error occurs.
func parseSSEStream(r io.Reader, ch chan<- provider.ProviderEvent) {
	defer close(ch)

	scanner := bufio.NewScanner(r)
	var currentEvent string

	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: "event: <type>" followed by "data: <json>"
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- provider.ProviderEvent{Type: provider.ProviderEventDone}
				return
			}

			if currentEvent != "" {
				handleSSEEvent(currentEvent, []byte(data), ch)
				currentEvent = ""
			}
			continue
		}

		// Empty lines are SSE delimiters, ignore them.
	}

	if err := scanner.Err(); err != nil {
		ch <- provider.ProviderEvent{
			Type: provider.ProviderEventError,
			Err:  fmt.Errorf("SSE stream read: %w", err),
		}
	}
}

// handleSSEEvent processes a single SSE event and emits the corresponding ProviderEvent.
func handleSSEEvent(eventType string, data []byte, ch chan<- provider.ProviderEvent) {
	switch eventType {
	case eventTextDelta:
		var d textDeltaData
		if err := json.Unmarshal(data, &d); err != nil {
			slog.Debug("failed to parse text delta", "error", err)
			return
		}
		ch <- provider.ProviderEvent{
			Type:  provider.ProviderEventTextDelta,
			Delta: d.Delta,
		}

	case eventTextDone:
		ch <- provider.ProviderEvent{Type: provider.ProviderEventTextDone}

	case eventFuncCallArgsDelta:
		var d funcCallArgsDeltaData
		if err := json.Unmarshal(data, &d); err != nil {
			slog.Debug("failed to parse function call args delta", "error", err)
			return
		}
		ch <- provider.ProviderEvent{
			Type:          provider.ProviderEventToolCallDelta,
			Delta:         d.Delta,
			ToolCallIndex: d.OutputIndex,
			ToolCallID:    d.CallID,
			FunctionName:  d.Name,
		}

	case eventFuncCallArgsDone:
		var d funcCallArgsDoneData
		if err := json.Unmarshal(data, &d); err != nil {
			slog.Debug("failed to parse function call args done", "error", err)
			return
		}
		ch <- provider.ProviderEvent{
			Type:          provider.ProviderEventToolCallDone,
			ToolCallIndex: d.OutputIndex,
			ToolCallID:    d.CallID,
			FunctionName:  d.Name,
		}

	case eventReasoningDelta:
		var d textDeltaData
		if err := json.Unmarshal(data, &d); err != nil {
			slog.Debug("failed to parse reasoning delta", "error", err)
			return
		}
		ch <- provider.ProviderEvent{
			Type:  provider.ProviderEventReasoningDelta,
			Delta: d.Delta,
		}

	case eventReasoningDone:
		ch <- provider.ProviderEvent{Type: provider.ProviderEventReasoningDone}

	case eventResponseCompleted:
		var d responseCompletedData
		if err := json.Unmarshal(data, &d); err != nil {
			slog.Debug("failed to parse response completed", "error", err)
			ch <- provider.ProviderEvent{Type: provider.ProviderEventDone}
			return
		}
		// Extract usage from the completed response.
		ev := provider.ProviderEvent{Type: provider.ProviderEventDone}
		if d.Response.Usage != nil {
			ev.Usage = &api.Usage{
				InputTokens:  d.Response.Usage.InputTokens,
				OutputTokens: d.Response.Usage.OutputTokens,
				TotalTokens:  d.Response.Usage.TotalTokens,
			}
		}
		ch <- ev

	case eventResponseFailed:
		ch <- provider.ProviderEvent{
			Type: provider.ProviderEventError,
			Err:  fmt.Errorf("backend response failed"),
		}

	case eventResponseCreated, eventOutputItemAdded, eventOutputItemDone,
		eventContentPartAdded, eventContentPartDone:
		// Lifecycle events that don't carry data needed by the engine.
		// The engine synthesizes its own lifecycle events.

	default:
		slog.Debug("unknown SSE event type, skipping", "event", eventType)
	}
}
