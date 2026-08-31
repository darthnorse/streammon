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
// embeddedExists reports whether the built frontend actually contains p. The
// image cases below assert a max-age tier, and a missing file is rewritten to
// the SPA shell (no-cache) rather than 404ing -- so without this they would
// FAIL, not skip, on a checkout where web/dist was never built.
func embeddedExists(p string) bool {
	f, err := webFS.Open("web/dist/" + strings.TrimPrefix(p, "/"))
	if err != nil {
		return false
	}
	f.Close()
	return true
}

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
		name      string
		path      string
		want      string
		needsFile bool // asserts a non-default tier, so the file must really exist
	}{
		{"index shell", "/", "no-cache", false},
		{"spa deep link falls back to the shell", "/discover/trending", "no-cache", false},
		{"content-hashed asset", "", "public, max-age=31536000, immutable", false}, // path resolved per-subtest
		{"missing asset falls back to the shell", "/assets/does-not-exist-abc123.js", "no-cache", false},
		{"favicon is cacheable art", "/favicon.ico", "public, max-age=86400", true},
		{"media flag sprite is cacheable art", "/media-flags/audio_codec/aac.png", "public, max-age=86400", true},
		// Unhashed but release-coupled: caching these re-creates the stale-bundle bug.
		{"theme bootstrap must revalidate", "/theme-init.js", "no-cache", false},
		{"webmanifest must revalidate", "/site.webmanifest", "no-cache", false},
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
			if tt.needsFile && !embeddedExists(path) {
				t.Skip("frontend not built; file is not embedded")
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

// isStaticImage decides the cache tier, so it is worth pinning independently of
// whether a frontend build happens to be embedded in this test binary.
func TestIsStaticImage(t *testing.T) {
	cacheable := []string{
		"/favicon.ico", "/apple-touch-icon.png", "/tmdb-logo.svg",
		"/media-flags/audio_codec/aac.png", "/UPPER.PNG", "/a.jpeg", "/a.webp", "/a.gif",
	}
	revalidate := []string{
		"/", "/index.html", "/theme-init.js", "/site.webmanifest",
		"/discover/trending", "/media-flags/ATTRIBUTION.md", "/noextension",
	}

	for _, p := range cacheable {
		if !isStaticImage(p) {
			t.Errorf("isStaticImage(%q) = false, want true", p)
		}
	}
	for _, p := range revalidate {
		if isStaticImage(p) {
			t.Errorf("isStaticImage(%q) = true, want false", p)
		}
	}
}
