package auth

import "context"

// identityKey is a private type for the identity context key.
type identityKey struct{}

// SetIdentity stores the authenticated identity in the context.
func SetIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// IdentityFromContext retrieves the authenticated identity.
// Returns nil if no identity is set (unauthenticated or NoOp).
func IdentityFromContext(ctx context.Context) *Identity {
	if v, ok := ctx.Value(identityKey{}).(*Identity); ok {
		return v
	}
	return nil
}
