package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/transport"
)

// writerState tracks the state of an SSE ResponseWriter.
type writerState int

const (
	writerIdle      writerState = iota // Initial state, no writes yet
	writerStreaming                    // WriteEvent has been called at least once
	writerCompleted                   // Terminal event sent or WriteResponse called
)

// terminalEvents are the event types that end a streaming response.
var terminalEvents = map[api.StreamEventType]bool{
	api.EventResponseCompleted:      true,
	api.EventResponseFailed:         true,
	api.EventResponseCancelled:      true,
	api.EventResponseRequiresAction: true,
	api.EventResponseIncomplete:     true,
	api.EventError:                  true,
}

// sseResponseWriter implements transport.ResponseWriter for HTTP/SSE responses.
// It handles both streaming (SSE) and non-streaming (JSON) output.
type sseResponseWriter struct {
	w  http.ResponseWriter
	rc *http.ResponseController

	mu    sync.Mutex
	state writerState

	// onResponseCreated is called when the first response.created event is
	// written, providing the response ID for in-flight registry registration.
	onResponseCreated func(id string)
}

var _ transport.ResponseWriter = (*sseResponseWriter)(nil)

// newSSEResponseWriter creates a new ResponseWriter wrapping an http.ResponseWriter.
// The onCreated callback is called with the response ID when the first
// response.created event is written (may be nil if not needed).
func newSSEResponseWriter(w http.ResponseWriter, onCreated func(id string)) *sseResponseWriter {
	return &sseResponseWriter{
		w:                 w,
		rc:                http.NewResponseController(w),
		onResponseCreated: onCreated,
	}
}

// WriteEvent sends a single SSE event. The event is formatted as:
//
//	event: {type}\n
//	data: {json}\n
//	\n
//
// After a terminal event, it also sends:
//
//	data: [DONE]\n
//	\n
func (s *sseResponseWriter) WriteEvent(ctx context.Context, event api.StreamEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == writerCompleted {
		return errors.New("cannot write event: writer is completed")
	}

	// First event: set SSE headers.
	if s.state == writerIdle {
		s.w.Header().Set("Content-Type", "text/event-stream")
		s.w.Header().Set("Cache-Control", "no-cache")
		s.w.Header().Set("Connection", "keep-alive")
		s.state = writerStreaming
	}

	// Intercept response.created to extract the response ID.
	if event.Type == api.EventResponseCreated && event.Response != nil && s.onResponseCreated != nil {
		s.onResponseCreated(event.Response.ID)
		s.onResponseCreated = nil // Only call once.
	}

	// Serialize the event as JSON.
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Write SSE format.
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event.Type, data); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	// Flush immediately.
	if err := s.rc.Flush(); err != nil {
		return fmt.Errorf("failed to flush: %w", err)
	}

	// If this was a terminal event, send [DONE] and mark completed.
	if terminalEvents[event.Type] {
		if _, err := fmt.Fprint(s.w, "data: [DONE]\n\n"); err != nil {
			return fmt.Errorf("failed to write [DONE]: %w", err)
		}
		if err := s.rc.Flush(); err != nil {
			return fmt.Errorf("failed to flush [DONE]: %w", err)
		}
		s.state = writerCompleted
	}

	return nil
}

// WriteResponse sends a complete non-streaming JSON response.
// This is mutually exclusive with WriteEvent.
func (s *sseResponseWriter) WriteResponse(ctx context.Context, resp *api.Response) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == writerStreaming {
		return errors.New("cannot write response: streaming has already started")
	}
	if s.state == writerCompleted {
		return errors.New("cannot write response: writer is completed")
	}

	s.w.Header().Set("Content-Type", "application/json")
	s.state = writerCompleted

	if err := json.NewEncoder(s.w).Encode(resp); err != nil {
		return fmt.Errorf("failed to encode response: %w", err)
	}

	return nil
}

// Flush ensures buffered data is sent to the client.
func (s *sseResponseWriter) Flush() error {
	return s.rc.Flush()
}

// hasStartedStreaming returns true if at least one SSE event has been written.
func (s *sseResponseWriter) hasStartedStreaming() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == writerStreaming || (s.state == writerCompleted && s.w.Header().Get("Content-Type") == "text/event-stream")
}
