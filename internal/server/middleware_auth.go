package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
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

// expectedAPIKeyLength is the full length of a valid plaintext key:
// the prefix plus 32 bytes hex-encoded.
const expectedAPIKeyLength = len(auth.APIKeyPrefix) + 64

// RequireAuthManager creates auth middleware using the auth.Manager.
// Two paths:
//  1. X-API-Key header → hash-compared against the stored API key. On match,
//     a synthetic admin user is injected (no DB lookup). Mismatch is 401 and
//     bumps the global auth rate limiter — does not fall through to cookies.
//  2. No header → existing session-cookie path.
//
// SECURITY: No fallback to default admin - auth is always required.
func RequireAuthManager(mgr *auth.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use Values, not Get: an explicitly-empty header (e.g. "X-API-Key: ")
			// must reject + rate-limit, not silently fall through to cookie auth.
			if vals := r.Header.Values("X-API-Key"); len(vals) > 0 {
				ip := rawClientIP(r)

				// Rate-limit before doing any work on attacker-controlled input.
				if !globalAuthRateLimiter.check(ip) {
					w.Header().Set("Retry-After", "900")
					http.Error(w, `{"error":"too many auth attempts, try again later"}`, http.StatusTooManyRequests)
					return
				}

				// Defense-in-depth: API-key acceptance only after setup is complete.
				if required, err := mgr.IsSetupRequired(); err != nil || required {
					globalAuthRateLimiter.record(ip)
					http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
					return
				}

				// Reject duplicates and malformed inputs before hashing — bounds work
				// on attacker input and disambiguates intent.
				if len(vals) > 1 || !validAPIKeyShape(vals[0]) {
					globalAuthRateLimiter.record(ip)
					http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
					return
				}

				apiKey := vals[0]
				stored, err := mgr.Store().GetAPIKey()
				if err != nil || !auth.CompareAPIKey(stored, apiKey) {
					globalAuthRateLimiter.record(ip)
					http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
					return
				}
				ctx := context.WithValue(r.Context(), userContextKey, syntheticAPIKeyUser())
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

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

			_ = mgr.Store().UpdateSessionActivity(cookie.Value)

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// validAPIKeyShape returns true iff the key matches the documented format
// (prefix + 64 hex chars). Cheap pre-check that bounds the SHA-256 work done
// on attacker-controlled input.
func validAPIKeyShape(s string) bool {
	if len(s) != expectedAPIKeyLength {
		return false
	}
	return strings.HasPrefix(s, auth.APIKeyPrefix)
}

// syntheticAPIKeyUser is the in-memory admin principal used for X-API-Key
// requests. ID=-1 is a sentinel; this user is never written to or looked up
// in the DB, so it cannot collide with any real row.
func syntheticAPIKeyUser() *models.User {
	return &models.User{
		ID:         -1,
		Name:       "api",
		Role:       models.RoleAdmin,
		APIKeyAuth: true,
	}
}

// RequireInteractiveSession rejects requests authenticated via X-API-Key.
// Apply to handlers that mutate the caller's own user record, manage the API
// key itself, or otherwise only make sense for a real human session.
func RequireInteractiveSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil || user.APIKeyAuth {
			http.Error(w, `{"error":"interactive session required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
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

func filterValid(attempts []time.Time, cutoff time.Time) []time.Time {
	valid := attempts[:0]
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	return valid
}

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

// rateLimitAuthWith applies the failed-attempt limiter keyed by keyFn(r).
// Only failed attempts (4xx/5xx responses) count toward the limit.
func rateLimitAuthWith(keyFn func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := keyFn(r)

		if !globalAuthRateLimiter.check(key) {
			// Log the raw peer + path only — never the attacker-controlled
			// username portion of the key, to avoid log injection.
			log.Printf("auth rate limit: ip=%s path=%s", rawClientIP(r), r.URL.Path)
			w.Header().Set("Retry-After", "900") // 15 minutes
			http.Error(w, `{"error":"too many login attempts, try again later"}`, http.StatusTooManyRequests)
			return
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		// Only count failed attempts (4xx/5xx)
		if rec.status >= 400 {
			globalAuthRateLimiter.record(key)
		}
	})
}

// RateLimitAuth applies IP-only rate limiting to authentication endpoints that
// carry no login username (setup, OIDC, password-change, API-key rotate).
// Keys on the raw socket peer captured by CaptureRawRemoteAddr so a spoofed
// X-Forwarded-For (already applied by middleware.RealIP) cannot rotate the
// limiter's bucket.
func RateLimitAuth(next http.Handler) http.Handler {
	return rateLimitAuthWith(func(r *http.Request) string { return rawClientIP(r) }, next)
}

// RateLimitLogin applies rate limiting to credential-login endpoints, scoped by
// "<rawClientIP>|<username>". Behind a reverse proxy every user shares one
// socket peer, so IP-only keying lets one bad actor's failed logins lock out
// all users. Scoping by the submitted account identifier confines a flood to
// the targeted account while still keying on the raw socket peer (not a
// spoofable X-Forwarded-For) for the IP portion. Falls back to IP-only when no
// username is present in the body (e.g. Plex token login).
func RateLimitLogin(next http.Handler) http.Handler {
	return rateLimitAuthWith(loginRateLimitKey, next)
}

// loginRateLimitKey builds the per-account limiter bucket key for a login
// request: "<ip>|<username>" when a username is submitted, else IP-only.
func loginRateLimitKey(r *http.Request) string {
	ip := rawClientIP(r)
	username := extractLoginUsername(r)
	if username == "" {
		return ip
	}
	return ip + "|" + username
}

// extractLoginUsername peeks the JSON request body for a "username" field so
// the login limiter can scope per account. The body is read in full and then
// restored via io.NopCloser so the downstream login handler can still decode
// it. Only the username is unmarshalled — the password (and any other field) is
// never read into a variable or logged. Returns "" (IP-only scoping) when the
// body is empty, unreadable, or carries no username.
func extractLoginUsername(r *http.Request) string {
	if r.Body == nil || r.Body == http.NoBody {
		return ""
	}
	buf, err := io.ReadAll(r.Body)
	// Restore the body unconditionally so the handler sees the original bytes,
	// even if the read hit the MaxBytesReader limit installed by limitBody.
	r.Body = io.NopCloser(bytes.NewReader(buf))
	if err != nil {
		return ""
	}
	var creds struct {
		Username string `json:"username"`
	}
	if json.Unmarshal(buf, &creds) != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(creds.Username))
}
