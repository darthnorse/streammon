package server

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed all:docs
var docsFS embed.FS

// docsETags caches the SHA-256-derived ETag for each embedded docs file.
// Computed once on registerDocsRoutes; cheap because the corpus is tiny.
var docsETags = map[string]string{}

// registerDocsRoutes wires the public, unauthenticated documentation endpoints.
// Must be called BEFORE the SPA catch-all so the SPA NotFound handler doesn't
// preempt these paths.
func (s *Server) registerDocsRoutes() {
	subFS, err := fs.Sub(docsFS, "docs")
	if err != nil {
		panic(err)
	}
	docsETags = computeDocsETags(subFS)

	// /openapi.yaml — convenience top-level alias for the spec.
	s.router.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		serveDocsFile(w, r, subFS, "openapi.yaml")
	})

	// /docs — Redoc loader page.
	s.router.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		serveDocsFile(w, r, subFS, "index.html")
	})

	// /docs/* — anything else under the docs/ dir.
	s.router.Get("/docs/*", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/docs/")
		if name == "" {
			name = "index.html"
		}
		serveDocsFile(w, r, subFS, name)
	})
}

func serveDocsFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	// Defense-in-depth: reject anything that isn't a clean fs path. fs.ReadFile
	// already does this, but stating it locally keeps the security property
	// from depending on a transitive fs.FS implementation detail.
	if !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}

	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if etag := docsETags[name]; etag != "" {
		w.Header().Set("ETag", etag)
		// Embed contents are immutable for the lifetime of the binary, so a
		// long max-age is safe; ETag is the actual cache validator.
		w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
		if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	if ct := contentTypeFor(name); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func contentTypeFor(name string) string {
	switch path.Ext(name) {
	case ".yaml", ".yml":
		return "application/yaml; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	}
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		return ct
	}
	return ""
}

func computeDocsETags(fsys fs.FS) map[string]string {
	out := map[string]string{}
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil
		}
		sum := sha256.Sum256(data)
		out[p] = `"` + hex.EncodeToString(sum[:16]) + `"`
		return nil
	})
	return out
}
