package server

import (
	"net/http"

	"streammon/internal/models"
)

func (s *Server) handleDashboardSessions(w http.ResponseWriter, r *http.Request) {
	if s.poller == nil {
		writeJSON(w, http.StatusOK, []models.ActiveStream{})
		return
	}
	writeJSON(w, http.StatusOK, s.poller.CurrentSessions())
}
