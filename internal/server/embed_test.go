package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// realAsset returns a content-hashed asset actually present in the embedded
// build, so the test does not pin a hash that changes every build. It reports
// ok=false rather than skipping: web/dist is gitignored and `make test` has no
// frontend-build step, so skipping here would abort the whole parent test and
// silently drop the cases that need no built asset.
func realAsset() (string, bool) {
	entries, err := fs.ReadDir(webFS, "web/dist/assets")
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".js") {
			return "/assets/" + e.Name(), true
		}
	}
	return "", false
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
		{"content-hashed asset", "", "public, max-age=31536000, immutable"}, // path resolved per-subtest
		{"missing asset falls back to the shell", "/assets/does-not-exist-abc123.js", "no-cache"},
		{"favicon is cacheable art", "/favicon.ico", "public, max-age=86400"},
		{"media flag sprite is cacheable art", "/media-flags/audio_codec/aac.png", "public, max-age=86400"},
		// Unhashed but release-coupled: caching these re-creates the stale-bundle bug.
		{"theme bootstrap must revalidate", "/theme-init.js", "no-cache"},
		{"webmanifest must revalidate", "/site.webmanifest", "no-cache"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path
			if path == "" {
				asset, ok := realAsset()
				if !ok {
					t.Skip("frontend not built; no embedded asset to request")
				}
				path = asset
			}

			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if got := w.Header().Get("Cache-Control"); got != tt.want {
				t.Errorf("Cache-Control for %s = %q, want %q", path, got, tt.want)
			}
		})
	}
}
