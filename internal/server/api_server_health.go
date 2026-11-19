package server

import (
	"net/http"

	"streammon/internal/models"
)

type serverHealthResponse struct {
	Failing []models.ServerPollFailure `json:"failing"`
}

func (s *Server) handleServerHealth(w http.ResponseWriter, r *http.Request) {
	resp := serverHealthResponse{Failing: []models.ServerPollFailure{}}
	if s.poller != nil {
		if failing := s.poller.FailingServers(); failing != nil {
			resp.Failing = failing
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
