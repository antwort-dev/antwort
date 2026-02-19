package vllm

import (
	"io"
	"net/http"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider/openaicompat"
)

// mapHTTPError delegates to openaicompat.MapHTTPError.
func mapHTTPError(resp *http.Response) *api.APIError {
	return openaicompat.MapHTTPError(resp)
}

// mapNetworkError delegates to openaicompat.MapNetworkError.
func mapNetworkError(err error) *api.APIError {
	return openaicompat.MapNetworkError(err)
}

// extractErrorMessage delegates to openaicompat.ExtractErrorMessage.
func extractErrorMessage(body io.Reader) string {
	return openaicompat.ExtractErrorMessage(body)
}
