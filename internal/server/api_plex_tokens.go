package server

import (
	"encoding/json"
	"net/http"
)

type plexTokensSettingPayload struct {
	Enabled   bool `json:"enabled"`
	Available bool `json:"available"`
}

func (s *Server) handleGetPlexTokensSetting(w http.ResponseWriter, r *http.Request) {
	available := s.store.HasEncryptor()
	enabled := false
	if available {
		var err error
		enabled, err = s.store.GetStorePlexTokens()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}
	writeJSON(w, http.StatusOK, plexTokensSettingPayload{Enabled: enabled, Available: available})
}

func (s *Server) handleUpdatePlexTokensSetting(w http.ResponseWriter, r *http.Request) {
	if !s.store.HasEncryptor() {
		writeError(w, http.StatusBadRequest, "encryption not available")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := s.store.SetStorePlexTokens(req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, plexTokensSettingPayload{
		Enabled:   req.Enabled,
		Available: true,
	})
}
