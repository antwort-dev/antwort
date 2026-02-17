package vllm

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

// parseSSEStream reads Chat Completions SSE chunks from the given reader,
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
func parseSSEStream(ctx context.Context, body io.Reader, ch chan<- provider.ProviderEvent) {
	scanner := bufio.NewScanner(body)
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
		var chunk chatCompletionChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			slog.Warn("skipping malformed SSE chunk",
				"error", err.Error(),
				"data", truncate(payload, 200),
			)
			continue
		}

		// Translate chunk to provider events and send them.
		translateChunk(&chunk, ch)
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

// translateChunk converts a single chatCompletionChunk into one or more
// ProviderEvent values sent on the channel.
func translateChunk(chunk *chatCompletionChunk, ch chan<- provider.ProviderEvent) {
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

		// Emit text done if we had text content.
		// The engine tracks whether text was started and handles this.
		ch <- provider.ProviderEvent{
			Type:  provider.ProviderEventTextDone,
			Delta: extractDeltaContent(delta.Content),
		}

		// Emit done with status and optional usage.
		doneEvent := provider.ProviderEvent{
			Type: provider.ProviderEventDone,
			Item: &api.Item{
				Status: mapFinishReasonToItemStatus(reason),
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

	// Handle tool call deltas (Phase 5, log warning for now).
	if len(delta.ToolCalls) > 0 {
		slog.Warn("tool call chunks received but tool call streaming not yet implemented (Phase 5)",
			"tool_call_count", len(delta.ToolCalls),
		)
		return
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
	// The delta has a role but no content yet.
	if delta.Role != "" {
		ch <- provider.ProviderEvent{
			Type:  provider.ProviderEventTextDelta,
			Delta: "", // Empty delta signals new message start.
		}
		return
	}

	// Empty delta with no content, no role, no tool calls.
	// This can happen with some backends. Silently skip.
}

// mapFinishReasonToItemStatus maps a Chat Completions finish_reason to an
// Item status value. This is used during streaming to set the final item
// status.
func mapFinishReasonToItemStatus(reason string) api.ItemStatus {
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

// extractDeltaContent safely extracts the content string from a delta pointer.
func extractDeltaContent(content *string) string {
	if content == nil {
		return ""
	}
	return *content
}

// truncate limits a string to maxLen characters for log output.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
