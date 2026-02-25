package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/storage"
	"github.com/rhuss/antwort/pkg/transport"
)

// Adapter serves the OpenResponses API over HTTP.
// It routes requests to the appropriate handler and serializes responses.
type Adapter struct {
	creator  transport.ResponseCreator
	store    transport.ResponseStore // nil if stateless-only
	inflight *transport.InFlightRegistry
	mux      *http.ServeMux
	config   Config
}

// Config holds configuration for the HTTP adapter.
type Config struct {
	Addr            string
	MaxBodySize     int64
	ShutdownTimeout int // seconds
}

// DefaultConfig returns the default adapter configuration.
func DefaultConfig() Config {
	return Config{
		Addr:            ":8080",
		MaxBodySize:     10 << 20, // 10 MB
		ShutdownTimeout: 30,
	}
}

// NewAdapter creates an HTTP adapter with the given ResponseCreator and options.
// The ResponseStore is optional; when nil, GET and DELETE endpoints return
// an error indicating the operation is not available.
// Middleware is applied to the ResponseCreator in the given order.
func NewAdapter(creator transport.ResponseCreator, store transport.ResponseStore, cfg Config, middlewares ...transport.Middleware) *Adapter {
	// Apply middleware chain to the creator.
	if len(middlewares) > 0 {
		creator = transport.Chain(middlewares...)(creator)
	}

	a := &Adapter{
		creator:  creator,
		store:    store,
		inflight: transport.NewInFlightRegistry(),
		mux:      http.NewServeMux(),
		config:   cfg,
	}

	a.mux.HandleFunc("POST /v1/responses", a.handleCreateResponse)
	a.mux.HandleFunc("GET /v1/responses/{id}/input_items", a.handleListInputItems)
	a.mux.HandleFunc("GET /v1/responses/{id}", a.handleGetResponse)
	a.mux.HandleFunc("GET /v1/responses", a.handleListResponses)
	a.mux.HandleFunc("DELETE /v1/responses/{id}", a.handleDeleteResponse)

	return a
}

// Handler returns the http.Handler for this adapter. Use this to integrate
// with an http.Server or test with httptest. The returned handler includes
// HTTP-level middleware for request ID propagation.
func (a *Adapter) Handler() http.Handler {
	return httpRequestIDMiddleware(a.mux)
}

// httpRequestIDMiddleware is HTTP-level middleware that propagates the
// X-Request-ID header. If present in the request, it is forwarded to
// the response. After the handler runs, it checks the context for a
// request ID (set by the transport-level RequestID middleware) and adds
// it to the response headers if not already set.
func httpRequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If client sent X-Request-ID, propagate it into context.
		if id := r.Header.Get("X-Request-ID"); id != "" {
			ctx := transport.ContextWithRequestID(r.Context(), id)
			r = r.WithContext(ctx)
		}
		// Use a response writer wrapper to capture and set the request ID
		// header before the first write.
		rw := &requestIDResponseWriter{ResponseWriter: w, r: r}
		next.ServeHTTP(rw, r)
	})
}

// requestIDResponseWriter wraps http.ResponseWriter to inject the
// X-Request-ID header before the first write.
type requestIDResponseWriter struct {
	http.ResponseWriter
	r           *http.Request
	headersSent bool
}

func (w *requestIDResponseWriter) WriteHeader(statusCode int) {
	w.ensureRequestIDHeader()
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *requestIDResponseWriter) Write(b []byte) (int, error) {
	w.ensureRequestIDHeader()
	return w.ResponseWriter.Write(b)
}

func (w *requestIDResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter for http.NewResponseController.
func (w *requestIDResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *requestIDResponseWriter) ensureRequestIDHeader() {
	if w.headersSent {
		return
	}
	w.headersSent = true
	if id := transport.RequestIDFromContext(w.r.Context()); id != "" {
		w.ResponseWriter.Header().Set("X-Request-ID", id)
	}
}

// handleCreateResponse handles POST /v1/responses.
func (a *Adapter) handleCreateResponse(w http.ResponseWriter, r *http.Request) {
	// Validate Content-Type.
	ct := r.Header.Get("Content-Type")
	if ct != "" && ct != "application/json" {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("content_type", "Content-Type must be application/json"),
			http.StatusUnsupportedMediaType,
		)
		return
	}

	// Limit body size.
	r.Body = http.MaxBytesReader(w, r.Body, a.config.MaxBodySize)

	// Decode request.
	var req api.CreateResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			transport.WriteErrorResponse(w,
				api.NewInvalidRequestError("body", fmt.Sprintf("request body too large (max %d bytes)", a.config.MaxBodySize)),
				http.StatusRequestEntityTooLarge,
			)
			return
		}
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("body", "invalid JSON: "+err.Error()),
			http.StatusBadRequest,
		)
		return
	}

	if req.Stream {
		a.handleStreamingResponse(w, r, &req)
		return
	}

	// Non-streaming: create ResponseWriter and dispatch.
	rw := newSSEResponseWriter(w, nil)
	if err := a.creator.CreateResponse(r.Context(), &req, rw); err != nil {
		a.writeHandlerError(w, rw, err)
		return
	}
}

