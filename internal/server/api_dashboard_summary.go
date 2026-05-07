package server

import (
	"fmt"
	"net/http"

	"streammon/internal/models"
)

type dashboardSummaryResponse struct {
	StreamCount       int   `json:"stream_count"`
	DirectPlayCount   int   `json:"direct_play_count"`
	DirectStreamCount int   `json:"direct_stream_count"`
	TranscodeCount    int   `json:"transcode_count"`
	TotalBandwidthBps int64 `json:"total_bandwidth_bps"`
	ActiveUserCount   int   `json:"active_user_count"`
	ServerCount       int   `json:"server_count"`
}

// classifyDecision buckets a session into one of three categories matching
// the rest of the codebase's three-way model:
//
//   - transcode if either video OR audio is being transcoded
//   - direct_stream if either video or audio is "copy" (remux/passthrough) and
//     neither is transcoded
//   - direct_play otherwise (true direct play with no codec changes)
func classifyDecision(s models.ActiveStream) (transcode, directStream bool) {
	switch {
	case s.VideoDecision == models.TranscodeDecisionTranscode || s.AudioDecision == models.TranscodeDecisionTranscode:
		return true, false
	case s.VideoDecision == models.TranscodeDecisionCopy || s.AudioDecision == models.TranscodeDecisionCopy:
		return false, true
	default:
		return false, false
	}
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
	// Dedup users by (server_id, user_name) so two distinct users with the
	// same display name on different server types don't collapse into one.
	users := map[string]struct{}{}
	for _, sess := range sessions {
		resp.StreamCount++
		resp.TotalBandwidthBps += sess.Bandwidth
		switch transcode, ds := classifyDecision(sess); {
		case transcode:
			resp.TranscodeCount++
		case ds:
			resp.DirectStreamCount++
		default:
			resp.DirectPlayCount++
		}
		if sess.UserName != "" {
			users[fmt.Sprintf("%d|%s", sess.ServerID, sess.UserName)] = struct{}{}
		}
	}
	resp.ActiveUserCount = len(users)

	writeJSON(w, http.StatusOK, resp)
}
