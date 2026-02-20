package mcp

import "context"

// AuthProvider supplies authentication headers for MCP server connections.
type AuthProvider interface {
	// GetHeaders returns the HTTP headers to include in MCP requests.
	GetHeaders(ctx context.Context) (map[string]string, error)
}

// StaticKeyAuth provides authentication via static headers configured
// at initialization time. Suitable for API key authentication.
type StaticKeyAuth struct {
	// Headers contains the static authentication headers.
	Headers map[string]string
}

// GetHeaders returns the configured static headers.
func (a *StaticKeyAuth) GetHeaders(_ context.Context) (map[string]string, error) {
	return a.Headers, nil
}
