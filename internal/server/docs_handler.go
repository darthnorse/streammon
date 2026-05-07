package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:docs
var docsFS embed.FS

// registerDocsRoutes wires the public, unauthenticated documentation endpoints.
// Must be called BEFORE the SPA catch-all so the SPA NotFound handler doesn't
// preempt these paths.
func (s *Server) registerDocsRoutes() {
	subFS, err := fs.Sub(docsFS, "docs")
	if err != nil {
		panic(err)
	}

	// /openapi.yaml — convenience top-level alias.
	s.router.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		serveDocsFile(w, r, subFS, "openapi.yaml", "application/yaml; charset=utf-8")
	})

	// /docs — Redoc loader page.
	s.router.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		serveDocsFile(w, r, subFS, "index.html", "text/html; charset=utf-8")
	})

	// /docs/* — anything else under the docs/ dir (redoc.standalone.js, openapi.yaml mirror).
	s.router.Get("/docs/*", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/docs/")
		if name == "" {
			name = "index.html"
		}
		ct := ""
		switch {
		case strings.HasSuffix(name, ".js"):
			ct = "application/javascript; charset=utf-8"
		case strings.HasSuffix(name, ".yaml"):
			ct = "application/yaml; charset=utf-8"
		case strings.HasSuffix(name, ".html"):
			ct = "text/html; charset=utf-8"
		}
		serveDocsFile(w, r, subFS, name, ct)
	})
}

func serveDocsFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name, contentType string) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
