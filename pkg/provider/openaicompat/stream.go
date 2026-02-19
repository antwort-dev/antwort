package openaicompat

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

// ToolCallBuffer tracks incremental tool call argument assembly across
// multiple SSE chunks for a single tool call index.
type ToolCallBuffer struct {
	ID   string
	Name string
	Args strings.Builder
}

// ParseSSEStream reads Chat Completions SSE chunks from the given reader,
// translates each chunk to ProviderEvent values, and sends them on ch.
// The channel is NOT closed by this function; the caller is responsible
// for closing it.
//
// SSE format expected:
//
//	data: {"id":"...","choices":[...]}\n
//	\n
//	data: [DONE]\n
//	\n
//
// Malformed chunks are logged and skipped. Context cancellation stops
// reading immediately.
func ParseSSEStream(ctx context.Context, body io.Reader, ch chan<- provider.ProviderEvent) {
	scanner := bufio.NewScanner(body)

	// Track tool call argument buffers across chunks (keyed by tool call index).
	toolCalls := make(map[int]*ToolCallBuffer)

	for scanner.Scan() {
		// Check for context cancellation between lines.
		if ctx.Err() != nil {
			return
		}

		line := scanner.Text()

		// SSE lines that don't start with "data: " are ignored
		// (e.g., empty lines, comments starting with ":").
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")

		// Handle the [DONE] sentinel.
		if payload == "[DONE]" {
			return
		}

		// Parse the JSON chunk.
		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			slog.Warn("skipping malformed SSE chunk",
				"error", err.Error(),
				"data", Truncate(payload, 200),
			)
			continue
		}

		// Translate chunk to provider events and send them.
		TranslateChunk(&chunk, toolCalls, ch)
	}

	// Scanner error (e.g., connection dropped).
	if err := scanner.Err(); err != nil {
		// Context cancellation is not an error from our perspective.
		if ctx.Err() != nil {
			return
		}
		ch <- provider.ProviderEvent{
			Type: provider.ProviderEventError,
			Err:  api.NewServerError("SSE stream read error: " + err.Error()),
		}
	}
}

