package engine

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/observability"
	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/tools"
	"github.com/rhuss/antwort/pkg/transport"
)

// runAgenticLoop executes the multi-turn agentic cycle for non-streaming
// requests. It calls provider.Complete in a loop, dispatching tool calls
// to executors, feeding results back, and repeating until a final answer
// is produced or a termination condition is met.
func (e *Engine) runAgenticLoop(ctx context.Context, req *api.CreateResponseRequest, provReq *provider.ProviderRequest, w transport.ResponseWriter) error {
	maxTurns := e.cfg.maxTurns()
	// Apply request-level max_tool_calls if set and lower than server max.
	if req.MaxToolCalls != nil && *req.MaxToolCalls > 0 && *req.MaxToolCalls < maxTurns {
		maxTurns = *req.MaxToolCalls
	}
	parallel := getParallelToolCalls(req)

	var allOutputItems []api.Item
	var cumulativeUsage api.Usage

	for turn := 0; turn < maxTurns; turn++ {
		// Check context before each turn.
		if ctx.Err() != nil {
			return e.buildAndWriteResponse(ctx, req, allOutputItems, &cumulativeUsage, api.ResponseStatusCancelled, nil, w)
		}

		// Call the provider.
		startTime := time.Now()
		provResp, err := e.provider.Complete(ctx, provReq)
		duration := time.Since(startTime)
		provName := e.provider.Name()

		if err != nil {
			observability.ProviderRequestsTotal.WithLabelValues(provName, req.Model, "error").Inc()
			observability.ProviderLatency.WithLabelValues(provName, req.Model).Observe(duration.Seconds())
			if ctx.Err() != nil {
				return e.buildAndWriteResponse(ctx, req, allOutputItems, &cumulativeUsage, api.ResponseStatusCancelled, nil, w)
			}
			return err
		}

		observability.ProviderRequestsTotal.WithLabelValues(provName, req.Model, "success").Inc()
		observability.ProviderLatency.WithLabelValues(provName, req.Model).Observe(duration.Seconds())
		observability.ProviderTokensTotal.WithLabelValues(provName, req.Model, "input").Add(float64(provResp.Usage.InputTokens))
		observability.ProviderTokensTotal.WithLabelValues(provName, req.Model, "output").Add(float64(provResp.Usage.OutputTokens))
		observability.RecordGenAIMetrics(provName, req.Model, duration, provResp.Usage.InputTokens, provResp.Usage.OutputTokens, nil)

		// Accumulate usage.
		cumulativeUsage.InputTokens += provResp.Usage.InputTokens
		cumulativeUsage.OutputTokens += provResp.Usage.OutputTokens
		cumulativeUsage.TotalTokens += provResp.Usage.TotalTokens

		// Assign item IDs.
		for i := range provResp.Items {
			if provResp.Items[i].ID == "" {
				provResp.Items[i].ID = api.NewItemID()
			}
		}

		// Collect output items.
		allOutputItems = append(allOutputItems, provResp.Items...)

		// Extract tool calls from the response.
		toolCalls := extractToolCalls(provResp.Items)

		// No tool calls: final answer. Return completed response.
		if len(toolCalls) == 0 {
			return e.buildAndWriteResponse(ctx, req, allOutputItems, &cumulativeUsage, provResp.Status, nil, w)
		}

		// Check if tool_choice is "none": don't enter loop.
		if req.ToolChoice != nil && req.ToolChoice.String == "none" {
			return e.buildAndWriteResponse(ctx, req, allOutputItems, &cumulativeUsage, api.ResponseStatusCompleted, nil, w)
		}

		// Check if any tool calls require client action (no matching executor).
		if e.hasUnhandledToolCalls(toolCalls) {
			return e.buildAndWriteResponse(ctx, req, allOutputItems, &cumulativeUsage, api.ResponseStatusRequiresAction, nil, w)
		}

		// Filter by allowed_tools.
		filterResult := tools.FilterAllowedTools(toolCalls, req.AllowedTools)

		// Execute allowed tool calls (concurrent or sequential based on request).
		results := e.executeTools(ctx, filterResult.Allowed, parallel)

		// Combine with rejected results.
		allResults := append(results, filterResult.Rejected...)

		// Convert results to function_call_output items and add to output.
		var resultItems []api.Item
		for _, r := range allResults {
			item := api.Item{
				ID:     api.NewItemID(),
				Type:   api.ItemTypeFunctionCallOutput,
				Status: api.ItemStatusCompleted,
				FunctionCallOutput: &api.FunctionCallOutputData{
					CallID: r.CallID,
					Output: r.Output,
				},
			}
			resultItems = append(resultItems, item)
		}
		allOutputItems = append(allOutputItems, resultItems...)

		// Build messages for next turn: first the assistant's tool call message,
		// then the tool results. The assistant message with tool_calls must
		// precede the tool role messages per Chat Completions convention.
		provReq.Messages = append(provReq.Messages, buildAssistantToolCallMessage(toolCalls))
		for _, r := range allResults {
			provReq.Messages = append(provReq.Messages, provider.ProviderMessage{
				Role:       "tool",
				Content:    r.Output,
				ToolCallID: r.CallID,
			})
		}
	}

	// Max turns reached: return incomplete.
	return e.buildAndWriteResponse(ctx, req, allOutputItems, &cumulativeUsage, api.ResponseStatusIncomplete, nil, w)
}

