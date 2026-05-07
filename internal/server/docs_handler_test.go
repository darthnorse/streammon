package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocs_IndexServesHTML(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type=%q want text/html", ct)
	}
	if !strings.Contains(w.Body.String(), "redoc.standalone.js") {
		t.Error("expected index.html to reference redoc.standalone.js")
	}
}

func TestDocs_OpenAPIServesYAML(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "yaml") {
		t.Errorf("Content-Type=%q want yaml", ct)
	}
	if !strings.Contains(w.Body.String(), "openapi: 3.0.3") {
		t.Error("expected openapi version line in body")
	}
}

func TestDocs_RedocAssetServes(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/docs/redoc.standalone.js", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if w.Body.Len() < 100_000 {
		t.Errorf("redoc bundle too small: %d bytes", w.Body.Len())
	}
}

func TestDocs_PublicNoAuthRequired(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, path := range []string{"/docs", "/openapi.yaml", "/docs/redoc.standalone.js"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		// Note: deliberately NOT setting any auth header / cookie.
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("%s requires auth (status=%d) — should be public", path, w.Code)
		}
	}
}
