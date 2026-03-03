package agent

import "strings"

// SubstituteVariables replaces {{variable_name}} placeholders in the template
// with values from the variables map. Undefined variables are left as literal text.
func SubstituteVariables(template string, variables map[string]string) string {
	if len(variables) == 0 || template == "" {
		return template
	}
	result := template
	for name, value := range variables {
		placeholder := "{{" + name + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}
