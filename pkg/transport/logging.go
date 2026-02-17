package transport

import (
	"context"
	"log/slog"
	"time"

	"github.com/rhuss/antwort/pkg/api"
)

// Logging returns middleware that emits structured log entries for each
// request. The log entry includes method (POST), path (/v1/responses),
// duration, request ID (from context), and whether the request succeeded
// or failed.
//
// Note: The HTTP method and path are not available at the ResponseCreator
// level. This middleware logs at the handler level. For full HTTP-level
// logging (including status codes), use HTTP-level middleware in the adapter.
func Logging(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next ResponseCreator) ResponseCreator {
		return ResponseCreatorFunc(func(ctx context.Context, req *api.CreateResponseRequest, w ResponseWriter) error {
			start := time.Now()
			requestID := RequestIDFromContext(ctx)

			err := next.CreateResponse(ctx, req, w)

			attrs := []slog.Attr{
				slog.String("request_id", requestID),
				slog.String("model", req.Model),
				slog.Bool("stream", req.Stream),
				slog.Duration("duration", time.Since(start)),
			}

			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
				logger.LogAttrs(ctx, slog.LevelError, "request failed", attrs...)
			} else {
				logger.LogAttrs(ctx, slog.LevelInfo, "request completed", attrs...)
			}

			return err
		})
	}
}