// runAgenticLoopStreaming executes the multi-turn agentic cycle for streaming
// requests. It manages event emission across turns with a single lifecycle.
func (e *Engine) runAgenticLoopStreaming(ctx context.Context, req *api.CreateResponseRequest, provReq *provider.ProviderRequest, w transport.ResponseWriter) error {
	maxTurns := e.cfg.maxTurns()
	if req.MaxToolCalls != nil && *req.MaxToolCalls > 0 && *req.MaxToolCalls < maxTurns {
		maxTurns = *req.MaxToolCalls
	}
	parallel := getParallelToolCalls(req)

	var cumulativeUsage api.Usage
	var allOutputItems []api.Item

	// Build initial response skeleton.
	resp := buildResponseFromRequest(req, api.ResponseStatusInProgress)

	state := &streamState{}

	// Emit response.created and response.in_progress once.
	if err := w.WriteEvent(ctx, api.StreamEvent{
		Type: api.EventResponseCreated, SequenceNumber: state.nextSeq(),
		Response: snapshotResponse(resp),
	}); err != nil {
		return err
	}
	if err := w.WriteEvent(ctx, api.StreamEvent{
		Type: api.EventResponseInProgress, SequenceNumber: state.nextSeq(),
		Response: snapshotResponse(resp),
	}); err != nil {
		return err
	}

	for turn := 0; turn < maxTurns; turn++ {
		if ctx.Err() != nil {
			return e.emitCancelled(ctx, resp, state, w)
		}

		// Start provider stream for this turn.
		turnStreamStart := time.Now()
		eventCh, err := e.provider.Stream(ctx, provReq)
		if err != nil {
			provName := e.provider.Name()
			observability.ProviderRequestsTotal.WithLabelValues(provName, req.Model, "error").Inc()
			observability.ProviderLatency.WithLabelValues(provName, req.Model).Observe(time.Since(turnStreamStart).Seconds())
			if ctx.Err() != nil {
				return e.emitCancelled(ctx, resp, state, w)
			}
			return e.emitFailed(ctx, resp, err, state, w)
		}

		// Consume events from this turn, accumulating items.
		turnItems, turnUsage, turnErr := e.consumeStreamTurn(ctx, eventCh, state, w)
		turnDuration := time.Since(turnStreamStart)

		// Record provider metrics for this streaming turn.
		{
			provName := e.provider.Name()
			if turnErr != nil {
				observability.ProviderRequestsTotal.WithLabelValues(provName, req.Model, "error").Inc()
			} else {
				observability.ProviderRequestsTotal.WithLabelValues(provName, req.Model, "success").Inc()
			}
			observability.ProviderLatency.WithLabelValues(provName, req.Model).Observe(turnDuration.Seconds())
			if turnUsage != nil {
				observability.ProviderTokensTotal.WithLabelValues(provName, req.Model, "input").Add(float64(turnUsage.InputTokens))
				observability.ProviderTokensTotal.WithLabelValues(provName, req.Model, "output").Add(float64(turnUsage.OutputTokens))
				observability.RecordGenAIMetrics(provName, req.Model, turnDuration, turnUsage.InputTokens, turnUsage.OutputTokens, nil)
			} else {
				observability.RecordGenAIMetrics(provName, req.Model, turnDuration, 0, 0, nil)
			}
		}

		if turnErr != nil {
			return turnErr
		}

		// Accumulate usage.
		if turnUsage != nil {
			cumulativeUsage.InputTokens += turnUsage.InputTokens
			cumulativeUsage.OutputTokens += turnUsage.OutputTokens
			cumulativeUsage.TotalTokens += turnUsage.TotalTokens
		}

		allOutputItems = append(allOutputItems, turnItems...)

		// Extract tool calls from this turn's items.
		toolCalls := extractToolCalls(turnItems)

		// No tool calls: final answer.
		if len(toolCalls) == 0 {
			resp.Output = allOutputItems
			resp.Usage = streamUsage(&cumulativeUsage, req)
			resp.Status = api.ResponseStatusCompleted
			return w.WriteEvent(ctx, api.StreamEvent{
				Type: api.EventResponseCompleted, SequenceNumber: state.nextSeq(),
				Response: resp,
			})
		}

		// Check tool_choice "none".
		if req.ToolChoice != nil && req.ToolChoice.String == "none" {
			resp.Output = allOutputItems
			resp.Usage = streamUsage(&cumulativeUsage, req)
			resp.Status = api.ResponseStatusCompleted
			return w.WriteEvent(ctx, api.StreamEvent{
				Type: api.EventResponseCompleted, SequenceNumber: state.nextSeq(),
				Response: resp,
			})
		}

		// Check for unhandled tool calls (requires_action).
		if e.hasUnhandledToolCalls(toolCalls) {
			resp.Output = allOutputItems
			resp.Usage = streamUsage(&cumulativeUsage, req)
			resp.Status = api.ResponseStatusRequiresAction
			return w.WriteEvent(ctx, api.StreamEvent{
				Type: api.EventResponseRequiresAction, SequenceNumber: state.nextSeq(),
				Response: resp,
			})
		}

		// Filter and execute tools (with lifecycle events in streaming mode).
		filterResult := tools.FilterAllowedTools(toolCalls, req.AllowedTools)
		results := e.executeToolsWithEvents(ctx, filterResult.Allowed, parallel, w, state)
		allResults := append(results, filterResult.Rejected...)

		// Append the assistant's tool call message before results.
		provReq.Messages = append(provReq.Messages, buildAssistantToolCallMessage(toolCalls))

		// Emit tool result items as events.
		for _, r := range allResults {
			item := api.Item{
				ID:     api.NewItemID(),
				Type:   api.ItemTypeFunctionCallOutput,
				Status: api.ItemStatusCompleted,
				FunctionCallOutput: &api.FunctionCallOutputData{
					CallID: r.CallID,
					Output: r.Output,
				},
			}
			allOutputItems = append(allOutputItems, item)

			state.outputIndex++
			if err := w.WriteEvent(ctx, api.StreamEvent{
				Type: api.EventOutputItemAdded, SequenceNumber: state.nextSeq(),
				Item: &item, OutputIndex: state.outputIndex,
			}); err != nil {
				return err
			}
			if err := w.WriteEvent(ctx, api.StreamEvent{
				Type: api.EventOutputItemDone, SequenceNumber: state.nextSeq(),
				Item: &item, OutputIndex: state.outputIndex,
			}); err != nil {
				return err
			}

			// Append tool result to conversation for next turn.
			provReq.Messages = append(provReq.Messages, provider.ProviderMessage{
				Role: "tool", Content: r.Output, ToolCallID: r.CallID,
			})
		}

		// Reset stream state for next turn (keep sequence numbers).
		state.textStarted = false
		state.toolCallItems = nil
	}

	// Max turns reached.
	resp.Output = allOutputItems
	resp.Usage = streamUsage(&cumulativeUsage, req)
	resp.Status = api.ResponseStatusIncomplete
	resp.IncompleteDetails = &api.IncompleteDetails{Reason: "max_output_tokens"}
	return w.WriteEvent(ctx, api.StreamEvent{
		Type: api.EventResponseIncomplete, SequenceNumber: state.nextSeq(),
		Response: resp,
	})
}

