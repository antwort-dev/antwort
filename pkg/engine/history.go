package engine

import (
	"context"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/provider"
	"github.com/rhuss/antwort/pkg/transport"
)

// loadConversationHistory reconstructs the full conversation message history
// from a chain of stored responses. It follows previous_response_id links
// iteratively, detects cycles, and returns messages in chronological order
// (oldest first).
//
// The most recent instructions from the chain are returned separately so
// the caller can use them as the system message (superseding earlier instructions).
func loadConversationHistory(ctx context.Context, store transport.ResponseStore, responseID string) ([]provider.ProviderMessage, error) {
	if store == nil {
		return nil, api.NewInvalidRequestError("previous_response_id", "conversation chaining requires a response store")
	}

	// Collect responses by following the chain.
	var chain []*api.Response
	visited := make(map[string]bool)

	currentID := responseID
	for currentID != "" {
		if visited[currentID] {
			return nil, api.NewInvalidRequestError("previous_response_id", "cycle detected in response chain")
		}
		visited[currentID] = true

		resp, err := store.GetResponse(ctx, currentID)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, api.NewNotFoundError("response " + currentID + " not found")
		}

		chain = append(chain, resp)
		currentID = resp.PreviousResponseID
	}

	// Reverse to chronological order (oldest first).
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	// Extract messages from each response in the chain.
	var messages []provider.ProviderMessage
	for _, resp := range chain {
		// Convert input items to messages.
		for _, item := range resp.Input {
			if msg := itemToMessage(item); msg != nil {
				messages = append(messages, *msg)
			}
		}

		// Convert output items to messages.
		for _, item := range resp.Output {
			if msg := itemToMessage(item); msg != nil {
				messages = append(messages, *msg)
			}
		}
	}

	return messages, nil
}

// itemToMessage converts an Item to a ProviderMessage for conversation
// history reconstruction. Returns nil for items that should be skipped
// (e.g., reasoning items).
func itemToMessage(item api.Item) *provider.ProviderMessage {
	switch item.Type {
	case api.ItemTypeMessage:
		if item.Message == nil {
			return nil
		}
		msg := &provider.ProviderMessage{
			Role: string(item.Message.Role),
		}
		// Use content parts for user messages, output for assistant messages.
		if item.Message.Role == api.RoleAssistant {
			if len(item.Message.Output) > 0 {
				msg.Content = item.Message.Output[0].Text
			}
		} else {
			if len(item.Message.Content) > 0 {
				msg.Content = item.Message.Content[0].Text
			}
		}
		return msg

	case api.ItemTypeFunctionCall:
		if item.FunctionCall == nil {
			return nil
		}
		return &provider.ProviderMessage{
			Role: "assistant",
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
		}

	case api.ItemTypeFunctionCallOutput:
		if item.FunctionCallOutput == nil {
			return nil
		}
		return &provider.ProviderMessage{
			Role:       "tool",
			Content:    item.FunctionCallOutput.Output,
			ToolCallID: item.FunctionCallOutput.CallID,
		}

	case api.ItemTypeReasoning:
		// Reasoning items are not sent to the backend.
		return nil

	default:
		return nil
	}
}
