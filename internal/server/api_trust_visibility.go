package server

import (
	"encoding/json"
	"net/http"
)

type trustScoreVisibilityPayload struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleGetTrustScoreVisibility(w http.ResponseWriter, r *http.Request) {
	enabled, err := s.store.GetGuestSetting("visible_trust_score")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, trustScoreVisibilityPayload{Enabled: enabled})
}

func (s *Server) handleUpdateTrustScoreVisibility(w http.ResponseWriter, r *http.Request) {
	var req trustScoreVisibilityPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := s.store.SetGuestSettings(map[string]bool{"visible_trust_score": req.Enabled}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, trustScoreVisibilityPayload{Enabled: req.Enabled})
}
