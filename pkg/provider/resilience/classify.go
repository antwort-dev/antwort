// Package resilience provides circuit breaker and retry logic for provider.Provider.
package resilience

import (
	"context"
	"errors"
	"net"
	"syscall"

	"github.com/rhuss/antwort/pkg/api"
)

// Classification represents how an error should be handled by the resilience layer.
type Classification int

const (
	// NonRetryable indicates the error is permanent and should not be retried.
	NonRetryable Classification = iota
	// Retryable indicates the error is transient and the request can be retried.
	Retryable
	// RateLimited indicates the backend returned 429 and the request should be
	// retried after the indicated wait, without counting toward circuit breaker failures.
	RateLimited
)

// Classify determines whether an error is retryable, rate-limited, or permanent.
//
// Classification rules:
//   - 429 Too Many Requests -> RateLimited (does not affect circuit breaker)
//   - 502/503/504 Server Error -> Retryable (affects circuit breaker)
//   - Connection refused/reset -> Retryable (affects circuit breaker)
//   - Context deadline exceeded -> Retryable (affects circuit breaker)
//   - 4xx Client Error -> NonRetryable
//   - Context cancelled -> NonRetryable
//   - nil -> NonRetryable (no error to classify)
func Classify(err error) Classification {
	if err == nil {
		return NonRetryable
	}

	// Check for context cancellation first (not retryable).
	if errors.Is(err, context.Canceled) {
		return NonRetryable
	}

	// Check for context deadline exceeded (retryable, backend may be slow).
	if errors.Is(err, context.DeadlineExceeded) {
		return Retryable
	}

	// Check for APIError with specific types.
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Type {
		case api.ErrorTypeTooManyRequests:
			return RateLimited
		case api.ErrorTypeServerError:
			// Only retry specific transient HTTP statuses (502/503/504).
			// Other 5xx errors (500, 501) are not retried.
			// If HTTPStatus is 0 (e.g., network error mapped to ServerError), retry.
			switch apiErr.HTTPStatus {
			case 0, 502, 503, 504:
				return Retryable
			default:
				return NonRetryable
			}
		case api.ErrorTypeInvalidRequest, api.ErrorTypeNotFound, api.ErrorTypeModelError:
			return NonRetryable
		}
	}

	// Check for network-level errors (connection refused, reset, timeout).
	if isNetworkError(err) {
		return Retryable
	}

	// Unknown errors are not retried.
	return NonRetryable
}

// isNetworkError checks if the error is a transient network error.
func isNetworkError(err error) bool {
	// Connection refused.
	if errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	// Connection reset.
	if errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	// net.Error with Timeout() indicates a network timeout.
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}
