package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/observability"
	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/tools"
	mcptools "github.com/rhuss/antwort/pkg/tools/mcp"
	"github.com/rhuss/antwort/pkg/transport"
)

// Engine orchestrates request processing between the transport layer
// and the provider backend. It implements transport.ResponseCreator.
type Engine struct {
	provider  provider.Provider
	store     transport.ResponseStore
	executors []tools.ToolExecutor
	cfg       Config
}

// Ensure Engine implements transport.ResponseCreator at compile time.
var _ transport.ResponseCreator = (*Engine)(nil)

// New creates a new Engine. The provider must not be nil. The store
// can be nil for stateless operation.
func New(p provider.Provider, store transport.ResponseStore, cfg Config) (*Engine, error) {
	if p == nil {
		return nil, fmt.Errorf("engine: provider must not be nil")
	}
	return &Engine{
		provider:  p,
		store:     store,
		executors: cfg.Executors,
		cfg:       cfg,
	}, nil
}

// CreateResponse handles a non-streaming or streaming response creation request.
func (e *Engine) CreateResponse(ctx context.Context, req *api.CreateResponseRequest, w transport.ResponseWriter) error {
	// Apply default model if the request omits it.
	if req.Model == "" {
		if e.cfg.DefaultModel != "" {
			req.Model = e.cfg.DefaultModel
		} else {
			return api.NewInvalidRequestError("model", "model is required")
		}
	}

	// Validate capabilities.
	if apiErr := provider.ValidateCapabilities(e.provider.Capabilities(), req); apiErr != nil {
		return apiErr
	}

	// Merge MCP-discovered tools into the request before translation.
	e.mergeMCPTools(ctx, req)

	// Translate the request to provider format.
	provReq := translateRequest(req)

	// If previous_response_id is set, reconstruct conversation history.
	if req.PreviousResponseID != "" {
		historyMsgs, err := loadConversationHistory(ctx, e.store, req.PreviousResponseID)
		if err != nil {
			return err
		}
		// Prepend history messages before the current request's messages.
		provReq.Messages = append(historyMsgs, provReq.Messages...)
	}

	// Determine if the agentic loop should be used:
	// - Executors are registered
	// - Tools are present in the request
	// - tool_choice is not "none"
	useLoop := e.hasExecutors() && len(req.Tools) > 0 &&
		!(req.ToolChoice != nil && req.ToolChoice.String == "none")

	if req.Stream {
		if useLoop {
			return e.runAgenticLoopStreaming(ctx, req, provReq, w)
		}
		return e.handleStreaming(ctx, req, provReq, w)
	}

	if useLoop {
		return e.runAgenticLoop(ctx, req, provReq, w)
	}
	return e.handleNonStreaming(ctx, req, provReq, w)
}

