package server

import (
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
