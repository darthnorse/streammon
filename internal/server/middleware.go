package server

import (
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

const maxBodySize = 1 << 20 // 1MB

type rawRemoteAddrKey struct{}

// CaptureRawRemoteAddr stashes r.RemoteAddr in the request context before any
// later middleware (notably middleware.RealIP) can rewrite it from
// X-Forwarded-For. Rate limiters that must key on the actual socket peer should
// read the captured value via rawClientIP.
func CaptureRawRemoteAddr(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), rawRemoteAddrKey{}, r.RemoteAddr)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// rawClientIP returns the host portion of the original socket peer captured by
// CaptureRawRemoteAddr. Falls back to r.RemoteAddr (which may have been
// rewritten by middleware.RealIP) if the capture middleware is not installed.
func rawClientIP(r *http.Request) string {
	addr, ok := r.Context().Value(rawRemoteAddrKey{}).(string)
	if !ok {
		addr = r.RemoteAddr
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
	done     chan struct{}
	stopOnce sync.Once
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		done:     make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) stop() {
	rl.stopOnce.Do(func() {
		close(rl.done)
	})
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-rl.done:
			return
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now().UTC()
			for ip, times := range rl.requests {
				valid := times[:0]
				for _, t := range times {
					if now.Sub(t) <= rl.window {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(rl.requests, ip)
				} else {
					rl.requests[ip] = valid
				}
			}
			rl.mu.Unlock()
		}
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now().UTC()
	times := rl.requests[ip]

	valid := times[:0]
	for _, t := range times {
		if now.Sub(t) <= rl.window {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[ip] = valid
		return false
	}

	rl.requests[ip] = append(valid, now)
	return true
}

// Global rate limiter for search endpoints: 30 requests per minute per IP
var searchRateLimiter = newRateLimiter(30, time.Minute)

// StopRateLimiter stops the global rate limiter's cleanup goroutine.
// Call this during server shutdown.
func StopRateLimiter() {
	searchRateLimiter.stop()
}

func rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Query().Get("search") != "" {
			ip := rawClientIP(r)
			if !searchRateLimiter.allow(ip) {
				log.Printf("search rate limit: ip=%s path=%s", ip, r.URL.Path)
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		next.ServeHTTP(w, r)
	})
}

func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// contentSecurityPolicy is deliberately conservative rather than exhaustively
// scoped to every third-party host currently in use:
//   - script-src has no 'unsafe-inline' — the only inline script (theme
//     bootstrap in index.html) was moved to a same-origin file
//     (web/public/theme-init.js) specifically so this can stay strict.
//   - style-src allows 'unsafe-inline' because Tailwind/React and the
//     Leaflet/Recharts dependencies set inline style attributes at runtime;
//     auditing every call site was impractical, and style injection is a
//     much lower-severity primitive than script injection.
//   - img-src allows any https: host (plus data: and self) rather than an
//     allowlist, since posters/avatars are fetched from several third
//     parties (TMDB, Plex.tv, Overseerr/Gravatar avatars) and map tiles
//     from *.basemaps.cartocdn.com — all already same-origin-proxied or
//     https, so a broad https: allowance doesn't add new risk beyond what
//     <img> tags could already load.
//   - connect-src/frame-ancestors/etc. are locked to 'self' since the SPA
//     only talks to its own backend (REST + SSE), never third-party APIs
//     directly from the browser.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: https:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		next.ServeHTTP(w, r)
	})
}
