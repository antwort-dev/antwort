package transport

import (
	"context"
	"fmt"

	"github.com/rhuss/antwort/pkg/api"
)

// Recovery returns middleware that catches panics in the handler and
// converts them to server error responses. The server continues to
// accept new requests after a panic is recovered.
func Recovery() Middleware {
	return func(next ResponseCreator) ResponseCreator {
		return ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) (retErr error) {
			defer func() {
				if r := recover(); r != nil {
					retErr = api.NewServerError(fmt.Sprintf("internal server error: %v", r))
				}
			}()
			return next.CreateResponse(ctx, req, w)
		})
	}
}
