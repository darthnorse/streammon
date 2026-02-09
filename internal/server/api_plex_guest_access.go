package server

import (
	"encoding/json"
	"net/http"
)

type plexGuestAccessPayload struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleGetPlexGuestAccess(w http.ResponseWriter, r *http.Request) {
	enabled, err := s.store.GetPlexGuestAccess()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, plexGuestAccessPayload{Enabled: enabled})
}

func (s *Server) handleUpdatePlexGuestAccess(w http.ResponseWriter, r *http.Request) {
	var req plexGuestAccessPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := s.store.SetPlexGuestAccess(req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, plexGuestAccessPayload{Enabled: req.Enabled})
}