// handleNonStreaming processes a non-streaming request.
func (e *Engine) handleNonStreaming(ctx context.Context, req *api.CreateResponseRequest, provReq *provider.ProviderRequest, w transport.ResponseWriter) error {
	startTime := time.Now()
	provResp, err := e.provider.Complete(ctx, provReq)
	duration := time.Since(startTime)

	provName := e.provider.Name()
	if err != nil {
		observability.ProviderRequestsTotal.WithLabelValues(provName, req.Model, "error").Inc()
		observability.ProviderLatency.WithLabelValues(provName, req.Model).Observe(duration.Seconds())
		return err
	}

	observability.ProviderRequestsTotal.WithLabelValues(provName, req.Model, "success").Inc()
	observability.ProviderLatency.WithLabelValues(provName, req.Model).Observe(duration.Seconds())
	observability.ProviderTokensTotal.WithLabelValues(provName, req.Model, "input").Add(float64(provResp.Usage.InputTokens))
	observability.ProviderTokensTotal.WithLabelValues(provName, req.Model, "output").Add(float64(provResp.Usage.OutputTokens))
	observability.RecordGenAIMetrics(provName, req.Model, duration, provResp.Usage.InputTokens, provResp.Usage.OutputTokens, nil)

	// Check for empty output (backend returned no choices).
	if provResp.Status == api.ResponseStatusFailed && len(provResp.Items) == 0 {
		return api.NewServerError("backend produced no output")
	}

	// Assign item IDs to output items if they don't have them.
	for i := range provResp.Items {
		if provResp.Items[i].ID == "" {
			provResp.Items[i].ID = api.NewItemID()
		}
	}

	// Build the API response.
	resp := &api.Response{
		ID:                 api.NewResponseID(),
		Object:             "response",
		Status:             provResp.Status,
		Output:             provResp.Items,
		Model:              provResp.Model,
		Usage:              &provResp.Usage,
		PreviousResponseID: stringPtr(req.PreviousResponseID),
		CreatedAt:          time.Now().Unix(),
		Tools:              ensureTools(req.Tools),
		ToolChoice:         toolChoiceValue(req.ToolChoice),
		Truncation:         getTruncation(req),
		Store:              isStateful(req),
		Text:               &api.TextConfig{Format: &api.TextFormat{Type: "text"}},
		ServiceTier:        getServiceTier(req),
		Metadata:           make(map[string]any),
		Temperature:        derefFloat64(req.Temperature),
		TopP:               derefFloat64(req.TopP),
		MaxOutputTokens:    req.MaxOutputTokens,
	}

	// Write the response to the client first.
	if err := w.WriteResponse(ctx, resp); err != nil {
		return err
	}

	// Save the response to the store (after client write).
	e.saveIfStateful(ctx, req, resp)

	return nil
}

