package server

import (
	"encoding/json"
	"net/http"
)

type showDiscoverPayload struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleGetShowDiscover(w http.ResponseWriter, r *http.Request) {
	enabled, err := s.store.GetGuestSetting("show_discover")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, showDiscoverPayload{Enabled: enabled})
}

func (s *Server) handleUpdateShowDiscover(w http.ResponseWriter, r *http.Request) {
	var req showDiscoverPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := s.store.SetGuestSettings(map[string]bool{"show_discover": req.Enabled}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, showDiscoverPayload{Enabled: req.Enabled})
}
