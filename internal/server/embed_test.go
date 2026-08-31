package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// realAsset returns a content-hashed asset actually present in the embedded
// build, so the test does not pin a hash that changes every build.
func realAsset(t *testing.T) string {
	t.Helper()
	entries, err := fs.ReadDir(webFS, "web/dist/assets")
	if err != nil {
		t.Skipf("no embedded assets dir (frontend not built): %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
			return "/assets/" + e.Name()
		}
	}
	t.Skip("no embedded .js asset found")
	return ""
}

// The SPA shell must revalidate on every load: it names the content-hashed
// bundle, so a cached copy pins the browser to a stale frontend across an
// upgrade. The hashed assets it names can be cached forever.
func TestServeSPACacheHeaders(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	tests := []struct {
		name string
		path string
		want string
	}{
		{"index shell", "/", "no-cache"},
		{"spa deep link falls back to the shell", "/discover/trending", "no-cache"},
		{"content-hashed asset", realAsset(t), "public, max-age=31536000, immutable"},
		{"missing asset falls back to the shell", "/assets/does-not-exist-abc123.js", "no-cache"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if got := w.Header().Get("Cache-Control"); got != tt.want {
				t.Errorf("Cache-Control for %s = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
