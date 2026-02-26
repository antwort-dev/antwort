package provider

import "log/slog"

// BuiltinToolTypes maps OpenResponses built-in tool type names to the
// function name used by the corresponding FunctionProvider.
var BuiltinToolTypes = map[string]string{
	"code_interpreter":   "code_interpreter",
	"file_search":        "file_search",
	"web_search_preview": "web_search",
}

// ExpandBuiltinTools replaces built-in tool type stubs (e.g., {"type": "code_interpreter"})
// with the full function definitions from builtinDefs. Both Chat Completions and
// Responses API providers call this before forwarding tools to the backend.
//
// A stub is identified by having a Type in BuiltinToolTypes and an empty Function.Name.
// Tools that are already fully defined (type: "function" with a name) pass through unchanged.
//
// If a built-in type has no matching definition in builtinDefs, it is dropped with a warning
// (the corresponding FunctionProvider is not registered).
func ExpandBuiltinTools(tools []ProviderTool, builtinDefs []ProviderTool) []ProviderTool {
	if len(builtinDefs) == 0 {
		return tools
	}

	// Index definitions by function name.
	defByName := make(map[string]ProviderTool, len(builtinDefs))
	for _, d := range builtinDefs {
		defByName[d.Function.Name] = d
	}

	expanded := make([]ProviderTool, 0, len(tools))
	for _, t := range tools {
		funcName, isBuiltin := BuiltinToolTypes[t.Type]
		if isBuiltin && t.Function.Name == "" {
			if def, found := defByName[funcName]; found {
				expanded = append(expanded, def)
				continue
			}
			slog.Warn("built-in tool type requested but no provider registered", "type", t.Type)
			continue
		}
		expanded = append(expanded, t)
	}
	return expanded
}
