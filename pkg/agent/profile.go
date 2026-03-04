// Package agent provides server-side agent profiles for antwort.
//
// An AgentProfile is a named configuration bundle (model, instructions template,
// tools, constraints) that can be referenced by name on create response requests
// via the "agent" field or the OpenAI "prompt" parameter.
package agent

import "github.com/rhuss/antwort/pkg/api"

// AgentProfile is a named server-side configuration bundle.
type AgentProfile struct {
	Name            string               `yaml:"name" json:"name"`
	Description     string               `yaml:"description" json:"description,omitempty"`
	Model           string               `yaml:"model" json:"model,omitempty"`
	Instructions    string               `yaml:"instructions" json:"-"`
	Tools           []api.ToolDefinition `yaml:"tools" json:"-"`
	Temperature     *float64             `yaml:"temperature" json:"-"`
	TopP            *float64             `yaml:"top_p" json:"-"`
	MaxOutputTokens *int                 `yaml:"max_output_tokens" json:"-"`
	MaxToolCalls    *int                 `yaml:"max_tool_calls" json:"-"`
	Reasoning       *api.ReasoningConfig `yaml:"reasoning" json:"-"`
	VectorStoreIDs  []string             `yaml:"vector_store_ids" json:"-"`
}

// ProfileResolver resolves a profile name to an AgentProfile.
type ProfileResolver interface {
	Resolve(name string) (*AgentProfile, error)
}

// ProfileSummary is a subset of AgentProfile for the list endpoint.
// It intentionally excludes instructions and tool definitions for security.
type ProfileSummary struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Model       string `json:"model,omitempty"`
}

// Summary returns a ProfileSummary from a profile.
func (p *AgentProfile) Summary() ProfileSummary {
	return ProfileSummary{
		Name:        p.Name,
		Description: p.Description,
		Model:       p.Model,
	}
}
