package server

import (
	"net/http"

	"streammon/internal/version"
)

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if s.version == nil {
		writeJSON(w, http.StatusOK, version.Info{Current: "unknown"})
		return
	}
	writeJSON(w, http.StatusOK, s.version.Info())
}
