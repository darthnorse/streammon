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

// registerDocsRoutes wires the public, unauthenticated documentation endpoints.
// Must be called BEFORE the SPA catch-all so the SPA NotFound handler doesn't
// preempt these paths.
//
// Caching strategy:
//   - The vendored `redoc.standalone.js` is treated as immutable for a long
//     time (~30 days). It only changes when we bump the pinned version, and
//     stale clients will still revalidate via ETag eventually.
//   - `index.html` and `openapi.yaml` are unversioned and update with the
//     binary, so we use a short max-age and force revalidation on each
//     request — the ETag handles 304s without a re-download.
func (s *Server) registerDocsRoutes() {
	subFS, err := fs.Sub(docsFS, "docs")
	if err != nil {
		panic(err)
	}
	etags := computeDocsETags(subFS)

	// /openapi.yaml — convenience top-level alias for the spec.
	s.router.Get("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		serveDocsFile(w, r, subFS, "openapi.yaml", etags)
	})

	// /docs — Redoc loader page.
	s.router.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		serveDocsFile(w, r, subFS, "index.html", etags)
	})

	// /docs/* — anything else under the docs/ dir.
	s.router.Get("/docs/*", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/docs/")
		if name == "" {
			name = "index.html"
		}
		serveDocsFile(w, r, subFS, name, etags)
	})
}

func serveDocsFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string, etags map[string]string) {
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

	if etag := etags[name]; etag != "" {
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", cacheControlFor(name))
		if etagMatches(r.Header.Get("If-None-Match"), etag) {
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

// cacheControlFor returns the Cache-Control header for a docs asset. Versioned
// assets (the Redoc bundle, which only changes on a version bump) get a long
// immutable max-age. Unversioned assets (the spec and the loader HTML) get a
// short max-age + must-revalidate so deploys are picked up promptly via ETag.
func cacheControlFor(name string) string {
	if name == "redoc.standalone.js" {
		return "public, max-age=2592000, immutable" // 30 days
	}
	return "public, max-age=300, must-revalidate"
}

// etagMatches implements the RFC 7232 §3.2 If-None-Match comparison: accepts a
// comma-separated list of ETags, ignores leading "W/" weak prefix, and treats
// "*" as a wildcard match.
func etagMatches(headerVal, etag string) bool {
	if headerVal == "" {
		return false
	}
	for _, candidate := range strings.Split(headerVal, ",") {
		candidate = strings.TrimSpace(candidate)
		candidate = strings.TrimPrefix(candidate, "W/")
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}

func contentTypeFor(name string) string {
	ext := path.Ext(name)
	switch ext {
	case ".yaml", ".yml":
		return "application/yaml; charset=utf-8"
	}
	return mime.TypeByExtension(ext)
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
