package server

import (
	"encoding/json"
	"net/http"
)

type guestAccessPayload struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleGetGuestAccess(w http.ResponseWriter, r *http.Request) {
	enabled, err := s.store.GetGuestAccess()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, guestAccessPayload{Enabled: enabled})
}

func (s *Server) handleUpdateGuestAccess(w http.ResponseWriter, r *http.Request) {
	var req guestAccessPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := s.store.SetGuestAccess(req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, guestAccessPayload{Enabled: req.Enabled})
}
