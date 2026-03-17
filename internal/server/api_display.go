package server

import (
	"encoding/json"
	"net/http"

	"streammon/internal/units"
)

type displaySettingsResponse struct {
	UnitSystem     string `json:"unit_system"`
	DiscoverRegion string `json:"discover_region"`
}

type displaySettingsRequest struct {
	UnitSystem     string  `json:"unit_system"`
	DiscoverRegion *string `json:"discover_region,omitempty"`
}

func (s *Server) handleGetDisplaySettings(w http.ResponseWriter, r *http.Request) {
	system, err := s.store.GetUnitSystem()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	region, err := s.store.GetDiscoverRegion()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, displaySettingsResponse{
		UnitSystem:     system,
		DiscoverRegion: region,
	})
}

func (s *Server) handleUpdateDisplaySettings(w http.ResponseWriter, r *http.Request) {
	var req displaySettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.UnitSystem != "" {
		if !units.IsValid(req.UnitSystem) {
			writeError(w, http.StatusBadRequest, "invalid unit system")
			return
		}
		if err := s.store.SetUnitSystem(req.UnitSystem); err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}

	if req.DiscoverRegion != nil {
		if err := s.store.SetDiscoverRegion(*req.DiscoverRegion); err != nil {
			writeError(w, http.StatusBadRequest, "invalid region code")
			return
		}
	}

	system, _ := s.store.GetUnitSystem()
	region, _ := s.store.GetDiscoverRegion()

	writeJSON(w, http.StatusOK, displaySettingsResponse{
		UnitSystem:     system,
		DiscoverRegion: region,
	})
}
