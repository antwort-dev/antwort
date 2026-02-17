package vllm

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/rhuss/antwort/pkg/api"
)

func makeResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestMapHTTPError_400(t *testing.T) {
	resp := makeResponse(400, `{"error":{"message":"bad model param","type":"invalid_request_error"}}`)
	apiErr := mapHTTPError(resp)

	if apiErr.Type != api.ErrorTypeInvalidRequest {
		t.Errorf("expected type %q, got %q", api.ErrorTypeInvalidRequest, apiErr.Type)
	}
	if apiErr.Message != "bad model param" {
		t.Errorf("expected parsed message, got %q", apiErr.Message)
	}
}

func TestMapHTTPError_400_NoBody(t *testing.T) {
	resp := makeResponse(400, "")
	apiErr := mapHTTPError(resp)

	if apiErr.Type != api.ErrorTypeInvalidRequest {
		t.Errorf("expected type %q, got %q", api.ErrorTypeInvalidRequest, apiErr.Type)
	}
	if apiErr.Message != "invalid request to backend" {
		t.Errorf("expected default message, got %q", apiErr.Message)
	}
}

func TestMapHTTPError_401(t *testing.T) {
	resp := makeResponse(401, "")
	apiErr := mapHTTPError(resp)

	if apiErr.Type != api.ErrorTypeServerError {
		t.Errorf("expected type %q, got %q", api.ErrorTypeServerError, apiErr.Type)
	}
}

func TestMapHTTPError_403(t *testing.T) {
	resp := makeResponse(403, "")
	apiErr := mapHTTPError(resp)

	if apiErr.Type != api.ErrorTypeServerError {
		t.Errorf("expected type %q, got %q", api.ErrorTypeServerError, apiErr.Type)
	}
}

func TestMapHTTPError_404(t *testing.T) {
	resp := makeResponse(404, `{"error":{"message":"Model not found"}}`)
	apiErr := mapHTTPError(resp)

	if apiErr.Type != api.ErrorTypeNotFound {
		t.Errorf("expected type %q, got %q", api.ErrorTypeNotFound, apiErr.Type)
	}
	if apiErr.Message != "Model not found" {
		t.Errorf("expected parsed message, got %q", apiErr.Message)
	}
}

func TestMapHTTPError_429(t *testing.T) {
	resp := makeResponse(429, "")
	apiErr := mapHTTPError(resp)

	if apiErr.Type != api.ErrorTypeTooManyRequests {
		t.Errorf("expected type %q, got %q", api.ErrorTypeTooManyRequests, apiErr.Type)
	}
}

func TestMapHTTPError_500(t *testing.T) {
	resp := makeResponse(500, `{"error":{"message":"internal error"}}`)
	apiErr := mapHTTPError(resp)

	if apiErr.Type != api.ErrorTypeServerError {
		t.Errorf("expected type %q, got %q", api.ErrorTypeServerError, apiErr.Type)
	}
	if apiErr.Message != "internal error" {
		t.Errorf("expected parsed message, got %q", apiErr.Message)
	}
}

func TestMapHTTPError_502(t *testing.T) {
	resp := makeResponse(502, "")
	apiErr := mapHTTPError(resp)

	if apiErr.Type != api.ErrorTypeServerError {
		t.Errorf("expected type %q, got %q", api.ErrorTypeServerError, apiErr.Type)
	}
}

func TestMapHTTPError_503(t *testing.T) {
	resp := makeResponse(503, "")
	apiErr := mapHTTPError(resp)

	if apiErr.Type != api.ErrorTypeServerError {
		t.Errorf("expected type %q, got %q", api.ErrorTypeServerError, apiErr.Type)
	}
}

func TestMapHTTPError_UnexpectedStatus(t *testing.T) {
	resp := makeResponse(418, "")
	apiErr := mapHTTPError(resp)

	if apiErr.Type != api.ErrorTypeServerError {
		t.Errorf("expected type %q, got %q", api.ErrorTypeServerError, apiErr.Type)
	}
}

func TestMapNetworkError(t *testing.T) {
	err := io.ErrUnexpectedEOF
	apiErr := mapNetworkError(err)

	if apiErr.Type != api.ErrorTypeServerError {
		t.Errorf("expected type %q, got %q", api.ErrorTypeServerError, apiErr.Type)
	}
	if apiErr.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestExtractErrorMessage_ValidJSON(t *testing.T) {
	body := `{"error":{"message":"something went wrong","type":"server_error"}}`
	msg := extractErrorMessage(bytes.NewBufferString(body))

	if msg != "something went wrong" {
		t.Errorf("expected %q, got %q", "something went wrong", msg)
	}
}

func TestExtractErrorMessage_InvalidJSON(t *testing.T) {
	msg := extractErrorMessage(bytes.NewBufferString("not json"))
	if msg != "" {
		t.Errorf("expected empty message, got %q", msg)
	}
}

func TestExtractErrorMessage_NilBody(t *testing.T) {
	msg := extractErrorMessage(nil)
	if msg != "" {
		t.Errorf("expected empty message, got %q", msg)
	}
}
