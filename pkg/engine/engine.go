package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/transport"
)

// Engine orchestrates request processing between the transport layer
// and the provider backend. It implements transport.ResponseCreator.
type Engine struct {
	provider provider.Provider
	store    transport.ResponseStore
	cfg      Config
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
		provider: p,
		store:    store,
		cfg:      cfg,
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

	// Translate the request to provider format.
	provReq := translateRequest(req)

	if req.Stream {
		return e.handleStreaming(ctx, req, provReq, w)
	}

	return e.handleNonStreaming(ctx, req, provReq, w)
}

// handleNonStreaming processes a non-streaming request.
func (e *Engine) handleNonStreaming(ctx context.Context, req *api.CreateResponseRequest, provReq *provider.ProviderRequest, w transport.ResponseWriter) error {
	provResp, err := e.provider.Complete(ctx, provReq)
	if err != nil {
		return err
	}

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
		PreviousResponseID: req.PreviousResponseID,
		CreatedAt:          time.Now().Unix(),
	}

	return w.WriteResponse(ctx, resp)
}

// handleStreaming processes a streaming request. It emits the full
// OpenResponses event lifecycle: response.created, response.in_progress,
// output_item.added, content_part.added, text deltas, text done,
// content_part.done, output_item.done, and response.completed/failed/cancelled.
func (e *Engine) handleStreaming(ctx context.Context, req *api.CreateResponseRequest, provReq *provider.ProviderRequest, w transport.ResponseWriter) error {
	// Start the provider stream.
	eventCh, err := e.provider.Stream(ctx, provReq)
	if err != nil {
		return err
	}

	// Build the initial response skeleton.
	resp := &api.Response{
		ID:                 api.NewResponseID(),
		Object:             "response",
		Status:             api.ResponseStatusInProgress,
		Model:              req.Model,
		PreviousResponseID: req.PreviousResponseID,
		CreatedAt:          time.Now().Unix(),
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

		// Emit output_item.added and content_part.added on first content event.
		if !itemAdded && (ev.Type == provider.ProviderEventTextDelta || ev.Type == provider.ProviderEventTextDone) {
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

			// Emit terminal lifecycle events.
			return e.emitStreamComplete(ctx, resp, &outputItem, accumulatedText, finalStatus, itemAdded, state, w)
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
		}
	}

	// Channel closed without a done event. Check for context cancellation.
	if ctx.Err() != nil {
		return e.emitCancelled(ctx, resp, state, w)
	}

	// Unexpected channel closure without done event.
	return e.emitStreamComplete(ctx, resp, &outputItem, accumulatedText, api.ResponseStatusCompleted, itemAdded, state, w)
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
func (e *Engine) emitStreamComplete(ctx context.Context, resp *api.Response, item *api.Item, accumulatedText string, finalStatus api.ResponseStatus, itemAdded bool, state *streamState, w transport.ResponseWriter) error {
	if itemAdded {
		// Emit content_part.done.
		if err := w.WriteEvent(ctx, api.StreamEvent{
			Type:           api.EventContentPartDone,
			SequenceNumber: state.nextSeq(),
			Part:           &api.OutputContentPart{Type: "output_text", Text: accumulatedText},
			ItemID:         item.ID,
			OutputIndex:    state.outputIndex,
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
			OutputIndex:    state.outputIndex,
		}); err != nil {
			return err
		}

		// Add to response output.
		resp.Output = []api.Item{*item}
	}

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
	if r.Output != nil {
		cp.Output = make([]api.Item, len(r.Output))
		copy(cp.Output, r.Output)
	}
	return &cp
}

// isStateful returns true if the request should be stored.
// Defaults to true unless explicitly set to false.
func isStateful(req *api.CreateResponseRequest) bool {
	if req.Store == nil {
		return true
	}
	return *req.Store
}
