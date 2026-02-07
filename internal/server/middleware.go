package server

import (
	"net/http"
	"sync"
	"time"
)

const maxBodySize = 1 << 20 // 1MB

// rateLimiter implements a per-IP sliding window rate limiter
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
	done     chan struct{}
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
	close(rl.done)
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

	// Filter to only requests within window
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
		// Only rate limit GET requests with search parameter
		if r.Method == http.MethodGet && r.URL.Query().Get("search") != "" {
			if !searchRateLimiter.allow(r.RemoteAddr) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
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
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
