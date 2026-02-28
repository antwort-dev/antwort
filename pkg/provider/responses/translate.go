package responses

import (
	"encoding/json"
	"fmt"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
)

// translateRequest converts a ProviderRequest to the Responses API wire format.
// Built-in tool types are expanded to function definitions before translation.
// Always sets store=false since the gateway manages state.
func translateRequest(req *provider.ProviderRequest) (*responsesRequest, error) {
	// Expand built-in tool types to function definitions.
	expandedTools := provider.ExpandBuiltinTools(req.Tools, req.BuiltinToolDefs)

	// Build input from messages.
	input, err := translateMessages(req.Messages)
	if err != nil {
		return nil, fmt.Errorf("translate messages: %w", err)
	}

	rr := &responsesRequest{
		Model:       req.Model,
		Input:       input,
		Store:       false, // Gateway manages state, not the backend.
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxOutputTokens: req.MaxTokens,
		Stop:        req.Stop,
		User:        req.User,
	}

	// Translate tools.
	for _, pt := range expandedTools {
		rr.Tools = append(rr.Tools, responsesTool{
			Type:        pt.Type,
			Name:        pt.Function.Name,
			Description: pt.Function.Description,
			Parameters:  pt.Function.Parameters,
		})
	}

	// Translate tool choice.
	if req.ToolChoice != nil {
		if req.ToolChoice.String != "" {
			rr.ToolChoice = req.ToolChoice.String
		} else if req.ToolChoice.Function != nil {
			rr.ToolChoice = req.ToolChoice.Function
		}
	}

	// Translate response format.
	// ResponseFormat is *api.TextConfig which has a Format *api.TextFormat field.
	// The wire format needs {"text": {"format": {<TextFormat fields>}}}, so we
	// marshal only the inner Format field to avoid double-nesting.
	if req.ResponseFormat != nil && req.ResponseFormat.Format != nil {
		formatJSON, err := json.Marshal(req.ResponseFormat.Format)
		if err == nil {
			rr.Text = &responsesTextConfig{Format: formatJSON}
		}
	}

	return rr, nil
}

// translateMessages converts ProviderMessages to a JSON array suitable for
// the Responses API input field.
func translateMessages(msgs []provider.ProviderMessage) (json.RawMessage, error) {
	type inputItem struct {
		Type    string `json:"type"`
		Role    string `json:"role,omitempty"`
		Content any    `json:"content,omitempty"`
		CallID  string `json:"call_id,omitempty"`
		Name    string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
		Output  string `json:"output,omitempty"`
		ID      string `json:"id,omitempty"`
		Status  string `json:"status,omitempty"`
	}

	var items []inputItem
	for _, msg := range msgs {
		switch msg.Role {
		case "system":
			// System messages become instructions (handled via request.instructions).
			// For input, include as an "message" item with role "system".
			items = append(items, inputItem{
				Type:    "message",
				Role:    "system",
				Content: msg.Content,
			})

		case "user":
			items = append(items, inputItem{
				Type:    "message",
				Role:    "user",
				Content: msg.Content,
			})

		case "assistant":
			if len(msg.ToolCalls) > 0 {
				// Assistant with tool calls becomes function_call items.
				for _, tc := range msg.ToolCalls {
					items = append(items, inputItem{
						Type:      "function_call",
						ID:        tc.ID,
						CallID:    tc.ID,
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
						Status:    "completed",
					})
				}
			}
			// If there's also text content, add a message item.
			if msg.Content != nil {
				contentStr, ok := msg.Content.(string)
				if ok && contentStr != "" {
					items = append(items, inputItem{
						Type:    "message",
						Role:    "assistant",
						Content: msg.Content,
						Status:  "completed",
					})
				}
			}

		case "tool":
			items = append(items, inputItem{
				Type:   "function_call_output",
				CallID: msg.ToolCallID,
				Output: fmt.Sprintf("%v", msg.Content),
			})
		}
	}

	return json.Marshal(items)
}

// translateResponse converts a Responses API response to a ProviderResponse.
func translateResponse(resp *responsesResponse) (*provider.ProviderResponse, error) {
	pr := &provider.ProviderResponse{
		Model:  resp.Model,
		Status: mapResponseStatus(resp.Status),
	}

	if resp.Usage != nil {
		pr.Usage = api.Usage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
	}

	// Convert output items.
	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			msgData := &api.MessageData{
				Role: api.MessageRole(item.Role),
			}
			// Parse content parts.
			var parts []responsesContentPart
			if err := json.Unmarshal(item.Content, &parts); err == nil {
				for _, p := range parts {
					if p.Type == "output_text" {
						msgData.Output = append(msgData.Output, api.OutputContentPart{
							Type: "output_text",
							Text: p.Text,
						})
					}
				}
			}
			pr.Items = append(pr.Items, api.Item{
				ID:      item.ID,
				Type:    api.ItemTypeMessage,
				Status:  api.ItemStatusCompleted,
				Message: msgData,
			})

		case "function_call":
			pr.Items = append(pr.Items, api.Item{
				ID:     item.ID,
				Type:   api.ItemTypeFunctionCall,
				Status: api.ItemStatusCompleted,
				FunctionCall: &api.FunctionCallData{
					Name:      item.Name,
					CallID:    item.CallID,
					Arguments: item.Arguments,
				},
			})
		}
	}

	return pr, nil
}

// mapResponseStatus maps the Responses API status string to the internal type.
func mapResponseStatus(status string) api.ResponseStatus {
	switch status {
	case "completed":
		return api.ResponseStatusCompleted
	case "incomplete":
		return api.ResponseStatusIncomplete
	case "failed":
		return api.ResponseStatusFailed
	default:
		return api.ResponseStatusCompleted
	}
}
