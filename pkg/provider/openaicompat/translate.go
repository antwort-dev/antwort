package openaicompat

import (
	"github.com/rhuss/antwort/pkg/provider"
)

// TranslateToChat converts a ProviderRequest into a ChatCompletionRequest
// suitable for the /v1/chat/completions endpoint.
func TranslateToChat(req *provider.ProviderRequest) ChatCompletionRequest {
	cr := ChatCompletionRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stop:        req.Stop,
		N:           1,
		Stream:      req.Stream,
	}

	// When streaming, enable usage reporting in the stream.
	if req.Stream {
		cr.StreamOptions = &ChatStreamOptions{
			IncludeUsage: true,
		}
	}

	// Translate messages.
	for _, pm := range req.Messages {
		cm := ChatMessage{
			Role:       pm.Role,
			Content:    pm.Content,
			ToolCallID: pm.ToolCallID,
			Name:       pm.Name,
		}
		// Translate tool calls.
		for _, tc := range pm.ToolCalls {
			cm.ToolCalls = append(cm.ToolCalls, ChatToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: ChatFunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		cr.Messages = append(cr.Messages, cm)
	}

	// Translate tools.
	for _, pt := range req.Tools {
		cr.Tools = append(cr.Tools, ChatTool{
			Type: pt.Type,
			Function: ChatFunctionDef{
				Name:        pt.Function.Name,
				Description: pt.Function.Description,
				Parameters:  pt.Function.Parameters,
			},
		})
	}

	// Map tool choice. The ChatCompletionRequest uses `any` for ToolChoice,
	// which allows both string and structured values.
	if req.ToolChoice != nil {
		if req.ToolChoice.String != "" {
			cr.ToolChoice = req.ToolChoice.String
		} else if req.ToolChoice.Function != nil {
			cr.ToolChoice = req.ToolChoice.Function
		}
	}

	return cr
}
