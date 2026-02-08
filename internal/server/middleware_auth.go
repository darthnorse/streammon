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

// setupCheck creates middleware that checks setup status.
// If requireSetup is true, only allows access when setup is needed.
// If requireSetup is false, only allows access when setup is complete.
func setupCheck(mgr *auth.Manager, requireSetup bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			required, err := mgr.IsSetupRequired()
			if err != nil {
				http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
				return
			}
			if requireSetup && !required {
				http.Error(w, `{"error":"setup already complete"}`, http.StatusForbidden)
				return
			}
			if !requireSetup && required {
				http.Error(w, `{"error":"setup required"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireSetup allows access only when setup is required (no users exist)
func RequireSetup(mgr *auth.Manager) func(http.Handler) http.Handler {
	return setupCheck(mgr, true)
}

// RequireSetupComplete blocks access when setup is still required.
// This prevents login endpoints from being used before an admin is created.
func RequireSetupComplete(mgr *auth.Manager) func(http.Handler) http.Handler {
	return setupCheck(mgr, false)
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
	stopOnce sync.Once
	stopCh   chan struct{}
}

func newAuthRateLimiter(limit int, window time.Duration) *authRateLimiter {
	rl := &authRateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		stopCh:   make(chan struct{}),
	}
	// Start background cleanup goroutine to prevent memory growth
	go rl.cleanupLoop()
	return rl
}

// Stop gracefully shuts down the cleanup goroutine
func (l *authRateLimiter) Stop() {
	l.stopOnce.Do(func() {
		close(l.stopCh)
	})
}

// cleanupLoop periodically removes stale entries from the rate limiter
func (l *authRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.cleanup()
		}
	}
}

// filterValid returns only attempts within the time window
func filterValid(attempts []time.Time, cutoff time.Time) []time.Time {
	valid := attempts[:0]
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	return valid
}

// cleanup removes all expired entries
func (l *authRateLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().UTC().Add(-l.window)
	for ip, attempts := range l.attempts {
		if valid := filterValid(attempts, cutoff); len(valid) == 0 {
			delete(l.attempts, ip)
		} else {
			l.attempts[ip] = valid
		}
	}
}

// check returns true if the IP is under the rate limit (does not increment)
func (l *authRateLimiter) check(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().UTC().Add(-l.window)
	valid := filterValid(l.attempts[ip], cutoff)
	l.attempts[ip] = valid

	return len(valid) < l.limit
}

// record adds a failed attempt for the IP
func (l *authRateLimiter) record(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.attempts[ip] = append(l.attempts[ip], time.Now().UTC())
}

// Global rate limiter for auth endpoints: 10 failed attempts per 15 minutes
var globalAuthRateLimiter = newAuthRateLimiter(10, 15*time.Minute)

// StopAuthRateLimiter stops the background cleanup goroutine for the auth rate limiter.
// Call this during graceful shutdown.
func StopAuthRateLimiter() {
	globalAuthRateLimiter.Stop()
}

// statusRecorder wraps ResponseWriter to capture the status code
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// RateLimitAuth applies rate limiting to authentication endpoints.
// Only failed attempts (4xx/5xx responses) count toward the limit.
// NOTE: Uses RemoteAddr only. If behind a trusted reverse proxy that sets
// X-Forwarded-For, configure the proxy to set RemoteAddr correctly instead.
func RateLimitAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use RemoteAddr directly - don't trust X-Forwarded-For which can be spoofed.
		ip := r.RemoteAddr
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}

		if !globalAuthRateLimiter.check(ip) {
			w.Header().Set("Retry-After", "900") // 15 minutes
			http.Error(w, `{"error":"too many login attempts, try again later"}`, http.StatusTooManyRequests)
			return
		}

		// Wrap response to capture status code
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		// Only count failed attempts (4xx/5xx)
		if rec.status >= 400 {
			globalAuthRateLimiter.record(ip)
		}
	})
}