// streamUsage returns usage for streaming responses, or nil if the client
// didn't opt in via stream_options.include_usage.
func streamUsage(usage *api.Usage, req *api.CreateResponseRequest) *api.Usage {
	if shouldIncludeStreamUsage(req) {
		return usage
	}
	return nil
}

// consumeStreamTurn reads all events from one provider.Stream turn,
// writing events to the ResponseWriter and collecting output items.
// Returns the items produced, usage (if any), and any error.
func (e *Engine) consumeStreamTurn(ctx context.Context, eventCh <-chan provider.ProviderEvent, state *streamState, w transport.ResponseWriter) ([]api.Item, *api.Usage, error) {
	var itemAdded bool
	var accumulatedText string
	var toolCallItems []api.Item
	var usage *api.Usage

	outputItem := api.Item{
		ID:     api.NewItemID(),
		Type:   api.ItemTypeMessage,
		Status: api.ItemStatusInProgress,
		Message: &api.MessageData{
			Role: api.RoleAssistant,
		},
	}
	state.itemID = outputItem.ID

	for ev := range eventCh {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}

		if ev.Type == provider.ProviderEventError {
			return nil, nil, ev.Err
		}

		// Emit output_item.added on first text content.
		if !itemAdded && (ev.Type == provider.ProviderEventTextDelta || ev.Type == provider.ProviderEventTextDone) {
			state.outputIndex++
			if err := e.emitItemLifecycleStart(ctx, &outputItem, state, w); err != nil {
				return nil, nil, err
			}
			itemAdded = true
		}

		if ev.Type == provider.ProviderEventDone {
			if ev.Usage != nil {
				usage = ev.Usage
			}

			// Build the items for this turn.
			var items []api.Item
			if itemAdded && accumulatedText != "" {
				outputItem.Status = api.ItemStatusCompleted
				outputItem.Message.Output = []api.OutputContentPart{
					{Type: "output_text", Text: accumulatedText},
				}

				// Emit text done, content_part.done, output_item.done.
				if err := w.WriteEvent(ctx, api.StreamEvent{
					Type: api.EventOutputTextDone, SequenceNumber: state.nextSeq(),
					ItemID: outputItem.ID, OutputIndex: state.outputIndex,
				}); err != nil {
					return nil, nil, err
				}
				if err := w.WriteEvent(ctx, api.StreamEvent{
					Type: api.EventContentPartDone, SequenceNumber: state.nextSeq(),
					Part:   &api.OutputContentPart{Type: "output_text", Text: accumulatedText},
					ItemID: outputItem.ID, OutputIndex: state.outputIndex,
				}); err != nil {
					return nil, nil, err
				}
				if err := w.WriteEvent(ctx, api.StreamEvent{
					Type: api.EventOutputItemDone, SequenceNumber: state.nextSeq(),
					Item: &outputItem, OutputIndex: state.outputIndex,
				}); err != nil {
					return nil, nil, err
				}

				items = append(items, outputItem)
			}

			items = append(items, toolCallItems...)
			return items, usage, nil
		}

		// Map and write events.
		streamEvents := mapProviderEvent(ev, state)
		for _, se := range streamEvents {
			if err := w.WriteEvent(ctx, se); err != nil {
				return nil, nil, err
			}

			if se.Type == api.EventOutputTextDelta {
				accumulatedText += se.Delta
			}

			if se.Type == api.EventOutputItemDone && se.Item != nil && se.Item.Type == api.ItemTypeFunctionCall {
				toolCallItems = append(toolCallItems, *se.Item)
			}
		}
	}

	// Channel closed. Build items.
	var items []api.Item
	if itemAdded && accumulatedText != "" {
		outputItem.Status = api.ItemStatusCompleted
		outputItem.Message.Output = []api.OutputContentPart{
			{Type: "output_text", Text: accumulatedText},
		}
		items = append(items, outputItem)
	}
	items = append(items, toolCallItems...)
	return items, usage, nil
}

