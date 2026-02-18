package storage

import "context"

// tenantKey is a private type for the tenant context key, preventing
// collisions with other packages (per Constitution VIII).
type tenantKey struct{}

// SetTenant injects a tenant identifier into the context.
func SetTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenantID)
}

// GetTenant extracts the tenant identifier from the context.
// Returns an empty string if no tenant is set (single-tenant mode).
func GetTenant(ctx context.Context) string {
	if v, ok := ctx.Value(tenantKey{}).(string); ok {
		return v
	}
	return ""
}
