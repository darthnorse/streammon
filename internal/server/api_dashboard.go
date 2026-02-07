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

	sessions := s.poller.CurrentSessions()

	// Viewers can only see their own sessions
	user := UserFromContext(r.Context())
	if user != nil && user.Role == models.RoleViewer {
		filtered := make([]models.ActiveStream, 0)
		for _, session := range sessions {
			if session.UserName == user.Name {
				filtered = append(filtered, session)
			}
		}
		writeJSON(w, http.StatusOK, filtered)
		return
	}

	writeJSON(w, http.StatusOK, sessions)
}