// handleStreaming processes a streaming request. It emits the full
// OpenResponses event lifecycle: response.created, response.in_progress,
// output_item.added, content_part.added, text deltas, text done,
// content_part.done, output_item.done, and response.completed/failed/cancelled.
func (e *Engine) handleStreaming(ctx context.Context, req *api.CreateResponseRequest, provReq *provider.ProviderRequest, w transport.ResponseWriter) error {
	// Start the provider stream.
	streamStart := time.Now()
	eventCh, err := e.provider.Stream(ctx, provReq)
	if err != nil {
		provName := e.provider.Name()
		observability.ProviderRequestsTotal.WithLabelValues(provName, req.Model, "error").Inc()
		observability.ProviderLatency.WithLabelValues(provName, req.Model).Observe(time.Since(streamStart).Seconds())
		return err
	}

	var firstTokenTime *time.Duration

	// Build the initial response skeleton.
	resp := &api.Response{
		ID:                 api.NewResponseID(),
		Object:             "response",
		Status:             api.ResponseStatusInProgress,
		Output:             []api.Item{},
		Model:              req.Model,
		PreviousResponseID: stringPtr(req.PreviousResponseID),
		CreatedAt:          time.Now().Unix(),
		Tools:              ensureTools(req.Tools),
		ToolChoice:         toolChoiceValue(req.ToolChoice),
		Truncation:         getTruncation(req),
		Store:              isStateful(req),
		Text:               &api.TextConfig{Format: &api.TextFormat{Type: "text"}},
		ServiceTier:        getServiceTier(req),
		Metadata:           make(map[string]any),
		Temperature:        derefFloat64(req.Temperature),
		TopP:               derefFloat64(req.TopP),
		MaxOutputTokens:    req.MaxOutputTokens,
	}

	// Initialize the stream state for event mapping.
	state := &streamState{}

	// Emit response.created (snapshot the response so later mutations
	// don't affect this event's payload).
	if err := w.WriteEvent(ctx, api.StreamEvent{
		Type:           api.EventResponseCreated,
		SequenceNumber: state.nextSeq(),
		Response:       snapshotResponse(resp),
	}); err != nil {
		return err
	}

	// Emit response.in_progress.
	if err := w.WriteEvent(ctx, api.StreamEvent{
		Type:           api.EventResponseInProgress,
		SequenceNumber: state.nextSeq(),
		Response:       snapshotResponse(resp),
	}); err != nil {
		return err
	}

	// Track whether we've emitted the output item lifecycle events.
	itemAdded := false
	var accumulatedText string
	var toolCallItems []api.Item

	// Create the output item that will be populated during streaming.
	outputItem := api.Item{
		ID:     api.NewItemID(),
		Type:   api.ItemTypeMessage,
		Status: api.ItemStatusInProgress,
		Message: &api.MessageData{
			Role: api.RoleAssistant,
		},
	}
	state.itemID = outputItem.ID
	state.outputIndex = 0

	// Consume events from the provider channel.
	for ev := range eventCh {
		// Check for context cancellation.
		if ctx.Err() != nil {
			return e.emitCancelled(ctx, resp, state, w)
		}

		// Handle error events.
		if ev.Type == provider.ProviderEventError {
			return e.emitFailed(ctx, resp, ev.Err, state, w)
		}

		// Emit output_item.added and content_part.added on first text content event.
		if !itemAdded && (ev.Type == provider.ProviderEventTextDelta || ev.Type == provider.ProviderEventTextDone) {
			if firstTokenTime == nil {
				ttft := time.Since(streamStart)
				firstTokenTime = &ttft
			}
			if err := e.emitItemLifecycleStart(ctx, &outputItem, state, w); err != nil {
				return err
			}
			itemAdded = true
		}

		// Handle the done event from the provider.
		if ev.Type == provider.ProviderEventDone {
			// Determine final response status from the event.
			finalStatus := api.ResponseStatusCompleted
			if ev.Item != nil {
				switch ev.Item.Status {
				case api.ItemStatusIncomplete:
					finalStatus = api.ResponseStatusIncomplete
				case api.ItemStatusFailed:
					finalStatus = api.ResponseStatusFailed
				}
			}

			// Update usage if provided.
			if ev.Usage != nil {
				resp.Usage = ev.Usage
			}

			// Record provider metrics for the streaming call.
			duration := time.Since(streamStart)
			provName := e.provider.Name()
			observability.ProviderRequestsTotal.WithLabelValues(provName, req.Model, "success").Inc()
			observability.ProviderLatency.WithLabelValues(provName, req.Model).Observe(duration.Seconds())
			if ev.Usage != nil {
				observability.ProviderTokensTotal.WithLabelValues(provName, req.Model, "input").Add(float64(ev.Usage.InputTokens))
				observability.ProviderTokensTotal.WithLabelValues(provName, req.Model, "output").Add(float64(ev.Usage.OutputTokens))
				observability.RecordGenAIMetrics(provName, req.Model, duration, ev.Usage.InputTokens, ev.Usage.OutputTokens, firstTokenTime)
			} else {
				observability.RecordGenAIMetrics(provName, req.Model, duration, 0, 0, firstTokenTime)
			}

			// Emit terminal lifecycle events.
			return e.emitStreamComplete(ctx, resp, &outputItem, accumulatedText, finalStatus, itemAdded, toolCallItems, state, w)
		}

		// Map provider event to stream events and write them.
		streamEvents := mapProviderEvent(ev, state)
		for _, se := range streamEvents {
			if err := w.WriteEvent(ctx, se); err != nil {
				return err
			}

			// Accumulate text for the final output item.
			if se.Type == api.EventOutputTextDelta {
				accumulatedText += se.Delta
			}

			// Collect completed tool call items for the final response.
			if se.Type == api.EventOutputItemDone && se.Item != nil && se.Item.Type == api.ItemTypeFunctionCall {
				toolCallItems = append(toolCallItems, *se.Item)
			}
		}
	}

	// Channel closed without a done event. Check for context cancellation.
	if ctx.Err() != nil {
		return e.emitCancelled(ctx, resp, state, w)
	}

	// Unexpected channel closure without done event.
	return e.emitStreamComplete(ctx, resp, &outputItem, accumulatedText, api.ResponseStatusCompleted, itemAdded, toolCallItems, state, w)
}

