package openaicompat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rhuss/antwort/pkg/api"
)

// MapHTTPError converts an HTTP response with a non-2xx status code into
// an APIError. It attempts to parse the response body as a ChatErrorResponse
// to extract a descriptive message.
func MapHTTPError(resp *http.Response) *api.APIError {
	// Try to read the body for an error message.
	message := ExtractErrorMessage(resp.Body)
	status := resp.StatusCode

	var apiErr *api.APIError
	switch {
	case status == http.StatusBadRequest:
		if message == "" {
			message = "invalid request to backend"
		}
		apiErr = api.NewInvalidRequestError("", message)

	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		if message == "" {
			message = "backend authentication failed"
		}
		apiErr = api.NewServerError(message)

	case status == http.StatusNotFound:
		if message == "" {
			message = "backend resource not found"
		}
		apiErr = api.NewNotFoundError(message)

	case status == http.StatusTooManyRequests:
		if message == "" {
			message = "backend rate limit exceeded"
		}
		apiErr = api.NewTooManyRequestsError(message)
		apiErr.RetryAfter = parseRetryAfter(strings.TrimSpace(resp.Header.Get("Retry-After")))

	case status >= http.StatusInternalServerError:
		if message == "" {
			message = fmt.Sprintf("backend server error (HTTP %d)", status)
		}
		apiErr = api.NewServerError(message)

	default:
		if message == "" {
			message = fmt.Sprintf("unexpected backend error (HTTP %d)", status)
		}
		apiErr = api.NewServerError(message)
	}

	apiErr.HTTPStatus = status
	return apiErr
}

// MapNetworkError converts a network-level error (connection refused, timeout,
// DNS resolution failure) into an APIError with a descriptive message.
func MapNetworkError(err error) *api.APIError {
	return api.NewServerError(fmt.Sprintf("backend connection error: %s", err.Error()))
}

// parseRetryAfter parses the Retry-After header value.
// Supports both seconds (integer) and HTTP-date formats.
// Returns 0 if the header is empty or cannot be parsed.
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	// Try seconds format first.
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	// Try HTTP-date format.
	if t, err := http.ParseTime(value); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

// ExtractErrorMessage tries to parse the response body as a ChatErrorResponse
// and returns the error message if found.
func ExtractErrorMessage(body io.Reader) string {
	if body == nil {
		return ""
	}

	data, err := io.ReadAll(io.LimitReader(body, 4096))
	if err != nil || len(data) == 0 {
		return ""
	}

	var errResp ChatErrorResponse
	if err := json.Unmarshal(data, &errResp); err == nil && errResp.Error.Message != "" {
		return errResp.Error.Message
	}

	return ""
}
