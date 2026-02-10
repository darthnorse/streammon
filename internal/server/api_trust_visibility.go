package server

import (
	"encoding/json"
	"net/http"
)

type trustScoreVisibilityPayload struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleGetTrustScoreVisibility(w http.ResponseWriter, r *http.Request) {
	enabled, err := s.store.GetTrustScoreVisibility()
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

	if err := s.store.SetTrustScoreVisibility(req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, trustScoreVisibilityPayload{Enabled: req.Enabled})
}
