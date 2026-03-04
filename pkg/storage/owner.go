package storage

import "context"

// ownerKey is a private type for the owner context key, preventing
// collisions with other packages.
type ownerKey struct{}

// adminKey is a private type for the admin flag context key.
type adminKey struct{}

// SetOwner injects the resource owner identifier into the context.
func SetOwner(ctx context.Context, owner string) context.Context {
	return context.WithValue(ctx, ownerKey{}, owner)
}

// GetOwner extracts the owner identifier from the context.
// Returns an empty string if no owner is set (unauthenticated or NoOp auth).
func GetOwner(ctx context.Context) string {
	if v, ok := ctx.Value(ownerKey{}).(string); ok {
		return v
	}
	return ""
}

// SetAdmin injects the admin flag into the context.
func SetAdmin(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, adminKey{}, isAdmin)
}

// GetAdmin returns whether the caller in context has admin privileges.
func GetAdmin(ctx context.Context) bool {
	if v, ok := ctx.Value(adminKey{}).(bool); ok {
		return v
	}
	return false
}
