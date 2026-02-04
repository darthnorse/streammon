package server

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
)

type maxmindSettingsResponse struct {
	LicenseKey     string `json:"license_key"`
	LastUpdated    string `json:"last_updated"`
	DBAvailable    bool   `json:"db_available"`
	ASNDBAvailable bool   `json:"asn_db_available"`
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
	asnDBAvailable := false
	if s.geoUpdater != nil {
		if _, err := os.Stat(s.geoUpdater.DBPath()); err == nil {
			dbAvailable = true
		}
		if _, err := os.Stat(s.geoUpdater.ASNDBPath()); err == nil {
			asnDBAvailable = true
		}
	}

	writeJSON(w, http.StatusOK, maxmindSettingsResponse{
		LicenseKey:     maskedKey,
		LastUpdated:    lastUpdated,
		DBAvailable:    dbAvailable,
		ASNDBAvailable: asnDBAvailable,
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
			writeError(w, http.StatusBadGateway, "download failed")
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
		if err := os.Remove(s.geoUpdater.ASNDBPath()); err != nil && !os.IsNotExist(err) {
			log.Printf("removing geoip asn db: %v", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type geoBackfillResponse struct {
	Resolved int `json:"resolved"`
	Skipped  int `json:"skipped"`
	Total    int `json:"total"`
}

func (s *Server) handleGeoBackfill(w http.ResponseWriter, r *http.Request) {
	if s.geoResolver == nil {
		writeError(w, http.StatusServiceUnavailable, "GeoIP resolver not configured")
		return
	}

	ips, err := s.store.GetUncachedIPs(5000)
	if err != nil {
		log.Printf("geo backfill get ips: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get IPs")
		return
	}

	resolved := 0
	skipped := 0

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			skipped++
			continue
		}

		geo := s.geoResolver.Lookup(ip)
		if geo == nil {
			skipped++
			continue
		}

		if err := s.store.SetCachedGeo(geo); err != nil {
			log.Printf("geo backfill cache %s: %v", ipStr, err)
			skipped++
			continue
		}
		resolved++
	}

	log.Printf("geo backfill: resolved %d, skipped %d, total %d", resolved, skipped, len(ips))
	writeJSON(w, http.StatusOK, geoBackfillResponse{
		Resolved: resolved,
		Skipped:  skipped,
		Total:    len(ips),
	})
}
