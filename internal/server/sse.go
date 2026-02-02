package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (s *Server) handleDashboardSSE(w http.ResponseWriter, r *http.Request) {
	if s.poller == nil {
		writeError(w, http.StatusServiceUnavailable, "poller not configured")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.poller.Subscribe()
	defer s.poller.Unsubscribe(ch)

	if data, err := json.Marshal(s.poller.CurrentSessions()); err == nil {
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
			data, err := json.Marshal(snapshot)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
