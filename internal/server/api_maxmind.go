package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type maxmindSettingsResponse struct {
	LicenseKey  string `json:"license_key"`
	LastUpdated string `json:"last_updated"`
	DBAvailable bool   `json:"db_available"`
}

type maxmindSettingsRequest struct {
	LicenseKey string `json:"license_key"`
}

func (s *Server) handleGetMaxMindSettings(w http.ResponseWriter, r *http.Request) {
	key, _ := s.store.GetSetting("maxmind.license_key")
	lastUpdated, _ := s.store.GetSetting("maxmind.last_updated")

	maskedKey := ""
	if len(key) > 4 {
		maskedKey = "****" + key[len(key)-4:]
	} else if key != "" {
		maskedKey = "****"
	}

	dbAvailable := false
	if s.geoUpdater != nil {
		_, err := os.Stat(s.geoUpdater.DBPath())
		dbAvailable = err == nil
	}

	writeJSON(w, http.StatusOK, maxmindSettingsResponse{
		LicenseKey:  maskedKey,
		LastUpdated: lastUpdated,
		DBAvailable: dbAvailable,
	})
}

func (s *Server) handleUpdateMaxMindSettings(w http.ResponseWriter, r *http.Request) {
	var req maxmindSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.LicenseKey == "" {
		writeError(w, http.StatusBadRequest, "license_key is required")
		return
	}

	if err := s.store.SetSetting("maxmind.license_key", req.LicenseKey); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save")
		return
	}

	if s.geoUpdater != nil {
		if err := s.geoUpdater.Download(); err != nil {
			log.Printf("maxmind download: %v", err)
			writeError(w, http.StatusBadGateway, "download failed: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteMaxMindSettings(w http.ResponseWriter, r *http.Request) {
	if err := s.store.SetSetting("maxmind.license_key", ""); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete")
		return
	}
	_ = s.store.SetSetting("maxmind.last_updated", "")
	if s.geoUpdater != nil {
		if err := os.Remove(s.geoUpdater.DBPath()); err != nil && !os.IsNotExist(err) {
			log.Printf("removing geoip db: %v", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