// handleStreamingResponse handles streaming POST requests (stream: true).
func (a *Adapter) handleStreamingResponse(w http.ResponseWriter, r *http.Request, req *api.CreateResponseRequest) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	var registeredID string
	rw := newSSEResponseWriter(w, func(id string) {
		registeredID = id
		a.inflight.Register(id, cancel)
	})

	err := a.creator.CreateResponse(ctx, req, rw)

	// Clean up in-flight registry after completion.
	if registeredID != "" {
		a.inflight.Remove(registeredID)
	}

	if err != nil {
		a.writeHandlerError(w, rw, err)
	}
}

// handleGetResponse handles GET /v1/responses/{id}.
func (a *Adapter) handleGetResponse(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("", "response retrieval is not available (no store configured)"),
			http.StatusNotImplemented,
		)
		return
	}

	id := r.PathValue("id")
	if !api.ValidateResponseID(id) {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("id", "malformed response ID"),
			http.StatusBadRequest,
		)
		return
	}

	resp, err := a.store.GetResponse(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			transport.WriteAPIError(w, api.NewNotFoundError("response "+id+" not found"))
		} else {
			var apiErr *api.APIError
			if errors.As(err, &apiErr) {
				transport.WriteAPIError(w, apiErr)
			} else {
				transport.WriteAPIError(w, api.NewServerError(err.Error()))
			}
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleDeleteResponse handles DELETE /v1/responses/{id}.
// It first checks the in-flight registry (for cancelling active streams),
// then falls through to the response store for standard deletion.
func (a *Adapter) handleDeleteResponse(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !api.ValidateResponseID(id) {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("id", "malformed response ID"),
			http.StatusBadRequest,
		)
		return
	}

	// Check in-flight registry first.
	if a.inflight.Cancel(id) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Fall through to store.
	if a.store == nil {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("", "response deletion is not available (no store configured)"),
			http.StatusNotImplemented,
		)
		return
	}

	if err := a.store.DeleteResponse(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			transport.WriteAPIError(w, api.NewNotFoundError("response "+id+" not found"))
		} else {
			var apiErr *api.APIError
			if errors.As(err, &apiErr) {
				transport.WriteAPIError(w, apiErr)
			} else {
				transport.WriteAPIError(w, api.NewServerError(err.Error()))
			}
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListResponses handles GET /v1/responses.
func (a *Adapter) handleListResponses(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("", "response listing is not available (no store configured)"),
			http.StatusNotImplemented,
		)
		return
	}

	opts, err := parseListOptions(r)
	if err != nil {
		transport.WriteErrorResponse(w, err, http.StatusBadRequest)
		return
	}

	result, storeErr := a.store.ListResponses(r.Context(), opts)
	if storeErr != nil {
		var apiErr *api.APIError
		if errors.As(storeErr, &apiErr) {
			transport.WriteAPIError(w, apiErr)
		} else {
			transport.WriteAPIError(w, api.NewServerError(storeErr.Error()))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleListInputItems handles GET /v1/responses/{id}/input_items.
func (a *Adapter) handleListInputItems(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("", "input items retrieval is not available (no store configured)"),
			http.StatusNotImplemented,
		)
		return
	}

	id := r.PathValue("id")
	if !api.ValidateResponseID(id) {
		transport.WriteErrorResponse(w,
			api.NewInvalidRequestError("id", "malformed response ID"),
			http.StatusBadRequest,
		)
		return
	}

	opts, err := parseListOptions(r)
	if err != nil {
		transport.WriteErrorResponse(w, err, http.StatusBadRequest)
		return
	}

	result, storeErr := a.store.GetInputItems(r.Context(), id, opts)
	if storeErr != nil {
		if errors.Is(storeErr, storage.ErrNotFound) {
			transport.WriteAPIError(w, api.NewNotFoundError("response "+id+" not found"))
		} else {
			var apiErr *api.APIError
			if errors.As(storeErr, &apiErr) {
				transport.WriteAPIError(w, apiErr)
			} else {
				transport.WriteAPIError(w, api.NewServerError(storeErr.Error()))
			}
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// parseListOptions extracts pagination parameters from query string.
func parseListOptions(r *http.Request) (transport.ListOptions, *api.APIError) {
	q := r.URL.Query()
	opts := transport.ListOptions{
		After:  q.Get("after"),
		Before: q.Get("before"),
		Model:  q.Get("model"),
		Order:  q.Get("order"),
	}

	if opts.After != "" && opts.Before != "" {
		return opts, api.NewInvalidRequestError("after", "cannot use both 'after' and 'before' cursors")
	}

	if opts.Order != "" && opts.Order != "asc" && opts.Order != "desc" {
		return opts, api.NewInvalidRequestError("order", "order must be 'asc' or 'desc'")
	}
	if opts.Order == "" {
		opts.Order = "desc"
	}

	if limitStr := q.Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			return opts, api.NewInvalidRequestError("limit", "limit must be a positive integer")
		}
		opts.Limit = limit
	}

	return opts, nil
}

// writeHandlerError writes an error response from the handler. If streaming
// has already started, it sends a response.failed event. Otherwise it writes
// a standard JSON error response.
func (a *Adapter) writeHandlerError(w http.ResponseWriter, rw *sseResponseWriter, err error) {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		apiErr = api.NewServerError(err.Error())
	}

	if rw.hasStartedStreaming() {
		// Streaming has begun; send response.failed event.
		failEvent := api.StreamEvent{
			Type: api.EventResponseFailed,
			Response: &api.Response{
				Status: api.ResponseStatusFailed,
				Error:  apiErr,
			},
		}
		rw.WriteEvent(context.Background(), failEvent)
		return
	}

	// No streaming started; return JSON error.
	transport.WriteAPIError(w, apiErr)
}
