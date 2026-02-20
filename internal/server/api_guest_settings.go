package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"streammon/internal/models"
	"streammon/internal/store"
)

type guestSettingsResponse struct {
	AccessEnabled       bool `json:"access_enabled"`
	StorePlexTokens     bool `json:"store_plex_tokens"`
	ShowDiscover        bool `json:"show_discover"`
	VisibleProfile      bool `json:"visible_profile"`
	VisibleTrustScore   bool `json:"visible_trust_score"`
	VisibleViolations   bool `json:"visible_violations"`
	VisibleWatchHistory bool `json:"visible_watch_history"`
	VisibleHousehold    bool `json:"visible_household"`
	VisibleDevices      bool `json:"visible_devices"`
	VisibleISPs         bool `json:"visible_isps"`
	PlexTokensAvailable bool `json:"plex_tokens_available"`
}

func (s *Server) handleGetGuestSettings(w http.ResponseWriter, r *http.Request) {
	gs, err := s.store.GetGuestSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	resp := guestSettingsResponse{
		AccessEnabled:       gs["access_enabled"],
		StorePlexTokens:     gs["store_plex_tokens"],
		ShowDiscover:        gs["show_discover"],
		VisibleProfile:      gs["visible_profile"],
		VisibleTrustScore:   gs["visible_trust_score"],
		VisibleViolations:   gs["visible_violations"],
		VisibleWatchHistory: gs["visible_watch_history"],
		VisibleHousehold:    gs["visible_household"],
		VisibleDevices:      gs["visible_devices"],
		VisibleISPs:         gs["visible_isps"],
	}
	user := UserFromContext(r.Context())
	if user != nil && user.Role == models.RoleAdmin {
		resp.PlexTokensAvailable = s.store.HasEncryptor()
	}
	writeJSON(w, http.StatusOK, resp)
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
	if enabled, ok := updates["store_plex_tokens"]; ok && !enabled {
		if err := s.store.DeleteProviderTokensByProvider(store.ProviderPlex); err != nil {
			log.Printf("DeleteProviderTokensByProvider error: %v", err)
		}
	}
	log.Printf("Guest settings updated: %v", updates)
	s.handleGetGuestSettings(w, r)
}
