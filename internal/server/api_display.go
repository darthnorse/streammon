package server

import (
	"encoding/json"
	"net/http"

	"streammon/internal/units"
)

type displaySettingsResponse struct {
	UnitSystem string `json:"unit_system"`
}

type displaySettingsRequest struct {
	UnitSystem string `json:"unit_system"`
}

func (s *Server) handleGetDisplaySettings(w http.ResponseWriter, r *http.Request) {
	system, err := s.store.GetUnitSystem()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, displaySettingsResponse{
		UnitSystem: system,
	})
}

func (s *Server) handleUpdateDisplaySettings(w http.ResponseWriter, r *http.Request) {
	var req displaySettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if !units.IsValid(req.UnitSystem) {
		writeError(w, http.StatusBadRequest, "invalid unit system")
		return
	}

	if err := s.store.SetUnitSystem(req.UnitSystem); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, displaySettingsResponse{
		UnitSystem: req.UnitSystem,
	})
}
