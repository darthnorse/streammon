package server

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:web/dist
var webFS embed.FS

// isStaticImage reports whether the path is release-independent artwork that is
// safe to cache without a content hash in its name.
func isStaticImage(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".png", ".ico", ".svg", ".jpg", ".jpeg", ".webp", ".gif":
		return true
	}
	return false
}

func (s *Server) serveSPA() {
	subFS, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(subFS))

	s.router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.NotFound(w, r)
			return
		}
		// Try serving the file directly; fall back to index.html for SPA routing
		f, err := subFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil {
			r.URL.Path = "/"
		} else {
			f.Close()
		}

		// embed.FS carries no modtimes, so http.FileServer emits neither
		// Last-Modified nor ETag. With no validator and no Cache-Control,
		// caches fall back to heuristic freshness and can pin a client to a
		// stale index.html -- and therefore to the previous frontend bundle --
		// across an upgrade. Checked after the SPA rewrite above so a missing
		// asset, now serving the shell, is not marked immutable.
		switch {
		case strings.HasPrefix(r.URL.Path, "/assets/"):
			// Vite content-hashes these names, so a changed file is a new URL.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case isStaticImage(r.URL.Path):
			// Icons and the media-flag sprites are not content-hashed, so they
			// cannot be immutable, but they are stable art rather than release
			// artifacts. Without this they re-download on every page load: no
			// validator means revalidation can never answer 304.
			w.Header().Set("Cache-Control", "public, max-age=86400")
		default:
			// index.html, theme-init.js and site.webmanifest are unhashed AND
			// change with a release, so they must revalidate every load.
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
}
