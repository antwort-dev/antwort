package engine

import (
	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

// translateRequest converts an OpenResponses CreateResponseRequest into
// a provider-level ProviderRequest suitable for backend invocation.
func translateRequest(req *api.CreateResponseRequest) *provider.ProviderRequest {
	pr := &provider.ProviderRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxOutputTokens,
		Stream:      req.Stream,
	}

	// Map tool choice directly if set.
	if req.ToolChoice != nil {
		pr.ToolChoice = req.ToolChoice
	}

	// System instructions become the first message.
	if req.Instructions != "" {
		pr.Messages = append(pr.Messages, provider.ProviderMessage{
			Role:    "system",
			Content: req.Instructions,
		})
	}

	// Translate each input Item to ProviderMessage(s).
	for _, item := range req.Input {
		msgs := translateItem(item)
		pr.Messages = append(pr.Messages, msgs...)
	}

	// Map tools from api.ToolDefinition to provider.ProviderTool.
	for _, t := range req.Tools {
		pr.Tools = append(pr.Tools, provider.ProviderTool{
			Type: t.Type,
			Function: provider.ProviderFunctionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	return pr
}

// translateItem converts a single api.Item into zero or more ProviderMessages.
// Reasoning items are intentionally skipped (not sent to backends).
func translateItem(item api.Item) []provider.ProviderMessage {
	switch item.Type {
	case api.ItemTypeMessage:
		return translateMessageItem(item)
	case api.ItemTypeFunctionCall:
		return translateFunctionCallItem(item)
	case api.ItemTypeFunctionCallOutput:
		return translateFunctionCallOutputItem(item)
	case api.ItemTypeReasoning:
		// Reasoning items are not sent to the backend.
		return nil
	default:
		return nil
	}
}

// translateMessageItem converts a message item based on its role.
func translateMessageItem(item api.Item) []provider.ProviderMessage {
	if item.Message == nil {
		return nil
	}

	role := string(item.Message.Role)

	switch item.Message.Role {
	case api.RoleUser:
		// For user messages, use content parts.
		content := extractUserContent(item.Message.Content)
		return []provider.ProviderMessage{
			{Role: role, Content: content},
		}

	case api.RoleAssistant:
		// For assistant messages, extract text from OutputContentParts.
		text := extractAssistantContent(item.Message.Output)
		return []provider.ProviderMessage{
			{Role: role, Content: text},
		}

	case api.RoleSystem:
		// System messages use the content parts text.
		content := extractUserContent(item.Message.Content)
		return []provider.ProviderMessage{
			{Role: role, Content: content},
		}

	default:
		return nil
	}
}

// extractUserContent builds a string from ContentParts (text-only for now).
func extractUserContent(parts []api.ContentPart) string {
	if len(parts) == 0 {
		return ""
	}
	// For text-only, concatenate all input_text parts.
	var result string
	for _, p := range parts {
		if p.Type == "input_text" {
			result += p.Text
		}
	}
	return result
}

// extractAssistantContent builds a string from OutputContentParts.
func extractAssistantContent(parts []api.OutputContentPart) string {
	if len(parts) == 0 {
		return ""
	}
	var result string
	for _, p := range parts {
		if p.Type == "output_text" {
			result += p.Text
		}
	}
	return result
}

// translateFunctionCallItem converts a function_call Item into an assistant
// message with a tool_calls entry.
func translateFunctionCallItem(item api.Item) []provider.ProviderMessage {
	if item.FunctionCall == nil {
		return nil
	}
	return []provider.ProviderMessage{
		{
			Role:    "assistant",
			Content: nil,
			ToolCalls: []provider.ProviderToolCall{
				{
					ID:   item.FunctionCall.CallID,
					Type: "function",
					Function: provider.ProviderFunctionCall{
						Name:      item.FunctionCall.Name,
						Arguments: item.FunctionCall.Arguments,
					},
				},
			},
		},
	}
}

// translateFunctionCallOutputItem converts a function_call_output Item into
// a tool-role message.
func translateFunctionCallOutputItem(item api.Item) []provider.ProviderMessage {
	if item.FunctionCallOutput == nil {
		return nil
	}
	return []provider.ProviderMessage{
		{
			Role:       "tool",
			Content:    item.FunctionCallOutput.Output,
			ToolCallID: item.FunctionCallOutput.CallID,
		},
	}
}
