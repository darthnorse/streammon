package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"streammon/internal/store"
)

type guestSettingsResponse struct {
	Settings map[string]bool `json:"settings"`
}

func (s *Server) handleGetGuestSettings(w http.ResponseWriter, r *http.Request) {
	gs, err := s.store.GetGuestSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, guestSettingsResponse{Settings: gs})
}

func (s *Server) handleUpdateGuestSettings(w http.ResponseWriter, r *http.Request) {
	var updates map[string]bool
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(updates) == 0 {
		writeError(w, http.StatusBadRequest, "no updates provided")
		return
	}
	for k := range updates {
		if !store.ValidGuestSettingKey(k) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown setting key: %q", k))
			return
		}
	}
	if err := s.store.SetGuestSettings(updates); err != nil {
		log.Printf("SetGuestSettings error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	log.Printf("Guest settings updated: %v", updates)
	s.handleGetGuestSettings(w, r)
}
