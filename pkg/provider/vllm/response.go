package vllm

import (
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/provider/openaicompat"
)

// translateResponse delegates to openaicompat.TranslateResponse.
func translateResponse(resp *chatCompletionResponse) *provider.ProviderResponse {
	return openaicompat.TranslateResponse(resp)
}

// mapFinishReason delegates to openaicompat.MapFinishReason.
func mapFinishReason(reason string) api.ResponseStatus {
	return openaicompat.MapFinishReason(reason)
}

// extractContentString delegates to openaicompat.ExtractContentString.
func extractContentString(content any) string {
	return openaicompat.ExtractContentString(content)
}
