package transport

import (
	"context"

	"github.com/rhuss/antwort/pkg/api"
)

// ResponseCreator handles the core create-response operation.
// It is the primary handler contract, available in both stateless and
// stateful deployments. The implementation receives a request and writes
// the result (streaming events or a complete response) to the ResponseWriter.
type ResponseCreator interface {
	CreateResponse(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error
}

// ResponseCreatorFunc is an adapter that allows using an ordinary function
// as a ResponseCreator.
type ResponseCreatorFunc func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error

// CreateResponse calls f(ctx, req, w).
func (f ResponseCreatorFunc) CreateResponse(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
	return f(ctx, req, w)
}

// ResponseStore handles retrieval and deletion of stored responses.
// It is only available in stateful deployments with persistence configured.
type ResponseStore interface {
	GetResponse(ctx context.Context, id string) (*api.Response, error)
	DeleteResponse(ctx context.Context, id string) error
}

// ResponseWriter abstracts streaming and non-streaming output for the handler.
// The transport layer creates a ResponseWriter for each request and provides
// it to the handler. The handler uses WriteEvent for streaming responses or
// WriteResponse for non-streaming responses.
//
// WriteEvent and WriteResponse are mutually exclusive on a single writer
// instance. Calling WriteEvent after WriteResponse (or vice versa) returns
// an error. Calling WriteEvent after a terminal event (response.completed,
// response.failed, or response.cancelled) also returns an error.
type ResponseWriter interface {
	// WriteEvent sends a single streaming event. Returns an error if called
	// after a terminal event has been sent or after WriteResponse was called.
	WriteEvent(ctx context.Context, event api.StreamEvent) error

	// WriteResponse sends a complete non-streaming response. Returns an error
	// if called after WriteEvent was called on this writer.
	WriteResponse(ctx context.Context, resp *api.Response) error

	// Flush ensures buffered data is sent to the client. Returns an error
	// if the client has disconnected.
	Flush() error
}
