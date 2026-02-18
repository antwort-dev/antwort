package vllm

import (
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

// translateResponse converts a chatCompletionResponse into a ProviderResponse.
// It uses only choices[0] and maps content, tool calls, finish reason, and usage.
func translateResponse(resp *chatCompletionResponse) *provider.ProviderResponse {
	pr := &provider.ProviderResponse{
		Model:  resp.Model,
		Status: api.ResponseStatusCompleted,
	}

	// Map usage.
	if resp.Usage != nil {
		pr.Usage = api.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
	}

	// Need at least one choice. Empty choices means the backend produced no output.
	if len(resp.Choices) == 0 {
		pr.Status = api.ResponseStatusFailed
		return pr
	}

	choice := resp.Choices[0]

	// Map finish_reason to response status.
	pr.Status = mapFinishReason(choice.FinishReason)

	// Parse message content into an assistant message Item.
	if contentStr := extractContentString(choice.Message.Content); contentStr != "" {
		pr.Items = append(pr.Items, api.Item{
			ID:     api.NewItemID(),
			Type:   api.ItemTypeMessage,
			Status: api.ItemStatusCompleted,
			Message: &api.MessageData{
				Role: api.RoleAssistant,
				Output: []api.OutputContentPart{
					{
						Type: "output_text",
						Text: contentStr,
					},
				},
			},
		})
	}

	// Parse reasoning content (e.g., DeepSeek R1).
	if choice.Message.ReasoningContent != nil && *choice.Message.ReasoningContent != "" {
		pr.Items = append(pr.Items, api.Item{
			ID:        api.NewItemID(),
			Type:      api.ItemTypeReasoning,
			Status:    api.ItemStatusCompleted,
			Reasoning: &api.ReasoningData{Content: *choice.Message.ReasoningContent},
		})
	}

	// Parse tool calls into separate function_call Items.
	for _, tc := range choice.Message.ToolCalls {
		pr.Items = append(pr.Items, api.Item{
			ID:     api.NewItemID(),
			Type:   api.ItemTypeFunctionCall,
			Status: api.ItemStatusCompleted,
			FunctionCall: &api.FunctionCallData{
				Name:      tc.Function.Name,
				CallID:    tc.ID,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return pr
}

// mapFinishReason converts a Chat Completions finish_reason string to a
// ResponseStatus.
func mapFinishReason(reason string) api.ResponseStatus {
	switch reason {
	case "stop":
		return api.ResponseStatusCompleted
	case "length":
		return api.ResponseStatusIncomplete
	case "tool_calls":
		return api.ResponseStatusCompleted
	case "content_filter":
		return api.ResponseStatusFailed
	default:
		// Unknown finish_reason: treat as completed, log warning upstream.
		return api.ResponseStatusCompleted
	}
}

// extractContentString attempts to get a plain string from the message content.
// The content field in Chat Completions can be a string or nil.
func extractContentString(content any) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	default:
		return ""
	}
}
