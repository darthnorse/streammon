package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders_CSPPresent(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("expected Content-Security-Policy header to be set")
	}
	for _, want := range []string{
		"default-src 'self'",
		"script-src 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		// The Plex sign-in flow fetches plex.tv directly from the browser
		// (web/src/lib/plexOAuth.ts), so connect-src must allow it.
		"connect-src 'self' https://plex.tv",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("expected CSP to contain %q, got %q", want, csp)
		}
	}
	// script-src must never regress to allowing inline scripts.
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Error("script-src should not allow unsafe-inline")
	}
}

// TestRateLimit_ExceededReturnsJSONError verifies the search rate limiter's
// 429 body is JSON (via writeError), not http.Error's default text/plain.
// The bucket for a test-only IP is pre-filled directly rather than firing 30
// real requests, and rate-limited via a dedicated fake IP so it can't be
// polluted by (or pollute) other tests sharing the global searchRateLimiter.
func TestRateLimit_ExceededReturnsJSONError(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	const ip = "203.0.113.99" // TEST-NET-3 (RFC 5737): reserved for docs/testing
	for i := 0; i < 30; i++ {
		searchRateLimiter.allow(ip)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/maintenance/exclusions?search=x", nil)
	req.RemoteAddr = ip + ":12345"
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("body is not valid JSON: %v (body=%s)", err, w.Body.String())
	}
	if body["error"] == "" {
		t.Errorf("expected non-empty error field, got %v", body)
	}
}

func TestSecurityHeaders_AppliedToAllResponses(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent-path", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("expected CSP header even on non-API/404 responses")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("expected X-Frame-Options: DENY, got %q", w.Header().Get("X-Frame-Options"))
	}
}
