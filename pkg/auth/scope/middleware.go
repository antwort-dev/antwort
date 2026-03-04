package scope

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/rhuss/antwort/pkg/auth"
)

// auditLogger defines the interface for audit event emission.
// This avoids an import cycle between auth/scope and audit packages.
type auditLogger interface {
	Log(ctx context.Context, event string, attrs ...any)
	LogWarn(ctx context.Context, event string, attrs ...any)
}

// noopAuditLogger is a no-op implementation used when no audit logger is provided.
type noopAuditLogger struct{}

func (noopAuditLogger) Log(context.Context, string, ...any)     {}
func (noopAuditLogger) LogWarn(context.Context, string, ...any) {}

// DefaultEndpointScopes maps HTTP method + path pattern to required scope.
// Path parameters are represented as segments starting with "{".
var DefaultEndpointScopes = map[string]string{
	"POST /v1/responses":                "responses:create",
	"GET /v1/responses":                 "responses:read",
	"GET /v1/responses/{id}":            "responses:read",
	"GET /v1/responses/{id}/input_items": "responses:read",
	"DELETE /v1/responses/{id}":         "responses:delete",
	"POST /v1/conversations":            "conversations:create",
	"GET /v1/conversations":             "conversations:read",
	"GET /v1/conversations/{id}":        "conversations:read",
	"DELETE /v1/conversations/{id}":     "conversations:delete",
	"GET /v1/conversations/{id}/items":  "conversations:read",
	"POST /v1/conversations/{id}/items": "conversations:write",
	"POST /v1/vector_stores":            "vector_stores:create",
	"GET /v1/vector_stores":             "vector_stores:read",
	"GET /v1/vector_stores/{id}":        "vector_stores:read",
	"DELETE /v1/vector_stores/{id}":     "vector_stores:delete",
	"POST /v1/files":                    "files:create",
	"GET /v1/files":                     "files:read",
	"GET /v1/files/{id}":                "files:read",
	"DELETE /v1/files/{id}":             "files:delete",
	"GET /v1/agents":                    "agents:read",
}

// endpointPattern is a compiled pattern for matching request paths.
type endpointPattern struct {
	method   string
	segments []string // path segments; segments starting with "{" match any value
	scope    string
}

// Middleware creates HTTP middleware that enforces scope-based authorization.
// If expandedRoles is nil or empty, all requests pass through (no enforcement).
// The endpointScopes map defines which scope is required for each endpoint pattern.
func Middleware(expandedRoles map[string]map[string]bool, endpointScopes map[string]string, loggers ...auditLogger) func(http.Handler) http.Handler {
	var al auditLogger = noopAuditLogger{}
	if len(loggers) > 0 && loggers[0] != nil {
		al = loggers[0]
	}

	if len(expandedRoles) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	// Pre-compile endpoint patterns for efficient matching.
	patterns := compilePatterns(endpointScopes)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity := auth.IdentityFromContext(r.Context())
			if identity == nil {
				// No identity means unauthenticated; pass through
				// (auth middleware handles rejection).
				next.ServeHTTP(w, r)
				return
			}

			// Find required scope for this endpoint.
			requiredScope := matchEndpoint(r.Method, r.URL.Path, patterns)
			if requiredScope == "" {
				// No scope requirement for this endpoint.
				next.ServeHTTP(w, r)
				return
			}

			// Compute effective scopes: identity scopes + role-expanded scopes.
			effectiveScopes := computeEffectiveScopes(identity, expandedRoles)

			// Wildcard grants access to everything.
			if effectiveScopes["*"] {
				next.ServeHTTP(w, r)
				return
			}

			// Check if required scope is in effective scopes.
			if effectiveScopes[requiredScope] {
				next.ServeHTTP(w, r)
				return
			}

			// Forbidden.
			al.LogWarn(r.Context(), "authz.scope_denied",
				"endpoint", r.Method+" "+r.URL.Path,
				"required_scope", requiredScope,
				"effective_scopes", formatScopes(effectiveScopes),
			)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, `{"error":{"type":"forbidden","message":"insufficient scope: requires %s"}}`, requiredScope)
		})
	}
}

// compilePatterns converts the endpoint scope map into compiled patterns.
func compilePatterns(endpointScopes map[string]string) []endpointPattern {
	patterns := make([]endpointPattern, 0, len(endpointScopes))
	for key, scope := range endpointScopes {
		parts := strings.SplitN(key, " ", 2)
		if len(parts) != 2 {
			continue
		}
		method := parts[0]
		path := parts[1]
		segments := strings.Split(strings.Trim(path, "/"), "/")
		patterns = append(patterns, endpointPattern{
			method:   method,
			segments: segments,
			scope:    scope,
		})
	}
	return patterns
}

// matchEndpoint finds the required scope for a request method and path.
// Returns empty string if no pattern matches.
func matchEndpoint(method, path string, patterns []endpointPattern) string {
	pathSegments := strings.Split(strings.Trim(path, "/"), "/")

	for _, p := range patterns {
		if p.method != method {
			continue
		}
		if len(p.segments) != len(pathSegments) {
			continue
		}
		match := true
		for i, seg := range p.segments {
			if strings.HasPrefix(seg, "{") {
				// Wildcard segment, matches anything.
				continue
			}
			if seg != pathSegments[i] {
				match = false
				break
			}
		}
		if match {
			return p.scope
		}
	}
	return ""
}

// computeEffectiveScopes returns the union of identity scopes and
// role-expanded scopes from the identity's metadata roles.
func computeEffectiveScopes(identity *auth.Identity, expandedRoles map[string]map[string]bool) map[string]bool {
	effective := make(map[string]bool)

	// Add direct scopes from identity.
	for _, s := range identity.Scopes {
		effective[s] = true
	}

	// Add scopes from expanded roles.
	if identity.Metadata != nil {
		rolesStr := identity.Metadata["roles"]
		if rolesStr != "" {
			for _, role := range strings.Split(rolesStr, ",") {
				role = strings.TrimSpace(role)
				if roleScopes, ok := expandedRoles[role]; ok {
					for s := range roleScopes {
						effective[s] = true
					}
				}
			}
		}
	}

	return effective
}

// formatScopes converts a scope set to a comma-separated string for logging.
func formatScopes(scopes map[string]bool) string {
	parts := make([]string, 0, len(scopes))
	for s := range scopes {
		parts = append(parts, s)
	}
	return strings.Join(parts, ",")
}
