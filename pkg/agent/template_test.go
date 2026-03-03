package agent

import "testing"

func TestSubstituteVariables(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		variables map[string]string
		want      string
	}{
		{"single variable", "Hello {{name}}", map[string]string{"name": "Alice"}, "Hello Alice"},
		{"multiple variables", "{{greeting}} {{name}}", map[string]string{"greeting": "Hi", "name": "Bob"}, "Hi Bob"},
		{"undefined variable left as-is", "Hello {{name}}, your role is {{role}}", map[string]string{"name": "Alice"}, "Hello Alice, your role is {{role}}"},
		{"no variables in template", "Hello world", map[string]string{"name": "Alice"}, "Hello world"},
		{"empty template", "", map[string]string{"name": "Alice"}, ""},
		{"nil variables", "Hello {{name}}", nil, "Hello {{name}}"},
		{"empty variables map", "Hello {{name}}", map[string]string{}, "Hello {{name}}"},
		{"variable used twice", "{{x}} and {{x}}", map[string]string{"x": "Y"}, "Y and Y"},
		{"variable with underscores", "Project: {{project_name}}", map[string]string{"project_name": "antwort"}, "Project: antwort"},
		{"no curly braces at all", "plain text", map[string]string{"x": "y"}, "plain text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstituteVariables(tt.template, tt.variables)
			if got != tt.want {
				t.Errorf("SubstituteVariables(%q, %v) = %q, want %q", tt.template, tt.variables, got, tt.want)
			}
		})
	}
}
