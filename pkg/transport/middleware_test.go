package transport

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
)

// recordingWriter is a minimal ResponseWriter for testing middleware.
type recordingWriter struct {
	events    []api.StreamEvent
	response  *api.Response
	flushed   bool
}

func (w *recordingWriter) WriteEvent(_ context.Context, event api.StreamEvent) error {
	w.events = append(w.events, event)
	return nil
}

func (w *recordingWriter) WriteResponse(_ context.Context, resp *api.Response) error {
	w.response = resp
	return nil
}

func (w *recordingWriter) Flush() error {
	w.flushed = true
	return nil
}

func TestChainAppliesMiddlewareInOrder(t *testing.T) {
	var order []string

	mw := func(name string) Middleware {
		return func(next ResponseCreator) ResponseCreator {
			return ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
				order = append(order, name+":before")
				err := next.CreateResponse(ctx, req, w)
				order = append(order, name+":after")
				return err
			})
		}
	}

	handler := ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
		order = append(order, "handler")
		return nil
	})

	chain := Chain(mw("first"), mw("second"), mw("third"))
	wrapped := chain(handler)

	wrapped.CreateResponse(context.Background(), &api.CreateResponseRequest{}, &recordingWriter{})

	expected := []string{
		"first:before", "second:before", "third:before",
		"handler",
		"third:after", "second:after", "first:after",
	}

	if len(order) != len(expected) {
		t.Fatalf("execution order length = %d, want %d: %v", len(order), len(expected), order)
	}
	for i, got := range order {
		if got != expected[i] {
			t.Errorf("order[%d] = %q, want %q", i, got, expected[i])
		}
	}
}

func TestRecoveryCatchesPanic(t *testing.T) {
	handler := ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
		panic("test panic")
	})

	wrapped := Recovery()(handler)
	err := wrapped.CreateResponse(context.Background(), &api.CreateResponseRequest{}, &recordingWriter{})

	if err == nil {
		t.Fatal("expected error after panic, got nil")
	}

	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *api.APIError, got %T: %v", err, err)
	}
	if apiErr.Type != api.ErrorTypeServerError {
		t.Errorf("error type = %q, want %q", apiErr.Type, api.ErrorTypeServerError)
	}
	if !strings.Contains(apiErr.Message, "test panic") {
		t.Errorf("error message = %q, should contain %q", apiErr.Message, "test panic")
	}
}

func TestRecoveryPassesThroughNormalExecution(t *testing.T) {
	handler := ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
		return nil
	})

	wrapped := Recovery()(handler)
	err := wrapped.CreateResponse(context.Background(), &api.CreateResponseRequest{}, &recordingWriter{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestIDGeneratesNewID(t *testing.T) {
	var capturedID string

	handler := ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
		capturedID = RequestIDFromContext(ctx)
		return nil
	})

	wrapped := RequestID()(handler)
	wrapped.CreateResponse(context.Background(), &api.CreateResponseRequest{}, &recordingWriter{})

	if capturedID == "" {
		t.Error("expected a generated request ID, got empty string")
	}
	if len(capturedID) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("request ID length = %d, want 32 (hex encoded)", len(capturedID))
	}
}

func TestRequestIDPropagatesExisting(t *testing.T) {
	var capturedID string

	handler := ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
		capturedID = RequestIDFromContext(ctx)
		return nil
	})

	ctx := ContextWithRequestID(context.Background(), "existing-id-123")
	wrapped := RequestID()(handler)
	wrapped.CreateResponse(ctx, &api.CreateResponseRequest{}, &recordingWriter{})

	if capturedID != "existing-id-123" {
		t.Errorf("request ID = %q, want %q", capturedID, "existing-id-123")
	}
}

func TestRequestIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	handler := ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
		ids[RequestIDFromContext(ctx)] = true
		return nil
	})

	wrapped := RequestID()(handler)
	for i := 0; i < 100; i++ {
		wrapped.CreateResponse(context.Background(), &api.CreateResponseRequest{}, &recordingWriter{})
	}

	if len(ids) != 100 {
		t.Errorf("expected 100 unique IDs, got %d", len(ids))
	}
}

func TestLoggingEmitsFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
		return nil
	})

	ctx := ContextWithRequestID(context.Background(), "req-log-test")
	wrapped := Logging(logger)(handler)
	wrapped.CreateResponse(ctx, &api.CreateResponseRequest{Model: "test-model", Stream: true}, &recordingWriter{})

	output := buf.String()
	for _, expected := range []string{"request_id=req-log-test", "model=test-model", "stream=true", "request completed"} {
		if !strings.Contains(output, expected) {
			t.Errorf("log output missing %q in:\n%s", expected, output)
		}
	}
}

func TestLoggingEmitsErrorOnFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handler := ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
		return api.NewServerError("test failure")
	})

	wrapped := Logging(logger)(handler)
	wrapped.CreateResponse(context.Background(), &api.CreateResponseRequest{Model: "test"}, &recordingWriter{})

	output := buf.String()
	if !strings.Contains(output, "request failed") {
		t.Errorf("log output missing 'request failed' in:\n%s", output)
	}
	if !strings.Contains(output, "test failure") {
		t.Errorf("log output missing error message in:\n%s", output)
	}
}
