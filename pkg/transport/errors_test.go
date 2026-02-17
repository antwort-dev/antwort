package transport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
)

func TestHTTPStatusFromError(t *testing.T) {
	tests := []struct {
		name       string
		errType    api.ErrorType
		wantStatus int
	}{
		{"invalid_request -> 400", api.ErrorTypeInvalidRequest, http.StatusBadRequest},
		{"not_found -> 404", api.ErrorTypeNotFound, http.StatusNotFound},
		{"too_many_requests -> 429", api.ErrorTypeTooManyRequests, http.StatusTooManyRequests},
		{"server_error -> 500", api.ErrorTypeServerError, http.StatusInternalServerError},
		{"model_error -> 500", api.ErrorTypeModelError, http.StatusInternalServerError},
		{"unknown type -> 500", api.ErrorType("unknown"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &api.APIError{Type: tt.errType, Message: "test"}
			got := HTTPStatusFromError(err)
			if got != tt.wantStatus {
				t.Errorf("HTTPStatusFromError(%q) = %d, want %d", tt.errType, got, tt.wantStatus)
			}
		})
	}
}

func TestWriteErrorResponse(t *testing.T) {
	apiErr := api.NewInvalidRequestError("model", "is required")
	rec := httptest.NewRecorder()

	WriteErrorResponse(rec, apiErr, http.StatusBadRequest)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var resp api.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Type != api.ErrorTypeInvalidRequest {
		t.Errorf("error type = %q, want %q", resp.Error.Type, api.ErrorTypeInvalidRequest)
	}
	if resp.Error.Param != "model" {
		t.Errorf("error param = %q, want %q", resp.Error.Param, "model")
	}
	if resp.Error.Message != "is required" {
		t.Errorf("error message = %q, want %q", resp.Error.Message, "is required")
	}
}

func TestWriteAPIError(t *testing.T) {
	tests := []struct {
		name       string
		apiErr     *api.APIError
		wantStatus int
	}{
		{
			"invalid_request",
			api.NewInvalidRequestError("model", "is required"),
			http.StatusBadRequest,
		},
		{
			"not_found",
			api.NewNotFoundError("response not found"),
			http.StatusNotFound,
		},
		{
			"server_error",
			api.NewServerError("internal failure"),
			http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteAPIError(rec, tt.apiErr)

			if rec.Code != tt.wantStatus {
				t.Errorf("status code = %d, want %d", rec.Code, tt.wantStatus)
			}

			var resp api.ErrorResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Error.Type != tt.apiErr.Type {
				t.Errorf("error type = %q, want %q", resp.Error.Type, tt.apiErr.Type)
			}
		})
	}
}
