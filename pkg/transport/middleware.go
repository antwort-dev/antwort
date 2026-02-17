package transport

import "context"

// Middleware wraps a ResponseCreator to add cross-cutting behavior.
// Middleware is applied in order: the first middleware in the chain is
// the outermost wrapper (executes first on the way in, last on the way out).
type Middleware func(ResponseCreator) ResponseCreator

// Chain composes multiple middleware into a single middleware.
// Middleware are applied in order: Chain(a, b, c) produces a(b(c(handler))).
func Chain(middlewares ...Middleware) Middleware {
	return func(next ResponseCreator) ResponseCreator {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// requestIDKeyType is the context key type for request IDs.
type requestIDKeyType struct{}

// requestIDKey is the context key for storing and retrieving request IDs.
var requestIDKey = requestIDKeyType{}

// RequestIDFromContext extracts the request ID from the context.
// Returns an empty string if no request ID is set.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// ContextWithRequestID returns a new context with the given request ID.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}
