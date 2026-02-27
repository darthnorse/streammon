package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"streammon/internal/mediautil"
	"streammon/internal/models"
	"streammon/internal/store"
	"streammon/internal/tautulli"
)

func (s *Server) tautulliDeps() integrationDeps {
	return integrationDeps{
		validateURL:  tautulli.ValidateURL,
		newClient:    func(url, apiKey string) (integrationClient, error) { return tautulli.NewClient(url, apiKey) },
		getConfig:    s.store.GetTautulliConfig,
		setConfig:    s.store.SetTautulliConfig,
		deleteConfig: s.store.DeleteTautulliConfig,
	}
}

func tautulliStreamer(cfg store.IntegrationConfig) (importStreamer, error) {
	client, err := tautulli.NewClient(cfg.URL, cfg.APIKey)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, serverID int64, pageSize int, handler func([]*models.WatchHistoryEntry, int) error) error {
		return client.StreamHistory(ctx, pageSize, func(batch tautulli.BatchResult) error {
			entries := make([]*models.WatchHistoryEntry, 0, len(batch.Records))
			for _, rec := range batch.Records {
				entry := convertTautulliRecord(rec, serverID)
				entry.TautulliReferenceID = int64(rec.ReferenceID)
				entries = append(entries, entry)
			}
			return handler(entries, batch.Total)
		})
	}, nil
}

func convertTautulliRecord(rec tautulli.HistoryRecord, serverID int64) *models.WatchHistoryEntry {
	mediaType := models.MediaTypeMovie
	switch rec.MediaType {
	case "episode":
		mediaType = models.MediaTypeTV
	case "track":
		mediaType = models.MediaTypeMusic
	}

	startedAt := time.Unix(rec.Started, 0).UTC()
	stoppedAt := time.Unix(rec.Stopped, 0).UTC()
	if rec.Stopped == 0 && rec.PlayDuration > 0 {
		// Use actual watch time, not total media duration, so concurrent
		// stream calculations reflect real overlap rather than full runtime.
		stoppedAt = startedAt.Add(time.Duration(rec.PlayDuration) * time.Second)
	}

	durationMs := clampMs(rec.Duration*1000, maxDurationMs)
	watchedMs := clampMs(rec.PlayDuration*1000, maxDurationMs)

	return &models.WatchHistoryEntry{
		ServerID:          serverID,
		ItemID:            string(rec.RatingKey),
		GrandparentItemID: string(rec.GrandparentRatingKey),
		UserName:          rec.User,
		MediaType:         mediaType,
		Title:             rec.Title,
		ParentTitle:       rec.ParentTitle,
		GrandparentTitle:  rec.GrandparentTitle,
		Year:              int(rec.Year),
		DurationMs:        durationMs,
		WatchedMs:         watchedMs,
		Player:            rec.Player,
		Platform:          rec.Platform,
		IPAddress:         rec.IPAddress,
		StartedAt:         startedAt,
		StoppedAt:         stoppedAt,
		SeasonNumber:      int(rec.ParentMediaIndex),
		EpisodeNumber:     int(rec.MediaIndex),
		ThumbURL:          rec.Thumb,
		VideoResolution:   rec.VideoFullResolution,
		TranscodeDecision: convertTranscodeDecision(rec.TranscodeDecision),
	}
}

func convertTranscodeDecision(decision string) models.TranscodeDecision {
	switch decision {
	case "transcode":
		return models.TranscodeDecisionTranscode
	case "copy":
		return models.TranscodeDecisionCopy
	default:
		return models.TranscodeDecisionDirectPlay
	}
}

type tautulliEnrichRequest struct {
	ServerID int64 `json:"server_id"`
}

type tautulliEnrichResponse struct {
	Total  int    `json:"total"`
	Status string `json:"status"` // "none", "started"
}

func (s *Server) handleStartEnrichment(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSettingsBody)
	var req tautulliEnrichRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.ServerID == 0 {
		writeError(w, http.StatusBadRequest, "server_id is required")
		return
	}

	srv, err := s.store.GetServer(req.ServerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if srv.DeletedAt != nil {
		writeError(w, http.StatusBadRequest, "server has been deleted")
		return
	}

	cfg, err := s.store.GetTautulliConfig()
	if err != nil {
		log.Printf("ERROR enrichment: GetTautulliConfig: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if !cfg.HasCredentials() {
		writeError(w, http.StatusBadRequest, "Tautulli settings not configured")
		return
	}

	count, err := s.store.CountUnenrichedHistory(r.Context(), req.ServerID)
	if err != nil {
		log.Printf("ERROR enrichment: CountUnenrichedHistory(server_id=%d): %v", req.ServerID, err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	if count == 0 {
		writeJSON(w, http.StatusOK, tautulliEnrichResponse{Total: 0, Status: "none"})
		return
	}

	client, err := tautulli.NewClient(cfg.URL, cfg.APIKey)
	if err != nil {
		log.Printf("ERROR enrichment: NewClient: %v", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	if !s.enrichment.start(s.appCtx, s.store, client, req.ServerID, count) {
		writeError(w, http.StatusConflict, "enrichment already running")
		return
	}
	writeJSON(w, http.StatusOK, tautulliEnrichResponse{Total: count, Status: "started"})
}

func (s *Server) handleStopEnrichment(w http.ResponseWriter, r *http.Request) {
	s.enrichment.stop()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleEnrichmentStatus(w http.ResponseWriter, r *http.Request) {
	st := s.enrichment.status()

	if !st.Running {
		serverID := st.ServerID
		if v := r.URL.Query().Get("server_id"); v != "" {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
				serverID = id
			}
		}
		if serverID > 0 {
			count, err := s.store.CountUnenrichedHistory(r.Context(), serverID)
			if err == nil {
				st.Total = count
				st.ServerID = serverID
			}
		}
	}

	writeJSON(w, http.StatusOK, st)
}

func enrichEntryFromStreamData(entry *models.WatchHistoryEntry, sd *tautulli.StreamData) {
	if sd.VideoCodec != "" {
		entry.VideoCodec = sd.VideoCodec
	}
	if sd.AudioCodec != "" {
		entry.AudioCodec = sd.AudioCodec
	}
	if sd.AudioChannels > 0 {
		entry.AudioChannels = sd.AudioChannels
	}
	if sd.Bandwidth > 0 {
		entry.Bandwidth = sd.Bandwidth
	}
	if sd.TranscodeDecision != "" {
		entry.TranscodeDecision = convertTranscodeDecision(sd.TranscodeDecision)
	}
	if sd.VideoDecision != "" {
		entry.VideoDecision = convertTranscodeDecision(sd.VideoDecision)
	}
	if sd.AudioDecision != "" {
		entry.AudioDecision = convertTranscodeDecision(sd.AudioDecision)
	}
	entry.TranscodeHWDecode = sd.TranscodeHWDecode
	entry.TranscodeHWEncode = sd.TranscodeHWEncode

	if sd.VideoDynamicRange != "" {
		entry.DynamicRange = sd.VideoDynamicRange
	}

	if entry.VideoResolution == "" && sd.VideoHeight > 0 {
		entry.VideoResolution = mediautil.HeightToResolution(sd.VideoHeight)
	}
}
