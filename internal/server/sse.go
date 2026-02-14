package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"streammon/internal/models"
)

func sseFlusher(w http.ResponseWriter) (http.Flusher, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	return flusher, true
}

func (s *Server) handleDashboardSSE(w http.ResponseWriter, r *http.Request) {
	if s.poller == nil {
		writeError(w, http.StatusServiceUnavailable, "poller not configured")
		return
	}

	flusher, ok := sseFlusher(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Check if viewer needs filtering
	user := UserFromContext(r.Context())
	isViewer := user != nil && user.Role == models.RoleViewer
	viewerName := ""
	if isViewer {
		viewerName = user.Name
	}

	ch := s.poller.Subscribe()
	defer s.poller.Unsubscribe(ch)

	// Send initial snapshot (filtered for viewers)
	sessions := s.poller.CurrentSessions()
	if isViewer {
		sessions = filterSessionsForUser(sessions, viewerName)
	}
	if data, err := json.Marshal(sessions); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case snapshot, ok := <-ch:
			if !ok {
				return
			}
			// Filter for viewers
			if isViewer {
				snapshot = filterSessionsForUser(snapshot, viewerName)
			}
			data, err := json.Marshal(snapshot)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func filterSessionsForUser(sessions []models.ActiveStream, userName string) []models.ActiveStream {
	filtered := make([]models.ActiveStream, 0)
	for _, session := range sessions {
		if session.UserName == userName {
			filtered = append(filtered, session)
		}
	}
	return filtered
}
