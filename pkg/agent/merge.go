package agent

import "github.com/rhuss/antwort/pkg/api"

// MergeProfileIntoRequest applies profile defaults to a request.
// Request-level fields always take precedence over profile defaults.
// Tools are merged (union of profile tools and request tools).
// Returns the profile's VectorStoreIDs (if any) for context injection.
func MergeProfileIntoRequest(profile *AgentProfile, req *api.CreateResponseRequest, variables map[string]string) []string {
	if profile == nil {
		return nil
	}

	// Model: profile provides default, request overrides.
	if req.Model == "" && profile.Model != "" {
		req.Model = profile.Model
	}

	// Instructions: profile provides default (with variable substitution), request overrides.
	if req.Instructions == "" && profile.Instructions != "" {
		instructions := SubstituteVariables(profile.Instructions, variables)
		req.Instructions = instructions
	}

	// Tools: union (profile tools + request tools).
	if len(profile.Tools) > 0 {
		// Build set of existing tool names to avoid duplicates.
		existing := make(map[string]bool, len(req.Tools))
		for _, t := range req.Tools {
			existing[t.Name] = true
		}
		for _, t := range profile.Tools {
			if !existing[t.Name] {
				req.Tools = append(req.Tools, t)
			}
		}
	}

	// Numeric parameters: profile provides default, request overrides.
	if req.Temperature == nil && profile.Temperature != nil {
		req.Temperature = profile.Temperature
	}
	if req.TopP == nil && profile.TopP != nil {
		req.TopP = profile.TopP
	}
	if req.MaxOutputTokens == nil && profile.MaxOutputTokens != nil {
		req.MaxOutputTokens = profile.MaxOutputTokens
	}
	if req.MaxToolCalls == nil && profile.MaxToolCalls != nil {
		req.MaxToolCalls = profile.MaxToolCalls
	}
	if req.Reasoning == nil && profile.Reasoning != nil {
		req.Reasoning = profile.Reasoning
	}

	return profile.VectorStoreIDs
}