// emitItemLifecycleStart emits the output_item.added and content_part.added events.
func (e *Engine) emitItemLifecycleStart(ctx context.Context, item *api.Item, state *streamState, w transport.ResponseWriter) error {
	// Emit output_item.added.
	if err := w.WriteEvent(ctx, api.StreamEvent{
		Type:           api.EventOutputItemAdded,
		SequenceNumber: state.nextSeq(),
		Item:           item,
		OutputIndex:    state.outputIndex,
	}); err != nil {
		return err
	}

	// Emit content_part.added.
	return w.WriteEvent(ctx, api.StreamEvent{
		Type:           api.EventContentPartAdded,
		SequenceNumber: state.nextSeq(),
		Part:           &api.OutputContentPart{Type: "output_text", Text: ""},
		ItemID:         item.ID,
		OutputIndex:    state.outputIndex,
		ContentIndex:   0,
	})
}

// emitStreamComplete emits the terminal lifecycle events for a completed stream.
func (e *Engine) emitStreamComplete(ctx context.Context, resp *api.Response, item *api.Item, accumulatedText string, finalStatus api.ResponseStatus, itemAdded bool, toolCallItems []api.Item, state *streamState, w transport.ResponseWriter) error {
	var outputItems []api.Item

	if itemAdded {
		// Emit content_part.done.
		if err := w.WriteEvent(ctx, api.StreamEvent{
			Type:           api.EventContentPartDone,
			SequenceNumber: state.nextSeq(),
			Part:           &api.OutputContentPart{Type: "output_text", Text: accumulatedText},
			ItemID:         item.ID,
			OutputIndex:    0, // Text item is always at output index 0.
			ContentIndex:   0,
		}); err != nil {
			return err
		}

		// Finalize the output item.
		item.Status = api.ItemStatusCompleted
		if finalStatus == api.ResponseStatusIncomplete {
			item.Status = api.ItemStatusIncomplete
		}
		item.Message.Output = []api.OutputContentPart{
			{Type: "output_text", Text: accumulatedText},
		}

		// Emit output_item.done.
		if err := w.WriteEvent(ctx, api.StreamEvent{
			Type:           api.EventOutputItemDone,
			SequenceNumber: state.nextSeq(),
			Item:           item,
			OutputIndex:    0,
		}); err != nil {
			return err
		}

		outputItems = append(outputItems, *item)
	}

	// Include tool call items in the response output.
	outputItems = append(outputItems, toolCallItems...)
	resp.Output = outputItems

	// Update response status.
	resp.Status = finalStatus

	// Emit response.completed (or other terminal event).
	eventType := api.EventResponseCompleted
	if finalStatus == api.ResponseStatusFailed {
		eventType = api.EventResponseFailed
	}

	return w.WriteEvent(ctx, api.StreamEvent{
		Type:           eventType,
		SequenceNumber: state.nextSeq(),
		Response:       resp,
	})
}

// emitFailed emits a response.failed event.
func (e *Engine) emitFailed(ctx context.Context, resp *api.Response, streamErr error, state *streamState, w transport.ResponseWriter) error {
	resp.Status = api.ResponseStatusFailed
	if apiErr, ok := streamErr.(*api.APIError); ok {
		resp.Error = apiErr
	} else if streamErr != nil {
		resp.Error = api.NewServerError(streamErr.Error())
	}

	return w.WriteEvent(ctx, api.StreamEvent{
		Type:           api.EventResponseFailed,
		SequenceNumber: state.nextSeq(),
		Response:       resp,
	})
}

// emitCancelled emits a response.cancelled event.
func (e *Engine) emitCancelled(ctx context.Context, resp *api.Response, state *streamState, w transport.ResponseWriter) error {
	resp.Status = api.ResponseStatusCancelled

	// Use a background context for writing the cancellation event,
	// since the original context is already cancelled.
	return w.WriteEvent(context.Background(), api.StreamEvent{
		Type:           api.EventResponseCancelled,
		SequenceNumber: state.nextSeq(),
		Response:       resp,
	})
}

