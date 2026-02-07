package server

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"streammon/internal/auth"
	"streammon/internal/models"
)

type contextKey string

const userContextKey contextKey = "user"

func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userContextKey).(*models.User)
	return u
}

// RequireAuthManager creates auth middleware using the auth.Manager
// SECURITY: No fallback to default admin - auth is always required
func RequireAuthManager(mgr *auth.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(auth.CookieName)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			user, err := mgr.Store().GetSessionUser(cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// Update session activity
			_ = mgr.Store().UpdateSessionActivity(cookie.Value)

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireSetup allows access only when setup is required (no users exist)
func RequireSetup(mgr *auth.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			required, err := mgr.IsSetupRequired()
			if err != nil {
				http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
				return
			}
			if !required {
				http.Error(w, `{"error":"setup already complete"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// testAdmin is used only when auth is disabled (test environments).
// SECURITY: This should never be used in production. Production deployments
// must use RequireAuthManager with a properly configured auth.Manager.
var testAdmin = &models.User{
	Name: "test-admin",
	Role: models.RoleAdmin,
}

// RequireAuth is kept for backward compatibility with legacy auth.Service.
// SECURITY NOTE: When auth is disabled (svc.Enabled() == false), a test admin
// user is injected. This is intentional for test environments only.
// Production deployments should use RequireAuthManager which doesn't have
// this fallback behavior.
func RequireAuth(svc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// When auth is disabled, inject test admin for backward compatibility.
			// SECURITY: This path should only be used in tests.
			if !svc.Enabled() {
				ctx := context.WithValue(r.Context(), userContextKey, testAdmin)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			cookie, err := r.Cookie(auth.CookieName)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			user, err := svc.Store().GetSessionUser(cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(role models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil || user.Role != role {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// authRateLimiter tracks login attempts per IP
type authRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func newAuthRateLimiter(limit int, window time.Duration) *authRateLimiter {
	rl := &authRateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	// Start background cleanup goroutine to prevent memory growth
	go rl.cleanupLoop()
	return rl
}

// cleanupLoop periodically removes stale entries from the rate limiter
func (l *authRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		l.cleanup()
	}
}

// cleanup removes all expired entries
func (l *authRateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	for ip, attempts := range l.attempts {
		valid := attempts[:0]
		for _, t := range attempts {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(l.attempts, ip)
		} else {
			l.attempts[ip] = valid
		}
	}
}

func (l *authRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// Clean old entries
	attempts := l.attempts[ip]
	valid := attempts[:0]
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.limit {
		l.attempts[ip] = valid
		return false
	}

	l.attempts[ip] = append(valid, now)
	return true
}

// Global rate limiter for auth endpoints: 10 attempts per 15 minutes
var globalAuthRateLimiter = newAuthRateLimiter(10, 15*time.Minute)

// RateLimitAuth applies rate limiting to authentication endpoints
// NOTE: Uses RemoteAddr only. If behind a trusted reverse proxy that sets
// X-Forwarded-For, configure the proxy to set RemoteAddr correctly instead.
func RateLimitAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use RemoteAddr directly - don't trust X-Forwarded-For which can be spoofed.
		// For deployments behind reverse proxies, configure the proxy to strip
		// client-provided X-Forwarded-For headers and set RemoteAddr correctly.
		ip := r.RemoteAddr

		// Strip port if present
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}

		if !globalAuthRateLimiter.allow(ip) {
			w.Header().Set("Retry-After", "900") // 15 minutes
			http.Error(w, `{"error":"too many login attempts, try again later"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
