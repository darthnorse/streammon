package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:web/dist
var webFS embed.FS

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
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			// Vite content-hashes these names, so a changed file is a new URL.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
}