// extractToolCalls extracts ToolCall values from function_call items.
func extractToolCalls(items []api.Item) []tools.ToolCall {
	var calls []tools.ToolCall
	for _, item := range items {
		if item.Type == api.ItemTypeFunctionCall && item.FunctionCall != nil {
			calls = append(calls, tools.ToolCall{
				ID:        item.FunctionCall.CallID,
				Name:      item.FunctionCall.Name,
				Arguments: item.FunctionCall.Arguments,
			})
		}
	}
	return calls
}

// hasUnhandledToolCalls returns true if any tool call cannot be handled
// by a registered executor.
func (e *Engine) hasUnhandledToolCalls(calls []tools.ToolCall) bool {
	for _, call := range calls {
		if e.findExecutor(call.Name) == nil {
			return true
		}
	}
	return false
}

// executeToolsConcurrently dispatches tool calls to executors in parallel
// and collects all results.
func (e *Engine) executeToolsConcurrently(ctx context.Context, calls []tools.ToolCall) []tools.ToolResult {
	if len(calls) == 0 {
		return nil
	}

	results := make([]tools.ToolResult, len(calls))
	var wg sync.WaitGroup

	for i, call := range calls {
		wg.Add(1)
		go func(idx int, tc tools.ToolCall) {
			defer wg.Done()

			exec := e.findExecutor(tc.Name)
			if exec == nil {
				results[idx] = tools.ToolResult{
					CallID:  tc.ID,
					Output:  "no executor found for tool " + tc.Name,
					IsError: true,
				}
				observability.ToolExecutionsTotal.WithLabelValues(tc.Name, "error").Inc()
				return
			}

			result, err := exec.Execute(ctx, tc)
			if err != nil {
				slog.Warn("tool execution error",
					"tool", tc.Name,
					"call_id", tc.ID,
					"error", err.Error(),
				)
				results[idx] = tools.ToolResult{
					CallID:  tc.ID,
					Output:  err.Error(),
					IsError: true,
				}
				observability.ToolExecutionsTotal.WithLabelValues(tc.Name, "error").Inc()
				return
			}

			status := "success"
			if result.IsError {
				status = "error"
			}
			observability.ToolExecutionsTotal.WithLabelValues(tc.Name, status).Inc()

			results[idx] = *result
		}(i, call)
	}

	wg.Wait()
	return results
}

