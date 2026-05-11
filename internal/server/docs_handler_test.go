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

func TestEtagMatches(t *testing.T) {
	const etag = `"abc"`
	cases := []struct {
		name      string
		headerVal string
		want      bool
	}{
		{"empty header", "", false},
		{"single match", `"abc"`, true},
		{"single mismatch", `"def"`, false},
		{"comma list with match", `"def", "abc"`, true},
		{"comma list with whitespace", `  "abc"  ,  "def"  `, true},
		{"comma list no match", `"def", "ghi"`, false},
		{"weak validator match", `W/"abc"`, true},
		{"weak validator inside list", `"old", W/"abc"`, true},
		{"wildcard", `*`, true},
		{"wildcard inside list", `"old", *`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := etagMatches(tc.headerVal, etag); got != tc.want {
				t.Errorf("etagMatches(%q, %q) = %v, want %v", tc.headerVal, etag, got, tc.want)
			}
		})
	}
}

func TestCacheControlFor(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"redoc.standalone.js", "public, max-age=2592000, immutable"},
		{"openapi.yaml", "public, max-age=300, must-revalidate"},
		{"index.html", "public, max-age=300, must-revalidate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cacheControlFor(tc.name); got != tc.want {
				t.Errorf("cacheControlFor(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestDocs_IfNoneMatchReturns304(t *testing.T) {
	srv, _ := newTestServer(t)

	// First request — capture ETag.
	req := httptest.NewRequest(http.MethodGet, "/docs", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("first response missing ETag")
	}

	// Second request with If-None-Match → 304.
	req2 := httptest.NewRequest(http.MethodGet, "/docs", nil)
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotModified {
		t.Errorf("expected 304 with matching If-None-Match, got %d", w2.Code)
	}
	if w2.Body.Len() != 0 {
		t.Errorf("304 body must be empty, got %d bytes", w2.Body.Len())
	}

	// Third request with non-matching If-None-Match → 200.
	req3 := httptest.NewRequest(http.MethodGet, "/docs", nil)
	req3.Header.Set("If-None-Match", `"stale"`)
	w3 := httptest.NewRecorder()
	srv.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("expected 200 with stale If-None-Match, got %d", w3.Code)
	}
}
