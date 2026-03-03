package agent

import (
	"fmt"
	"sync"

	"github.com/rhuss/antwort/pkg/api"
	"github.com/rhuss/antwort/pkg/config"
)

// ConfigResolver resolves profiles from config file data.
type ConfigResolver struct {
	mu       sync.RWMutex
	profiles map[string]*AgentProfile
}

// Compile-time check.
var _ ProfileResolver = (*ConfigResolver)(nil)

// NewConfigResolver creates a ProfileResolver from config agent profiles.
func NewConfigResolver(agents map[string]config.AgentProfileConfig) (*ConfigResolver, error) {
	profiles := make(map[string]*AgentProfile, len(agents))

	for name, cfg := range agents {
		profile := &AgentProfile{
			Name:            name,
			Description:     cfg.Description,
			Model:           cfg.Model,
			Instructions:    cfg.Instructions,
			Temperature:     cfg.Temperature,
			TopP:            cfg.TopP,
			MaxOutputTokens: cfg.MaxOutputTokens,
			MaxToolCalls:    cfg.MaxToolCalls,
		}

		// Convert tool configs to ToolDefinitions.
		for _, toolCfg := range cfg.Tools {
			td := api.ToolDefinition{}
			if t, ok := toolCfg["type"].(string); ok {
				td.Type = t
			}
			if n, ok := toolCfg["name"].(string); ok {
				td.Name = n
			}
			if d, ok := toolCfg["description"].(string); ok {
				td.Description = d
			}
			profile.Tools = append(profile.Tools, td)
		}

		// Convert reasoning config.
		if cfg.Reasoning != nil {
			profile.Reasoning = &api.ReasoningConfig{}
			if cfg.Reasoning.Effort != "" {
				effort := cfg.Reasoning.Effort
				profile.Reasoning.Effort = &effort
			}
			if cfg.Reasoning.Summary != "" {
				summary := cfg.Reasoning.Summary
				profile.Reasoning.Summary = &summary
			}
		}

		profiles[name] = profile
	}

	return &ConfigResolver{profiles: profiles}, nil
}

// Resolve returns the profile with the given name.
func (r *ConfigResolver) Resolve(name string) (*AgentProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	profile, ok := r.profiles[name]
	if !ok {
		return nil, fmt.Errorf("agent profile %q not found", name)
	}
	return profile, nil
}

// List returns summaries of all profiles.
func (r *ConfigResolver) List() []ProfileSummary {
	r.mu.RLock()
	defer r.mu.RUnlock()

	summaries := make([]ProfileSummary, 0, len(r.profiles))
	for _, p := range r.profiles {
		summaries = append(summaries, p.Summary())
	}
	return summaries
}
