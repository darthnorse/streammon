package server

import (
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

// userKey deduplicates active users across servers. Two distinct users with
// the same display name on different servers must not collapse, so the key
// is keyed on (server_id, user_name) rather than name alone.
type userKey struct {
	serverID int64
	name     string
}

// classifyDecision buckets a session into one of three values matching the
// rest of the codebase's three-way model:
//
//   - Transcode  — either video OR audio is being transcoded
//   - Copy       — either is "copy" (remux/passthrough) and neither is transcoded
//   - DirectPlay — true direct play with no codec changes anywhere
func classifyDecision(s models.ActiveStream) models.TranscodeDecision {
	switch {
	case s.VideoDecision == models.TranscodeDecisionTranscode || s.AudioDecision == models.TranscodeDecisionTranscode:
		return models.TranscodeDecisionTranscode
	case s.VideoDecision == models.TranscodeDecisionCopy || s.AudioDecision == models.TranscodeDecisionCopy:
		return models.TranscodeDecisionCopy
	default:
		return models.TranscodeDecisionDirectPlay
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
	users := map[userKey]struct{}{}
	for _, sess := range sessions {
		resp.StreamCount++
		resp.TotalBandwidthBps += sess.Bandwidth
		switch classifyDecision(sess) {
		case models.TranscodeDecisionTranscode:
			resp.TranscodeCount++
		case models.TranscodeDecisionCopy:
			resp.DirectStreamCount++
		default:
			resp.DirectPlayCount++
		}
		if sess.UserName != "" {
			users[userKey{serverID: sess.ServerID, name: sess.UserName}] = struct{}{}
		}
	}
	resp.ActiveUserCount = len(users)

	writeJSON(w, http.StatusOK, resp)
}
