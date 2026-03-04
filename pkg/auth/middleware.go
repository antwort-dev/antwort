package auth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/rhuss/antwort/pkg/observability"
	"github.com/rhuss/antwort/pkg/storage"
)

// AuditLogger defines the interface for audit event emission.
// This avoids an import cycle between auth and audit packages.
// The *audit.Logger type satisfies this interface.
type AuditLogger interface {
	Log(ctx context.Context, event string, attrs ...any)
	LogWarn(ctx context.Context, event string, attrs ...any)
}

// noopAuditLogger is a no-op implementation used when no audit logger is provided.
type noopAuditLogger struct{}

func (noopAuditLogger) Log(context.Context, string, ...any)     {}
func (noopAuditLogger) LogWarn(context.Context, string, ...any) {}

// Middleware creates HTTP middleware from an AuthChain and optional RateLimiter.
// It checks the bypass list, runs authentication, injects tenant and owner context,
// and optionally enforces rate limits.
// The adminRole parameter is used to check if the authenticated user has admin
// privileges. Pass empty string to disable admin detection.
func Middleware(chain *AuthChain, limiter RateLimiter, bypassEndpoints []string, al AuditLogger, adminRole ...string) func(http.Handler) http.Handler {
	if al == nil {
		al = noopAuditLogger{}
	}
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
				al.LogWarn(r.Context(), "auth.failure",
					"auth_method", "unknown",
					"remote_addr", r.RemoteAddr,
					"error", result.Err.Error(),
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
			al.Log(r.Context(), "auth.success",
				"auth_method", determineAuthMethod(result),
				"remote_addr", r.RemoteAddr,
			)

			// Rate limiting (if configured).
			if limiter != nil {
				if err := limiter.Allow(r.Context(), result.Identity); err != nil {
					slog.Warn("rate limit exceeded",
						"subject", result.Identity.Subject,
						"tier", result.Identity.ServiceTier,
					)
					al.LogWarn(r.Context(), "auth.rate_limited",
						"tier", result.Identity.ServiceTier,
						"remote_addr", r.RemoteAddr,
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

			// Inject owner (Identity.Subject) for resource ownership.
			ctx = storage.SetOwner(ctx, result.Identity.Subject)

			// Inject admin flag if an admin role is configured.
			if len(adminRole) > 0 && adminRole[0] != "" {
				ctx = storage.SetAdmin(ctx, IsAdmin(result.Identity, adminRole[0]))
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// DefaultBypassEndpoints lists endpoints that skip authentication.
var DefaultBypassEndpoints = []string{"/healthz", "/readyz", "/metrics"}

// determineAuthMethod returns the authentication method used based on the
// identity metadata. Returns "jwt" if metadata contains JWT-related keys,
// "apikey" if it contains an API key marker, or "unknown" otherwise.
func determineAuthMethod(result AuthResult) string {
	if result.Identity == nil {
		return "unknown"
	}
	if result.Identity.Metadata != nil {
		if _, ok := result.Identity.Metadata["jwt_issuer"]; ok {
			return "jwt"
		}
		if _, ok := result.Identity.Metadata["jwt"]; ok {
			return "jwt"
		}
		if _, ok := result.Identity.Metadata["api_key"]; ok {
			return "apikey"
		}
	}
	// If the identity has scopes (typically from JWT), assume JWT.
	if len(result.Identity.Scopes) > 0 {
		return "jwt"
	}
	return "unknown"
}
