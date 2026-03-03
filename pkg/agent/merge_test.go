package agent

import (
	"testing"

	"github.com/rhuss/antwort/pkg/api"
)

func float64Ptr(f float64) *float64 { return &f }
func intPtr(i int) *int             { return &i }

func TestMergeProfileIntoRequest_ModelDefault(t *testing.T) {
	profile := &AgentProfile{Model: "model-a"}
	req := &api.CreateResponseRequest{}
	MergeProfileIntoRequest(profile, req, nil)
	if req.Model != "model-a" {
		t.Errorf("model: got %q, want model-a", req.Model)
	}
}

func TestMergeProfileIntoRequest_ModelOverride(t *testing.T) {
	profile := &AgentProfile{Model: "model-a"}
	req := &api.CreateResponseRequest{Model: "model-b"}
	MergeProfileIntoRequest(profile, req, nil)
	if req.Model != "model-b" {
		t.Errorf("model: got %q, want model-b (request wins)", req.Model)
	}
}

func TestMergeProfileIntoRequest_ToolsUnion(t *testing.T) {
	profile := &AgentProfile{
		Tools: []api.ToolDefinition{
			{Name: "web_search", Type: "builtin"},
		},
	}
	req := &api.CreateResponseRequest{
		Tools: []api.ToolDefinition{
			{Name: "code_interpreter", Type: "builtin"},
		},
	}
	MergeProfileIntoRequest(profile, req, nil)
	if len(req.Tools) != 2 {
		t.Fatalf("tools: got %d, want 2 (union)", len(req.Tools))
	}
}

func TestMergeProfileIntoRequest_ToolsNoDuplicate(t *testing.T) {
	profile := &AgentProfile{
		Tools: []api.ToolDefinition{
			{Name: "web_search", Type: "builtin"},
		},
	}
	req := &api.CreateResponseRequest{
		Tools: []api.ToolDefinition{
			{Name: "web_search", Type: "builtin"},
		},
	}
	MergeProfileIntoRequest(profile, req, nil)
	if len(req.Tools) != 1 {
		t.Fatalf("tools: got %d, want 1 (no duplicate)", len(req.Tools))
	}
}

func TestMergeProfileIntoRequest_TemperatureDefault(t *testing.T) {
	profile := &AgentProfile{Temperature: float64Ptr(0.3)}
	req := &api.CreateResponseRequest{}
	MergeProfileIntoRequest(profile, req, nil)
	if req.Temperature == nil || *req.Temperature != 0.3 {
		t.Error("temperature should be 0.3 from profile")
	}
}

func TestMergeProfileIntoRequest_TemperatureOverride(t *testing.T) {
	profile := &AgentProfile{Temperature: float64Ptr(0.3)}
	req := &api.CreateResponseRequest{Temperature: float64Ptr(0.7)}
	MergeProfileIntoRequest(profile, req, nil)
	if *req.Temperature != 0.7 {
		t.Errorf("temperature: got %f, want 0.7 (request wins)", *req.Temperature)
	}
}

func TestMergeProfileIntoRequest_InstructionsWithVariables(t *testing.T) {
	profile := &AgentProfile{Instructions: "Help with {{project_name}}"}
	req := &api.CreateResponseRequest{}
	vars := map[string]string{"project_name": "antwort"}
	MergeProfileIntoRequest(profile, req, vars)
	if req.Instructions != "Help with antwort" {
		t.Errorf("instructions: got %q, want 'Help with antwort'", req.Instructions)
	}
}

func TestMergeProfileIntoRequest_NilProfile(t *testing.T) {
	req := &api.CreateResponseRequest{Model: "model-b"}
	MergeProfileIntoRequest(nil, req, nil)
	if req.Model != "model-b" {
		t.Error("nil profile should not change request")
	}
}

func TestMergeProfileIntoRequest_NoProfile(t *testing.T) {
	req := &api.CreateResponseRequest{}
	MergeProfileIntoRequest(nil, req, nil)
	if req.Model != "" {
		t.Error("nil profile should leave model empty")
	}
}
