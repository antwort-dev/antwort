// Package scope provides scope-based authorization for API endpoints.
package scope

import (
	"fmt"
	"strings"
)

// ExpandRoles resolves role definitions with possible references to other roles.
// Each role maps to a list of scopes. If a scope matches another role name, it is
// expanded recursively. The wildcard "*" is preserved as-is.
// Returns an error on circular references or undefined role references.
func ExpandRoles(config map[string][]string) (map[string]map[string]bool, error) {
	result := make(map[string]map[string]bool, len(config))

	for roleName := range config {
		if _, ok := result[roleName]; ok {
			continue
		}
		visited := map[string]bool{}
		scopes, err := resolveRole(roleName, config, visited, result)
		if err != nil {
			return nil, err
		}
		result[roleName] = scopes
	}

	return result, nil
}

// resolveRole recursively expands a single role, detecting cycles via visited set.
func resolveRole(
	name string,
	config map[string][]string,
	visited map[string]bool,
	cache map[string]map[string]bool,
) (map[string]bool, error) {
	// Check cache first.
	if cached, ok := cache[name]; ok {
		return cached, nil
	}

	// Cycle detection.
	if visited[name] {
		return nil, fmt.Errorf("circular role reference detected: %s", name)
	}
	visited[name] = true

	entries, ok := config[name]
	if !ok {
		return nil, fmt.Errorf("undefined role reference: %s", name)
	}

	scopes := make(map[string]bool, len(entries))
	for _, entry := range entries {
		// Wildcard is kept as-is.
		if entry == "*" {
			scopes["*"] = true
			continue
		}

		// If entry matches another role name, expand it.
		if _, isRole := config[entry]; isRole {
			expanded, err := resolveRole(entry, config, visited, cache)
			if err != nil {
				return nil, err
			}
			for s := range expanded {
				scopes[s] = true
			}
		} else if isRoleReference(entry) {
			// Looks like a role reference but doesn't exist.
			return nil, fmt.Errorf("undefined role reference: %s", entry)
		} else {
			scopes[entry] = true
		}
	}

	cache[name] = scopes
	delete(visited, name)
	return scopes, nil
}

// isRoleReference returns true if the entry looks like a role name rather than
// a scope. Scopes use "resource:action" format (contain ":"), while role
// references are plain identifiers without colons.
func isRoleReference(entry string) bool {
	return !strings.Contains(entry, ":")
}