// TranslateChunk converts a single ChatCompletionChunk into one or more
// ProviderEvent values sent on the channel. The toolCalls map tracks
// incremental tool call argument assembly across chunks.
func TranslateChunk(chunk *ChatCompletionChunk, toolCalls map[int]*ToolCallBuffer, ch chan<- provider.ProviderEvent) {
	// No choices means nothing to translate (e.g., a usage-only final chunk).
	if len(chunk.Choices) == 0 {
		// Check if this is a usage-only chunk (sent with stream_options.include_usage).
		if chunk.Usage != nil {
			ch <- provider.ProviderEvent{
				Type: provider.ProviderEventDone,
				Usage: &api.Usage{
					InputTokens:  chunk.Usage.PromptTokens,
					OutputTokens: chunk.Usage.CompletionTokens,
					TotalTokens:  chunk.Usage.TotalTokens,
				},
			}
		}
		return
	}

	choice := chunk.Choices[0]
	delta := choice.Delta

	// Check for finish_reason signaling stream completion for this choice.
	if choice.FinishReason != nil {
		reason := *choice.FinishReason

		// If we have buffered tool calls, flush them as done events.
		if reason == "tool_calls" || len(toolCalls) > 0 {
			FlushToolCalls(toolCalls, ch)
		}

		// Emit text done if we had text content.
		ch <- provider.ProviderEvent{
			Type:  provider.ProviderEventTextDone,
			Delta: ExtractDeltaContent(delta.Content),
		}

		// Emit done with status and optional usage.
		doneEvent := provider.ProviderEvent{
			Type: provider.ProviderEventDone,
			Item: &api.Item{
				Status: MapFinishReasonToItemStatus(reason),
			},
		}
		if chunk.Usage != nil {
			doneEvent.Usage = &api.Usage{
				InputTokens:  chunk.Usage.PromptTokens,
				OutputTokens: chunk.Usage.CompletionTokens,
				TotalTokens:  chunk.Usage.TotalTokens,
			}
		}
		ch <- doneEvent
		return
	}

	// Handle tool call deltas.
	if len(delta.ToolCalls) > 0 {
		for _, tc := range delta.ToolCalls {
			buf, exists := toolCalls[tc.Index]
			if !exists {
				// First chunk for this tool call index: contains id and function name.
				buf = &ToolCallBuffer{
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
				toolCalls[tc.Index] = buf

				// Emit the first delta event with the function name.
				ch <- provider.ProviderEvent{
					Type:          provider.ProviderEventToolCallDelta,
					ToolCallIndex: tc.Index,
					ToolCallID:    tc.ID,
					FunctionName:  tc.Function.Name,
					Delta:         tc.Function.Arguments,
				}
			} else {
				// Continuation chunk: accumulate arguments.
				ch <- provider.ProviderEvent{
					Type:          provider.ProviderEventToolCallDelta,
					ToolCallIndex: tc.Index,
					ToolCallID:    buf.ID,
					Delta:         tc.Function.Arguments,
				}
			}

			// Always accumulate arguments in the buffer.
			buf.Args.WriteString(tc.Function.Arguments)
		}
		return
	}

	// Handle reasoning content delta (e.g., DeepSeek R1).
	if delta.ReasoningContent != nil && *delta.ReasoningContent != "" {
		ch <- provider.ProviderEvent{
			Type:  provider.ProviderEventReasoningDelta,
			Delta: *delta.ReasoningContent,
		}
		// Don't return: the same chunk might also have text content.
	}

	// Handle text content delta.
	if delta.Content != nil && *delta.Content != "" {
		ch <- provider.ProviderEvent{
			Type:  provider.ProviderEventTextDelta,
			Delta: *delta.Content,
		}
		return
	}

	// Handle role-only chunk (first chunk signaling a new message).
	if delta.Role != "" && delta.ReasoningContent == nil {
		ch <- provider.ProviderEvent{
			Type:  provider.ProviderEventTextDelta,
			Delta: "", // Empty delta signals new message start.
		}
		return
	}

	// Empty delta with no content, no role, no tool calls.
	// This can happen with some backends. Silently skip.
}

// FlushToolCalls emits ProviderEventToolCallDone for each buffered tool call
// and clears the buffer.
func FlushToolCalls(toolCalls map[int]*ToolCallBuffer, ch chan<- provider.ProviderEvent) {
	for idx, buf := range toolCalls {
		ch <- provider.ProviderEvent{
			Type:          provider.ProviderEventToolCallDone,
			ToolCallIndex: idx,
			ToolCallID:    buf.ID,
			FunctionName:  buf.Name,
			Delta:         buf.Args.String(),
			Item: &api.Item{
				Type:   api.ItemTypeFunctionCall,
				Status: api.ItemStatusCompleted,
				FunctionCall: &api.FunctionCallData{
					Name:      buf.Name,
					CallID:    buf.ID,
					Arguments: buf.Args.String(),
				},
			},
		}
	}
	// Clear the map.
	for k := range toolCalls {
		delete(toolCalls, k)
	}
}

// MapFinishReasonToItemStatus maps a Chat Completions finish_reason to an
// Item status value. This is used during streaming to set the final item
// status.
func MapFinishReasonToItemStatus(reason string) api.ItemStatus {
	switch reason {
	case "stop":
		return api.ItemStatusCompleted
	case "length":
		return api.ItemStatusIncomplete
	case "tool_calls":
		return api.ItemStatusCompleted
	case "content_filter":
		return api.ItemStatusIncomplete
	default:
		slog.Warn("unknown finish_reason in stream, treating as completed",
			"finish_reason", reason,
		)
		return api.ItemStatusCompleted
	}
}

// ExtractDeltaContent safely extracts the content string from a delta pointer.
func ExtractDeltaContent(content *string) string {
	if content == nil {
		return ""
	}
	return *content
}

// Truncate limits a string to maxLen characters for log output.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
