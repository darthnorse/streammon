package server

import (
	"net/http"

	"streammon/internal/models"
)

type dashboardSummaryResponse struct {
	StreamCount       int   `json:"stream_count"`
	DirectPlayCount   int   `json:"direct_play_count"`
	TranscodeCount    int   `json:"transcode_count"`
	TotalBandwidthBps int64 `json:"total_bandwidth_bps"`
	ActiveUserCount   int   `json:"active_user_count"`
	ServerCount       int   `json:"server_count"`
}

func (s *Server) handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	resp := dashboardSummaryResponse{}

	servers, err := s.store.ListServers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	resp.ServerCount = len(servers)

	if s.poller == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	sessions := s.poller.CurrentSessions()
	users := map[string]struct{}{}
	for _, sess := range sessions {
		resp.StreamCount++
		resp.TotalBandwidthBps += sess.Bandwidth
		if sess.VideoDecision == models.TranscodeDecisionTranscode || sess.AudioDecision == models.TranscodeDecisionTranscode {
			resp.TranscodeCount++
		} else {
			resp.DirectPlayCount++
		}
		if sess.UserName != "" {
			users[sess.UserName] = struct{}{}
		}
	}
	resp.ActiveUserCount = len(users)

	writeJSON(w, http.StatusOK, resp)
}
