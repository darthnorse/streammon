package server

import (
	"encoding/json"
	"net/http"
)

type maintenanceSettingsResponse struct {
	ResolutionWidthAware bool `json:"resolution_width_aware"`
}

type maintenanceSettingsRequest struct {
	ResolutionWidthAware *bool `json:"resolution_width_aware,omitempty"`
}

func (s *Server) handleGetMaintenanceSettings(w http.ResponseWriter, r *http.Request) {
	widthAware, err := s.store.GetMaintenanceResolutionWidthAware()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, maintenanceSettingsResponse{ResolutionWidthAware: widthAware})
}

func (s *Server) handleUpdateMaintenanceSettings(w http.ResponseWriter, r *http.Request) {
	var req maintenanceSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.ResolutionWidthAware != nil {
		if err := s.store.SetMaintenanceResolutionWidthAware(*req.ResolutionWidthAware); err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
	}
	widthAware, _ := s.store.GetMaintenanceResolutionWidthAware()
	writeJSON(w, http.StatusOK, maintenanceSettingsResponse{ResolutionWidthAware: widthAware})
}
