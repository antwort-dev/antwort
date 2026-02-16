package api

import (
	"encoding/json"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func boolPtr(b bool) *bool         { return &b }
func intPtr(i int) *int            { return &i }
func float64Ptr(f float64) *float64 { return &f }

// validRequest returns a minimal valid CreateResponseRequest.
func validRequest() *CreateResponseRequest {
	return &CreateResponseRequest{
		Model: "test-model",
		Input: []Item{
			{
				Type:    ItemTypeMessage,
				Message: &MessageData{Role: RoleUser, Content: []ContentPart{{Type: "input_text", Text: "hello"}}},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// TestValidateRequest
// ---------------------------------------------------------------------------

func TestValidateRequest(t *testing.T) {
	cfg := DefaultValidationConfig()

	tests := []struct {
		name      string
		modify    func(r *CreateResponseRequest)
		wantErr   bool
		wantParam string
	}{
		{
			name:    "valid request accepted",
			modify:  func(r *CreateResponseRequest) {},
			wantErr: false,
		},
		{
			name:      "missing model rejected",
			modify:    func(r *CreateResponseRequest) { r.Model = "" },
			wantErr:   true,
			wantParam: "model",
		},
		{
			name:      "empty input rejected",
			modify:    func(r *CreateResponseRequest) { r.Input = nil },
			wantErr:   true,
			wantParam: "input",
		},
		{
			name:      "max_output_tokens=0 rejected",
			modify:    func(r *CreateResponseRequest) { r.MaxOutputTokens = intPtr(0) },
			wantErr:   true,
			wantParam: "max_output_tokens",
		},
		{
			name:      "negative max_output_tokens rejected",
			modify:    func(r *CreateResponseRequest) { r.MaxOutputTokens = intPtr(-5) },
			wantErr:   true,
			wantParam: "max_output_tokens",
		},
		{
			name:      "temperature -0.1 rejected",
			modify:    func(r *CreateResponseRequest) { r.Temperature = float64Ptr(-0.1) },
			wantErr:   true,
			wantParam: "temperature",
		},
		{
			name:      "temperature 2.1 rejected",
			modify:    func(r *CreateResponseRequest) { r.Temperature = float64Ptr(2.1) },
			wantErr:   true,
			wantParam: "temperature",
		},
		{
			name:      "top_p -0.1 rejected",
			modify:    func(r *CreateResponseRequest) { r.TopP = float64Ptr(-0.1) },
			wantErr:   true,
			wantParam: "top_p",
		},
		{
			name:      "top_p 1.1 rejected",
			modify:    func(r *CreateResponseRequest) { r.TopP = float64Ptr(1.1) },
			wantErr:   true,
			wantParam: "top_p",
		},
		{
			name:      "truncation invalid rejected",
			modify:    func(r *CreateResponseRequest) { r.Truncation = "invalid" },
			wantErr:   true,
			wantParam: "truncation",
		},
		{
			name:    "truncation auto accepted",
			modify:  func(r *CreateResponseRequest) { r.Truncation = "auto" },
			wantErr: false,
		},
		{
			name:    "truncation disabled accepted",
			modify:  func(r *CreateResponseRequest) { r.Truncation = "disabled" },
			wantErr: false,
		},
		{
			name: "forced tool_choice referencing missing tool rejected",
			modify: func(r *CreateResponseRequest) {
				r.ToolChoice = &ToolChoice{Function: &ToolChoiceFunction{Type: "function", Name: "nonexistent"}}
			},
			wantErr:   true,
			wantParam: "tool_choice",
		},
		{
			name: "forced tool_choice referencing existing tool accepted",
			modify: func(r *CreateResponseRequest) {
				r.Tools = []ToolDefinition{{Type: "function", Name: "my_func"}}
				r.ToolChoice = &ToolChoice{Function: &ToolChoiceFunction{Type: "function", Name: "my_func"}}
			},
			wantErr: false,
		},
		{
			name: "input exceeding MaxInputItems rejected",
			modify: func(r *CreateResponseRequest) {
				items := make([]Item, cfg.MaxInputItems+1)
				for i := range items {
					items[i] = Item{
						Type:    ItemTypeMessage,
						Message: &MessageData{Role: RoleUser},
					}
				}
				r.Input = items
			},
			wantErr:   true,
			wantParam: "input",
		},
		{
			name: "tools exceeding MaxTools rejected",
			modify: func(r *CreateResponseRequest) {
				tools := make([]ToolDefinition, cfg.MaxTools+1)
				for i := range tools {
					tools[i] = ToolDefinition{Type: "function", Name: "tool"}
				}
				r.Tools = tools
			},
			wantErr:   true,
			wantParam: "tools",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validRequest()
			tt.modify(req)
			err := ValidateRequest(req, cfg)

			if tt.wantErr && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error but got: %v", err)
			}
			if tt.wantErr && err != nil && tt.wantParam != "" {
				if err.Param != tt.wantParam {
					t.Errorf("expected param %q, got %q", tt.wantParam, err.Param)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestValidateItem
// ---------------------------------------------------------------------------

func TestValidateItem(t *testing.T) {
	tests := []struct {
		name      string
		item      Item
		wantErr   bool
		wantParam string
	}{
		{
			name: "valid message item accepted",
			item: Item{
				Type:    ItemTypeMessage,
				Message: &MessageData{Role: RoleUser, Content: []ContentPart{{Type: "input_text", Text: "hi"}}},
			},
			wantErr: false,
		},
		{
			name: "valid function_call item accepted",
			item: Item{
				Type:         ItemTypeFunctionCall,
				FunctionCall: &FunctionCallData{Name: "fn", CallID: "call_1", Arguments: "{}"},
			},
			wantErr: false,
		},
		{
			name: "valid function_call_output item accepted",
			item: Item{
				Type:               ItemTypeFunctionCallOutput,
				FunctionCallOutput: &FunctionCallOutputData{CallID: "call_1", Output: "result"},
			},
			wantErr: false,
		},
		{
			name: "valid reasoning item accepted",
			item: Item{
				Type:      ItemTypeReasoning,
				Reasoning: &ReasoningData{Content: "thinking..."},
			},
			wantErr: false,
		},
		{
			name: "valid extension item accepted",
			item: Item{
				Type:      "acme:telemetry",
				Extension: json.RawMessage(`{"key":"value"}`),
			},
			wantErr: false,
		},
		{
			name:      "empty type rejected",
			item:      Item{Type: "", Message: &MessageData{Role: RoleUser}},
			wantErr:   true,
			wantParam: "type",
		},
		{
			name:      "invalid type (no colon, not standard) rejected",
			item:      Item{Type: "bogus", Message: &MessageData{Role: RoleUser}},
			wantErr:   true,
			wantParam: "type",
		},
		{
			name:      "extension type without Extension data rejected",
			item:      Item{Type: "acme:telemetry"},
			wantErr:   true,
			wantParam: "extension",
		},
		{
			name: "multiple type-specific fields populated rejected",
			item: Item{
				Type:         ItemTypeMessage,
				Message:      &MessageData{Role: RoleUser},
				FunctionCall: &FunctionCallData{Name: "fn"},
			},
			wantErr:   true,
			wantParam: "type",
		},
		{
			name: "type/field mismatch (type=message but FunctionCall populated) rejected",
			item: Item{
				Type:         ItemTypeMessage,
				FunctionCall: &FunctionCallData{Name: "fn", CallID: "call_1", Arguments: "{}"},
			},
			wantErr:   true,
			wantParam: "message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateItem(&tt.item)

			if tt.wantErr && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error but got: %v", err)
			}
			if tt.wantErr && err != nil && tt.wantParam != "" {
				if err.Param != tt.wantParam {
					t.Errorf("expected param %q, got %q", tt.wantParam, err.Param)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestIsStateless
// ---------------------------------------------------------------------------

func TestIsStateless(t *testing.T) {
	tests := []struct {
		name  string
		store *bool
		want  bool
	}{
		{name: "store=nil -> false", store: nil, want: false},
		{name: "store=true -> false", store: boolPtr(true), want: false},
		{name: "store=false -> true", store: boolPtr(false), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &CreateResponseRequest{Store: tt.store}
			got := IsStateless(req)
			if got != tt.want {
				t.Errorf("IsStateless() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestResolveStore
// ---------------------------------------------------------------------------

func TestResolveStore(t *testing.T) {
	tests := []struct {
		name  string
		store *bool
		want  bool
	}{
		{name: "store=nil -> true (default)", store: nil, want: true},
		{name: "store=true -> true", store: boolPtr(true), want: true},
		{name: "store=false -> false", store: boolPtr(false), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &CreateResponseRequest{Store: tt.store}
			got := ResolveStore(req)
			if got != tt.want {
				t.Errorf("ResolveStore() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestValidateStatelessConstraints
// ---------------------------------------------------------------------------

func TestValidateStatelessConstraints(t *testing.T) {
	tests := []struct {
		name               string
		store              *bool
		previousResponseID string
		wantErr            bool
	}{
		{
			name:               "store=false + previous_response_id -> error",
			store:              boolPtr(false),
			previousResponseID: "resp_abc123",
			wantErr:            true,
		},
		{
			name:               "store=false + no previous_response_id -> nil",
			store:              boolPtr(false),
			previousResponseID: "",
			wantErr:            false,
		},
		{
			name:               "store=true + previous_response_id -> nil",
			store:              boolPtr(true),
			previousResponseID: "resp_abc123",
			wantErr:            false,
		},
		{
			name:               "store=nil + previous_response_id -> nil",
			store:              nil,
			previousResponseID: "resp_abc123",
			wantErr:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &CreateResponseRequest{
				Store:              tt.store,
				PreviousResponseID: tt.previousResponseID,
			}
			err := ValidateStatelessConstraints(req)

			if tt.wantErr && err == nil {
				t.Fatal("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error but got: %v", err)
			}
		})
	}
}