// snapshotResponse creates a shallow copy of a Response so that later
// mutations to the original don't affect event payloads already written.
func snapshotResponse(r *api.Response) *api.Response {
	cp := *r
	// Copy output slice to avoid shared backing array.
	// Always ensure non-nil (serializes as [] not null).
	if r.Output != nil {
		cp.Output = make([]api.Item, len(r.Output))
		copy(cp.Output, r.Output)
	} else {
		cp.Output = []api.Item{}
	}
	return &cp
}

// mergeMCPTools discovers tools from MCP executors and merges them into
// the request's tool list. Explicit tools in the request take precedence
// over MCP-discovered tools with the same name.
func (e *Engine) mergeMCPTools(ctx context.Context, req *api.CreateResponseRequest) {
	for _, exec := range e.executors {
		if mcpExec, ok := exec.(*mcptools.MCPExecutor); ok {
			discovered := mcpExec.DiscoveredTools()
			if len(discovered) == 0 {
				// Trigger lazy discovery.
				mcpExec.CanExecute("__trigger_discovery__")
				discovered = mcpExec.DiscoveredTools()
			}

			// Build a set of existing tool names for dedup.
			existing := make(map[string]bool, len(req.Tools))
			for _, t := range req.Tools {
				existing[t.Name] = true
			}

			// Merge discovered tools that don't conflict.
			for _, t := range discovered {
				if !existing[t.Name] {
					req.Tools = append(req.Tools, t)
				}
			}
		}
	}
}

// hasExecutors returns true if any tool executors are registered.
func (e *Engine) hasExecutors() bool {
	return len(e.executors) > 0
}

// findExecutor returns the first executor that can handle the given tool name.
// Returns nil if no executor matches.
func (e *Engine) findExecutor(toolName string) tools.ToolExecutor {
	for _, exec := range e.executors {
		if exec.CanExecute(toolName) {
			return exec
		}
	}
	return nil
}

// saveIfStateful saves the response to the store if conditions are met:
// store is configured, request has store=true (default), and the response
// has input items populated. Save failures are logged but do not affect
// the client response.
func (e *Engine) saveIfStateful(ctx context.Context, req *api.CreateResponseRequest, resp *api.Response) {
	if e.store == nil || !isStateful(req) {
		return
	}

	// Populate input items from the request for full conversation reconstruction.
	if resp.Input == nil {
		resp.Input = req.Input
	}

	if err := e.store.SaveResponse(ctx, resp); err != nil {
		slog.Warn("failed to save response to store",
			"response_id", resp.ID,
			"error", err.Error(),
		)
	}
}

// isStateful returns true if the request should be stored.
// Defaults to true unless explicitly set to false.
func isStateful(req *api.CreateResponseRequest) bool {
	if req.Store == nil {
		return true
	}
	return *req.Store
}

// stringPtr converts a string to a pointer to string.
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// getTruncation returns the truncation setting from the request, defaulting to "disabled".
func getTruncation(req *api.CreateResponseRequest) string {
	if req.Truncation != "" {
		return req.Truncation
	}
	return "disabled"
}

// getServiceTier returns the service tier from the request, defaulting to "default".
func getServiceTier(req *api.CreateResponseRequest) string {
	if req.ServiceTier != "" {
		return req.ServiceTier
	}
	return "default"
}

// derefFloat64 returns the value of a *float64, or 0.0 if nil.
func derefFloat64(p *float64) float64 {
	if p == nil {
		return 0.0
	}
	return *p
}

// toolChoiceValue converts a *ToolChoice to a serializable value for the response.
// Returns "auto" as default when nil.
func toolChoiceValue(tc *api.ToolChoice) any {
	if tc == nil {
		return "auto"
	}
	if tc.String != "" {
		return tc.String
	}
	if tc.Function != nil {
		return tc.Function
	}
	return "auto"
}

// ensureTools returns the tools slice, defaulting to an empty slice (not nil)
// so it serializes as [] instead of null.
func ensureTools(tools []api.ToolDefinition) []api.ToolDefinition {
	if tools == nil {
		return []api.ToolDefinition{}
	}
	return tools
}
