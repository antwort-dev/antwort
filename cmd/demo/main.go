package main

import (
	"encoding/json"
	"fmt"

	"github.com/rhuss/antwort/pkg/api"
)

func main() {
	fmt.Println("=== antwort core protocol demo ===")
	fmt.Println()

	// 1. Build a request with mixed input items
	req := &api.CreateResponseRequest{
		Model: "meta-llama/Llama-3-8B",
		Input: []api.Item{
			{
				ID:     api.NewItemID(),
				Type:   api.ItemTypeMessage,
				Status: api.ItemStatusCompleted,
				Message: &api.MessageData{
					Role: api.RoleUser,
					Content: []api.ContentPart{
						{Type: "input_text", Text: "What is the capital of France?"},
					},
				},
			},
		},
		ToolChoice: &api.ToolChoiceAuto,
		Stream:     true,
	}

	// 2. Validate request
	if err := api.ValidateRequest(req, api.DefaultValidationConfig()); err != nil {
		fmt.Printf("Validation FAILED: %v\n", err)
		return
	}
	fmt.Println("[1] Request validated successfully")

	// 3. Serialize request to JSON
	data, _ := json.MarshalIndent(req, "", "  ")
	fmt.Printf("\n[2] Request JSON:\n%s\n", data)

	// 4. Build a response (simulating what a provider would return)
	resp := api.Response{
		ID:     api.NewResponseID(),
		Object: "response",
		Status: api.ResponseStatusCompleted,
		Model:  "meta-llama/Llama-3-8B",
		Output: []api.Item{
			{
				ID:     api.NewItemID(),
				Type:   api.ItemTypeMessage,
				Status: api.ItemStatusCompleted,
				Message: &api.MessageData{
					Role: api.RoleAssistant,
					Output: []api.OutputContentPart{
						{
							Type: "output_text",
							Text: "The capital of France is Paris.",
							Annotations: []api.Annotation{
								{Type: "url_citation", Text: "Wikipedia", StartIndex: 27, EndIndex: 32},
							},
						},
					},
				},
			},
		},
		Usage:     &api.Usage{InputTokens: 12, OutputTokens: 8, TotalTokens: 20},
		CreatedAt: 1700000000,
	}

	data, _ = json.MarshalIndent(resp, "", "  ")
	fmt.Printf("\n[3] Response JSON:\n%s\n", data)

	// 5. Round-trip: marshal -> unmarshal -> verify
	var parsed api.Response
	_ = json.Unmarshal(data, &parsed)
	fmt.Printf("\n[4] Round-trip check:")
	fmt.Printf("\n    ID:     %s", parsed.ID)
	fmt.Printf("\n    Status: %s", parsed.Status)
	fmt.Printf("\n    Model:  %s", parsed.Model)
	fmt.Printf("\n    Output: %q", parsed.Output[0].Message.Output[0].Text)
	fmt.Printf("\n    Tokens: %d in / %d out / %d total\n",
		parsed.Usage.InputTokens, parsed.Usage.OutputTokens, parsed.Usage.TotalTokens)

	// 6. State machine transitions
	fmt.Println("\n[5] State machine transitions:")
	transitions := []struct {
		from api.ResponseStatus
		to   api.ResponseStatus
	}{
		{"", api.ResponseStatusInProgress},
		{api.ResponseStatusInProgress, api.ResponseStatusCompleted},
		{api.ResponseStatusCompleted, api.ResponseStatusInProgress},
		{api.ResponseStatusFailed, api.ResponseStatusCompleted},
	}
	for _, t := range transitions {
		from := string(t.from)
		if from == "" {
			from = "(initial)"
		}
		if err := api.ValidateResponseTransition(t.from, t.to); err != nil {
			fmt.Printf("    %s -> %s: BLOCKED (%s)\n", from, t.to, err.Message)
		} else {
			fmt.Printf("    %s -> %s: OK\n", from, t.to)
		}
	}

	// 7. Validation error demo
	fmt.Println("\n[6] Validation error examples:")
	badReq := &api.CreateResponseRequest{Model: "", Input: nil}
	if err := api.ValidateRequest(badReq, api.DefaultValidationConfig()); err != nil {
		fmt.Printf("    Missing model: %v\n", err)
	}

	badReq2 := &api.CreateResponseRequest{
		Model:       "test",
		Input:       []api.Item{{Type: api.ItemTypeMessage, Message: &api.MessageData{Role: api.RoleUser}}},
		Temperature: float64Ptr(3.0),
	}
	if err := api.ValidateRequest(badReq2, api.DefaultValidationConfig()); err != nil {
		fmt.Printf("    Bad temperature: %v\n", err)
	}

	// 8. Extension types
	fmt.Println("\n[7] Extension type detection:")
	types := []api.ItemType{"message", "function_call", "acme:telemetry", "vendor:custom"}
	for _, t := range types {
		fmt.Printf("    %-20s extension=%v\n", t, api.IsExtensionType(t))
	}

	// 9. Streaming events
	fmt.Println("\n[8] Streaming event sample:")
	event := api.StreamEvent{
		Type:           api.EventOutputTextDelta,
		SequenceNumber: 42,
		Delta:          "Paris",
		ItemID:         resp.Output[0].ID,
		OutputIndex:    0,
		ContentIndex:   0,
	}
	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	fmt.Printf("%s\n", eventJSON)

	fmt.Println("\n=== demo complete ===")
}

func float64Ptr(f float64) *float64 { return &f }
