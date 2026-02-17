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
// For now, only the non-streaming path is implemented. Streaming is deferred
// to Phase 4.
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
		// Streaming path deferred to Phase 4.
		return api.NewInvalidRequestError("stream", "streaming not yet implemented")
	}

	// Non-streaming path: call provider.Complete.
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

	// Store the response if a store is available and the request is stateful.
	// NOTE: The ResponseStore interface currently only defines Get/Delete.
	// A Save/Put method will be added in the persistence spec. For now,
	// storage is a no-op.

	// Write the response.
	return w.WriteResponse(ctx, resp)
}

// isStateful returns true if the request should be stored.
// Defaults to true unless explicitly set to false.
func isStateful(req *api.CreateResponseRequest) bool {
	if req.Store == nil {
		return true
	}
	return *req.Store
}
