package transport

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/rhuss/antwort/pkg/api"
)

// RequestID returns middleware that assigns a unique request ID to each
// request. If the incoming request context already carries a request ID
// (set by the HTTP adapter from the X-Request-ID header), that value is
// used. Otherwise, a new unique ID is generated.
//
// The request ID is stored in the context and can be retrieved with
// RequestIDFromContext.
func RequestID() Middleware {
	return func(next ResponseCreator) ResponseCreator {
		return ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
			id := RequestIDFromContext(ctx)
			if id == "" {
				id = generateRequestID()
				ctx = ContextWithRequestID(ctx, id)
			}
			return next.CreateResponse(ctx, req, w)
		})
	}
}

// generateRequestID creates a new unique request ID as a hex string.
func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
