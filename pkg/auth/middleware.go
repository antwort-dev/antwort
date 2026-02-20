package auth

import (
	"log/slog"
	"net/http"

	"github.com/rhuss/antwort/pkg/observability"
	"github.com/rhuss/antwort/pkg/storage"
)

// Middleware creates HTTP middleware from an AuthChain and optional RateLimiter.
// It checks the bypass list, runs authentication, injects tenant context,
// and optionally enforces rate limits.
func Middleware(chain *AuthChain, limiter RateLimiter, bypassEndpoints []string) func(http.Handler) http.Handler {
	bypass := make(map[string]bool, len(bypassEndpoints))
	for _, ep := range bypassEndpoints {
		bypass[ep] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check bypass list.
			if bypass[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Run auth chain.
			result := chain.Authenticate(r.Context(), r)

			if result.Decision == No {
				slog.Warn("authentication failed",
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
					"error", result.Err,
				)
				http.Error(w, `{"error":{"type":"invalid_request","message":"authentication required"}}`, http.StatusUnauthorized)
				return
			}

			if result.Decision != Yes || result.Identity == nil {
				http.Error(w, `{"error":{"type":"invalid_request","message":"authentication required"}}`, http.StatusUnauthorized)
				return
			}

			// Validate identity.
			if result.Identity.Subject == "" {
				slog.Error("authenticator returned identity with empty subject")
				http.Error(w, `{"error":{"type":"server_error","message":"internal authentication error"}}`, http.StatusInternalServerError)
				return
			}

			slog.Debug("authentication succeeded",
				"subject", result.Identity.Subject,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)

			// Rate limiting (if configured).
			if limiter != nil {
				if err := limiter.Allow(r.Context(), result.Identity); err != nil {
					slog.Warn("rate limit exceeded",
						"subject", result.Identity.Subject,
						"tier", result.Identity.ServiceTier,
					)
					observability.RateLimitRejectedTotal.WithLabelValues(result.Identity.ServiceTier).Inc()
					http.Error(w, `{"error":{"type":"too_many_requests","message":"rate limit exceeded"}}`, http.StatusTooManyRequests)
					return
				}
			}

			// Inject identity into context.
			ctx := SetIdentity(r.Context(), result.Identity)

			// Inject tenant for storage scoping.
			if tenantID := result.Identity.TenantID(); tenantID != "" {
				ctx = storage.SetTenant(ctx, tenantID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DefaultBypassEndpoints lists endpoints that skip authentication.
var DefaultBypassEndpoints = []string{"/healthz", "/readyz", "/metrics"}
