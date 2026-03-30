package resilience

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
)

// timeoutError implements net.Error with Timeout() == true.
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Classification
	}{
		{
			name: "nil error",
			err:  nil,
			want: NonRetryable,
		},
		{
			name: "server error without HTTP status (network error)",
			err:  api.NewServerError("backend down"),
			want: Retryable,
		},
		{
			name: "server error 502",
			err:  &api.APIError{Type: api.ErrorTypeServerError, Message: "bad gateway", HTTPStatus: 502},
			want: Retryable,
		},
		{
			name: "server error 503",
			err:  &api.APIError{Type: api.ErrorTypeServerError, Message: "unavailable", HTTPStatus: 503},
			want: Retryable,
		},
		{
			name: "server error 504",
			err:  &api.APIError{Type: api.ErrorTypeServerError, Message: "gateway timeout", HTTPStatus: 504},
			want: Retryable,
		},
		{
			name: "server error 500 not retryable",
			err:  &api.APIError{Type: api.ErrorTypeServerError, Message: "internal error", HTTPStatus: 500},
			want: NonRetryable,
		},
		{
			name: "too many requests",
			err:  api.NewTooManyRequestsError("rate limited"),
			want: RateLimited,
		},
		{
			name: "invalid request",
			err:  api.NewInvalidRequestError("model", "bad model"),
			want: NonRetryable,
		},
		{
			name: "not found",
			err:  api.NewNotFoundError("not found"),
			want: NonRetryable,
		},
		{
			name: "model error",
			err:  api.NewModelError("model failed"),
			want: NonRetryable,
		},
		{
			name: "context cancelled",
			err:  context.Canceled,
			want: NonRetryable,
		},
		{
			name: "context deadline exceeded",
			err:  context.DeadlineExceeded,
			want: Retryable,
		},
		{
			name: "connection refused",
			err:  &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED},
			want: Retryable,
		},
		{
			name: "connection reset",
			err:  &net.OpError{Op: "read", Err: syscall.ECONNRESET},
			want: Retryable,
		},
		{
			name: "network timeout",
			err:  &net.OpError{Op: "dial", Err: &timeoutError{}},
			want: Retryable,
		},
		{
			name: "unknown error",
			err:  errors.New("something unexpected"),
			want: NonRetryable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.err)
			if got != tt.want {
				t.Errorf("Classify() = %d, want %d", got, tt.want)
			}
		})
	}
}