// executeToolsSequentially dispatches tool calls one at a time.
// Used when parallel_tool_calls is false.
func (e *Engine) executeToolsSequentially(ctx context.Context, calls []tools.ToolCall) []tools.ToolResult {
	if len(calls) == 0 {
		return nil
	}

	results := make([]tools.ToolResult, len(calls))
	for i, tc := range calls {
		if ctx.Err() != nil {
			results[i] = tools.ToolResult{
				CallID:  tc.ID,
				Output:  "context cancelled",
				IsError: true,
			}
			continue
		}

		exec := e.findExecutor(tc.Name)
		if exec == nil {
			results[i] = tools.ToolResult{
				CallID:  tc.ID,
				Output:  "no executor found for tool " + tc.Name,
				IsError: true,
			}
			observability.ToolExecutionsTotal.WithLabelValues(tc.Name, "error").Inc()
			continue
		}

		result, err := exec.Execute(ctx, tc)
		if err != nil {
			slog.Warn("tool execution error",
				"tool", tc.Name,
				"call_id", tc.ID,
				"error", err.Error(),
			)
			results[i] = tools.ToolResult{
				CallID:  tc.ID,
				Output:  err.Error(),
				IsError: true,
			}
			observability.ToolExecutionsTotal.WithLabelValues(tc.Name, "error").Inc()
			continue
		}

		status := "success"
		if result.IsError {
			status = "error"
		}
		observability.ToolExecutionsTotal.WithLabelValues(tc.Name, status).Inc()
		results[i] = *result
	}
	return results
}

// executeTools dispatches tool calls using concurrent or sequential execution
// based on the request's parallel_tool_calls setting.
func (e *Engine) executeTools(ctx context.Context, calls []tools.ToolCall, parallel bool) []tools.ToolResult {
	if parallel {
		return e.executeToolsConcurrently(ctx, calls)
	}
	return e.executeToolsSequentially(ctx, calls)
}

// classifyToolType determines the tool lifecycle event category based on
// the executor kind and tool name.
func classifyToolType(toolName string, executor tools.ToolExecutor) string {
	if executor == nil {
		return "function"
	}
	switch executor.Kind() {
	case tools.ToolKindMCP:
		return "mcp"
	case tools.ToolKindBuiltin:
		if toolName == "file_search" {
			return "file_search"
		}
		if toolName == "web_search" {
			return "web_search"
		}
		return "function"
	default:
		return "function"
	}
}

// toolLifecycleEvents returns the in_progress, searching (if applicable),
// completed, and failed event types for a given tool classification.
func toolLifecycleEvents(toolType string) (inProgress, searching, completed, failed api.StreamEventType) {
	switch toolType {
	case "mcp":
		return api.EventMCPCallInProgress, "", api.EventMCPCallCompleted, api.EventMCPCallFailed
	case "file_search":
		return api.EventFileSearchCallInProgress, api.EventFileSearchCallSearching,
			api.EventFileSearchCallCompleted, ""
	case "web_search":
		return api.EventWebSearchCallInProgress, api.EventWebSearchCallSearching,
			api.EventWebSearchCallCompleted, ""
	default:
		return "", "", "", ""
	}
}

