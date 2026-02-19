package vllm

import (
	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/provider/openaicompat"
)

// translateToChat delegates to openaicompat.TranslateToChat.
func translateToChat(req *provider.ProviderRequest) chatCompletionRequest {
	return openaicompat.TranslateToChat(req)
}
