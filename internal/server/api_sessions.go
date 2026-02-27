package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"streammon/internal/models"
)

const defaultTerminateMessage = "Your stream has been terminated by an administrator."

type terminateRequest struct {
	ServerID        int64  `json:"server_id"`
	SessionID       string `json:"session_id"`
	PlexSessionUUID string `json:"plex_session_uuid"`
	Message         string `json:"message"`
}

func (s *Server) handleTerminateSession(w http.ResponseWriter, r *http.Request) {
	if s.poller == nil {
		writeError(w, http.StatusServiceUnavailable, "poller not configured")
		return
	}

	var req terminateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.ServerID == 0 || req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "server_id and session_id are required")
		return
	}

	ms, ok := s.poller.GetServer(req.ServerID)
	if !ok {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}

	message := req.Message
	if runes := []rune(message); len(runes) > 500 {
		message = string(runes[:500])
	}
	if message == "" {
		message = defaultTerminateMessage
	}

	terminateID := req.SessionID
	if ms.Type() == models.ServerTypePlex && req.PlexSessionUUID != "" {
		terminateID = req.PlexSessionUUID
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := ms.TerminateSession(ctx, terminateID, message); err != nil {
		slog.Error("terminate session failed", "server_id", req.ServerID, "session_id", terminateID, "error", err)
		msg := "failed to terminate session"
		if errors.Is(err, models.ErrPlexPassRequired) {
			msg = "Plex Pass may be required to terminate sessions"
		}
		writeError(w, http.StatusBadGateway, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