// executeToolsWithEvents wraps tool execution with lifecycle SSE events.
// Used only in streaming mode. Emits in_progress before execution,
// searching for search tools, and completed/failed after execution.
func (e *Engine) executeToolsWithEvents(ctx context.Context, calls []tools.ToolCall, parallel bool, w transport.ResponseWriter, state *streamState) []tools.ToolResult {
	if len(calls) == 0 {
		return nil
	}

	results := make([]tools.ToolResult, len(calls))

	execOne := func(idx int, tc tools.ToolCall) {
		exec := e.findExecutor(tc.Name)
		toolType := classifyToolType(tc.Name, exec)
		inProgress, searching, completed, failed := toolLifecycleEvents(toolType)

		// Emit in_progress event.
		if inProgress != "" {
			_ = w.WriteEvent(ctx, api.StreamEvent{
				Type:           inProgress,
				SequenceNumber: state.nextSeq(),
				ItemID:         tc.ID,
				OutputIndex:    state.outputIndex + idx + 1,
			})

			// Emit searching for search tools.
			if searching != "" {
				_ = w.WriteEvent(ctx, api.StreamEvent{
					Type:           searching,
					SequenceNumber: state.nextSeq(),
					ItemID:         tc.ID,
					OutputIndex:    state.outputIndex + idx + 1,
				})
			}
		}

		// Execute the tool.
		if exec == nil {
			results[idx] = tools.ToolResult{
				CallID:  tc.ID,
				Output:  "no executor found for tool " + tc.Name,
				IsError: true,
			}
			observability.ToolExecutionsTotal.WithLabelValues(tc.Name, "error").Inc()
			if failed != "" {
				_ = w.WriteEvent(ctx, api.StreamEvent{
					Type:           failed,
					SequenceNumber: state.nextSeq(),
					ItemID:         tc.ID,
					OutputIndex:    state.outputIndex + idx + 1,
				})
			}
			return
		}

		result, err := exec.Execute(ctx, tc)
		if err != nil {
			slog.Warn("tool execution error",
				"tool", tc.Name,
				"call_id", tc.ID,
				"error", err.Error(),
			)
			results[idx] = tools.ToolResult{
				CallID:  tc.ID,
				Output:  err.Error(),
				IsError: true,
			}
			observability.ToolExecutionsTotal.WithLabelValues(tc.Name, "error").Inc()
			if failed != "" {
				_ = w.WriteEvent(ctx, api.StreamEvent{
					Type:           failed,
					SequenceNumber: state.nextSeq(),
					ItemID:         tc.ID,
					OutputIndex:    state.outputIndex + idx + 1,
				})
			}
			return
		}

		status := "success"
		if result.IsError {
			status = "error"
		}
		observability.ToolExecutionsTotal.WithLabelValues(tc.Name, status).Inc()
		results[idx] = *result

		// Emit completed event.
		if completed != "" {
			_ = w.WriteEvent(ctx, api.StreamEvent{
				Type:           completed,
				SequenceNumber: state.nextSeq(),
				ItemID:         tc.ID,
				OutputIndex:    state.outputIndex + idx + 1,
			})
		}
	}

	if parallel {
		var wg sync.WaitGroup
		for i, call := range calls {
			wg.Add(1)
			go func(idx int, tc tools.ToolCall) {
				defer wg.Done()
				execOne(idx, tc)
			}(i, call)
		}
		wg.Wait()
	} else {
		for i, call := range calls {
			execOne(i, call)
		}
	}

	return results
}

// buildAssistantToolCallMessage creates an assistant message with tool_calls
// for the conversation history. Per Chat Completions convention, the assistant
// message containing tool_calls must precede the tool role result messages.
func buildAssistantToolCallMessage(calls []tools.ToolCall) provider.ProviderMessage {
	var toolCalls []provider.ProviderToolCall
	for _, tc := range calls {
		toolCalls = append(toolCalls, provider.ProviderToolCall{
			ID:   tc.ID,
			Type: "function",
			Function: provider.ProviderFunctionCall{
				Name:      tc.Name,
				Arguments: tc.Arguments,
			},
		})
	}
	return provider.ProviderMessage{
		Role:      "assistant",
		ToolCalls: toolCalls,
	}
}

// buildAndWriteResponse creates the final response and writes it.
func (e *Engine) buildAndWriteResponse(ctx context.Context, req *api.CreateResponseRequest, items []api.Item, usage *api.Usage, status api.ResponseStatus, respErr *api.APIError, w transport.ResponseWriter) error {
	resp := buildResponseFromRequest(req, status)
	resp.Output = items
	resp.Usage = usage
	resp.Error = respErr
	if status == api.ResponseStatusIncomplete {
		resp.IncompleteDetails = &api.IncompleteDetails{Reason: "max_output_tokens"}
	}
	return w.WriteResponse(ctx, resp)
}
