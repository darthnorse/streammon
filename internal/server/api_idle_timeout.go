package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"streammon/internal/store"
)

type idleTimeoutPayload struct {
	IdleTimeoutMinutes int `json:"idle_timeout_minutes"`
}

func (s *Server) handleGetIdleTimeout(w http.ResponseWriter, r *http.Request) {
	minutes, err := s.store.GetIdleTimeoutMinutes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, idleTimeoutPayload{IdleTimeoutMinutes: minutes})
}

func (s *Server) handleUpdateIdleTimeout(w http.ResponseWriter, r *http.Request) {
	var req idleTimeoutPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.IdleTimeoutMinutes < 0 || req.IdleTimeoutMinutes > store.MaxIdleTimeoutMinutes {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("idle timeout must be between 0 and %d", store.MaxIdleTimeoutMinutes))
		return
	}

	if err := s.store.SetIdleTimeoutMinutes(req.IdleTimeoutMinutes); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	if s.poller != nil {
		s.poller.RefreshIdleTimeout()
	}

	writeJSON(w, http.StatusOK, idleTimeoutPayload{IdleTimeoutMinutes: req.IdleTimeoutMinutes})
}
