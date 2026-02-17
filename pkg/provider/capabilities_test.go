package provider

import (
	"testing"

	"github.com/rhuss/antwort/pkg/api"
)

func TestValidateCapabilities(t *testing.T) {
	tests := []struct {
		name      string
		caps      ProviderCapabilities
		req       *api.CreateResponseRequest
		wantErr   bool
		wantParam string
	}{
		{
			name: "text request with minimal caps",
			caps: ProviderCapabilities{},
			req: &api.CreateResponseRequest{
				Model: "test",
				Input: []api.Item{{
					ID: "item_test", Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
					Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "hello"}}},
				}},
			},
			wantErr: false,
		},
		{
			name: "streaming request without streaming support",
			caps: ProviderCapabilities{Streaming: false},
			req: &api.CreateResponseRequest{
				Model:  "test",
				Stream: true,
				Input: []api.Item{{
					ID: "item_test", Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
					Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "hello"}}},
				}},
			},
			wantErr:   true,
			wantParam: "stream",
		},
		{
			name: "streaming request with streaming support",
			caps: ProviderCapabilities{Streaming: true},
			req: &api.CreateResponseRequest{
				Model:  "test",
				Stream: true,
				Input: []api.Item{{
					ID: "item_test", Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
					Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "hello"}}},
				}},
			},
			wantErr: false,
		},
		{
			name: "tools request without tool calling support",
			caps: ProviderCapabilities{},
			req: &api.CreateResponseRequest{
				Model: "test",
				Tools: []api.ToolDefinition{{Type: "function", Name: "get_weather"}},
				Input: []api.Item{{
					ID: "item_test", Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
					Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "hello"}}},
				}},
			},
			wantErr:   true,
			wantParam: "tools",
		},
		{
			name: "tools request with tool calling support",
			caps: ProviderCapabilities{ToolCalling: true},
			req: &api.CreateResponseRequest{
				Model: "test",
				Tools: []api.ToolDefinition{{Type: "function", Name: "get_weather"}},
				Input: []api.Item{{
					ID: "item_test", Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
					Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_text", Text: "hello"}}},
				}},
			},
			wantErr: false,
		},
		{
			name: "image input without vision support",
			caps: ProviderCapabilities{},
			req: &api.CreateResponseRequest{
				Model: "test",
				Input: []api.Item{{
					ID: "item_test", Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
					Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_image", URL: "https://example.com/img.png"}}},
				}},
			},
			wantErr:   true,
			wantParam: "input",
		},
		{
			name: "image input with vision support",
			caps: ProviderCapabilities{Vision: true},
			req: &api.CreateResponseRequest{
				Model: "test",
				Input: []api.Item{{
					ID: "item_test", Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
					Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_image", URL: "https://example.com/img.png"}}},
				}},
			},
			wantErr: false,
		},
		{
			name: "audio input without audio support",
			caps: ProviderCapabilities{},
			req: &api.CreateResponseRequest{
				Model: "test",
				Input: []api.Item{{
					ID: "item_test", Type: api.ItemTypeMessage, Status: api.ItemStatusCompleted,
					Message: &api.MessageData{Role: api.RoleUser, Content: []api.ContentPart{{Type: "input_audio", URL: "https://example.com/audio.mp3"}}},
				}},
			},
			wantErr:   true,
			wantParam: "input",
		},
		{
			name: "non-message item skipped",
			caps: ProviderCapabilities{},
			req: &api.CreateResponseRequest{
				Model: "test",
				Input: []api.Item{{
					ID: "item_test", Type: api.ItemTypeFunctionCallOutput, Status: api.ItemStatusCompleted,
					FunctionCallOutput: &api.FunctionCallOutputData{CallID: "call_1", Output: "result"},
				}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCapabilities(tt.caps, tt.req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if err.Param != tt.wantParam {
					t.Errorf("expected param %q, got %q", tt.wantParam, err.Param)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
