package agent

import (
	"testing"

	"github.com/rhuss/antwort/pkg/config"
)

func TestConfigResolver_ResolveByName(t *testing.T) {
	agents := map[string]config.AgentProfileConfig{
		"devops": {
			Description:  "DevOps helper",
			Model:        "qwen-2.5",
			Instructions: "You are a DevOps assistant for {{project}}",
		},
	}

	resolver, err := NewConfigResolver(agents)
	if err != nil {
		t.Fatal(err)
	}

	profile, err := resolver.Resolve("devops")
	if err != nil {
		t.Fatal(err)
	}
	if profile.Name != "devops" {
		t.Errorf("name: got %q", profile.Name)
	}
	if profile.Model != "qwen-2.5" {
		t.Errorf("model: got %q", profile.Model)
	}
	if profile.Instructions != "You are a DevOps assistant for {{project}}" {
		t.Errorf("instructions: got %q", profile.Instructions)
	}
}

func TestConfigResolver_NotFound(t *testing.T) {
	resolver, _ := NewConfigResolver(map[string]config.AgentProfileConfig{})
	_, err := resolver.Resolve("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestConfigResolver_EmptyConfig(t *testing.T) {
	resolver, err := NewConfigResolver(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.Resolve("anything")
	if err == nil {
		t.Error("expected error for empty config")
	}
}

func TestConfigResolver_List(t *testing.T) {
	agents := map[string]config.AgentProfileConfig{
		"a": {Description: "Profile A", Model: "model-a"},
		"b": {Description: "Profile B", Model: "model-b"},
	}

	resolver, _ := NewConfigResolver(agents)
	summaries := resolver.List()
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
}

func TestConfigResolver_WithTools(t *testing.T) {
	agents := map[string]config.AgentProfileConfig{
		"tooled": {
			Tools: []map[string]interface{}{
				{"type": "builtin", "name": "web_search"},
			},
		},
	}

	resolver, _ := NewConfigResolver(agents)
	profile, _ := resolver.Resolve("tooled")
	if len(profile.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(profile.Tools))
	}
	if profile.Tools[0].Name != "web_search" {
		t.Errorf("tool name: got %q", profile.Tools[0].Name)
	}
}
