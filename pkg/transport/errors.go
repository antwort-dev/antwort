package transport

import (
	"encoding/json"
	"net/http"

	"github.com/rhuss/antwort/pkg/api"
)

// HTTPStatusFromError maps an APIError type to the corresponding HTTP status
// code. Transport-level errors (body too large, unsupported content type,
// method not allowed) are handled separately by the HTTP adapter.
func HTTPStatusFromError(err *api.APIError) int {
	switch err.Type {
	case api.ErrorTypeInvalidRequest:
		return http.StatusBadRequest
	case api.ErrorTypeNotFound:
		return http.StatusNotFound
	case api.ErrorTypeTooManyRequests:
		return http.StatusTooManyRequests
	case api.ErrorTypeServerError, api.ErrorTypeModelError:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// WriteErrorResponse writes a JSON error response using the ErrorResponse
// wrapper format from pkg/api. It sets the Content-Type header and writes
// the HTTP status code.
func WriteErrorResponse(w http.ResponseWriter, apiErr *api.APIError, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(api.ErrorResponse{Error: apiErr})
}

// WriteAPIError writes an APIError response, deriving the HTTP status code
// from the error type.
func WriteAPIError(w http.ResponseWriter, apiErr *api.APIError) {
	WriteErrorResponse(w, apiErr, HTTPStatusFromError(apiErr))
}
