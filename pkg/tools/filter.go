package tools

// FilterResult holds the outcome of filtering tool calls against allowed_tools.
type FilterResult struct {
	// Allowed contains tool calls that passed the filter.
	Allowed []ToolCall

	// Rejected contains tool calls that were not in the allowed list,
	// paired with error results to feed back to the model.
	Rejected []ToolResult
}

// FilterAllowedTools checks each tool call against the allowed list.
// If allowedTools is empty or nil, all tool calls are allowed.
// Returns a FilterResult with allowed and rejected tool calls.
func FilterAllowedTools(calls []ToolCall, allowedTools []string) FilterResult {
	// No filter: all allowed.
	if len(allowedTools) == 0 {
		return FilterResult{Allowed: calls}
	}

	// Build lookup set.
	allowed := make(map[string]bool, len(allowedTools))
	for _, name := range allowedTools {
		allowed[name] = true
	}

	var result FilterResult
	for _, call := range calls {
		if allowed[call.Name] {
			result.Allowed = append(result.Allowed, call)
		} else {
			result.Rejected = append(result.Rejected, ToolResult{
				CallID:  call.ID,
				Output:  "tool " + call.Name + " is not in the allowed_tools list",
				IsError: true,
			})
		}
	}

	return result
}
