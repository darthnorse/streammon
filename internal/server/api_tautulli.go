package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
	"streammon/internal/tautulli"
)

type tautulliSettingsResponse struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

type tautulliSettingsRequest struct {
	URL    string `json:"url"`
	APIKey string `json:"api_key"`
}

type tautulliTestResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type tautulliImportRequest struct {
	ServerID int64 `json:"server_id"`
}

type tautulliImportResponse struct {
	Imported int    `json:"imported"`
	Skipped  int    `json:"skipped"`
	Total    int    `json:"total"`
	Error    string `json:"error,omitempty"`
}

type tautulliProgressEvent struct {
	Type      string `json:"type"` // "progress" or "complete" or "error"
	Processed int    `json:"processed"`
	Total     int    `json:"total"`
	Inserted  int    `json:"inserted"`
	Skipped   int    `json:"skipped"`
	Error     string `json:"error,omitempty"`
}

func (s *Server) handleGetTautulliSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetTautulliConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	apiKey := ""
	if cfg.APIKey != "" {
		apiKey = maskedSecret
	}

	writeJSON(w, http.StatusOK, tautulliSettingsResponse{
		URL:    cfg.URL,
		APIKey: apiKey,
	})
}

func (s *Server) handleUpdateTautulliSettings(w http.ResponseWriter, r *http.Request) {
	var req tautulliSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.APIKey == maskedSecret {
		req.APIKey = ""
	}

	if req.URL != "" {
		if err := tautulli.ValidateURL(req.URL); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	storeCfg := store.TautulliConfig{
		URL:    req.URL,
		APIKey: req.APIKey,
	}

	if err := s.store.SetTautulliConfig(storeCfg); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteTautulliSettings(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteTautulliConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleTestTautulliConnection(w http.ResponseWriter, r *http.Request) {
	var req tautulliSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	apiKey := req.APIKey
	if apiKey == "" || apiKey == maskedSecret {
		cfg, err := s.store.GetTautulliConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal")
			return
		}
		apiKey = cfg.APIKey
	}

	if apiKey == "" {
		writeError(w, http.StatusBadRequest, "api_key is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	client, err := tautulli.NewClient(req.URL, apiKey)
	if err != nil {
		writeJSON(w, http.StatusOK, tautulliTestResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if err := client.TestConnection(ctx); err != nil {
		writeJSON(w, http.StatusOK, tautulliTestResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, tautulliTestResponse{Success: true})
}

func (s *Server) handleTautulliImport(w http.ResponseWriter, r *http.Request) {
	var req tautulliImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.ServerID == 0 {
		writeError(w, http.StatusBadRequest, "server_id is required")
		return
	}

	_, err := s.store.GetServer(req.ServerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	cfg, err := s.store.GetTautulliConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	if cfg.URL == "" || cfg.APIKey == "" {
		writeError(w, http.StatusBadRequest, "Tautulli settings not configured")
		return
	}

	client, err := tautulli.NewClient(cfg.URL, cfg.APIKey)
	if err != nil {
		writeJSON(w, http.StatusOK, tautulliImportResponse{
			Error: err.Error(),
		})
		return
	}

	// Set up SSE streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	sendEvent := func(event tautulliProgressEvent) {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	var totalInserted, totalSkipped, totalRecords, processed int

	// Rate limiter for enrichment: 10 requests/second to avoid overwhelming Tautulli
	enrichRateLimiter := time.NewTicker(100 * time.Millisecond)
	defer enrichRateLimiter.Stop()

	err = client.StreamHistory(ctx, 1000, func(batch tautulli.BatchResult) error {
		entries := make([]*models.WatchHistoryEntry, 0, len(batch.Records))
		for i, rec := range batch.Records {
			entry := convertTautulliRecord(rec, req.ServerID)

			if rec.ReferenceID != 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-enrichRateLimiter.C:
				}
				streamData, err := client.GetStreamData(ctx, int(rec.ReferenceID))
				if err != nil {
					log.Printf("Failed to get stream data for reference_id %d: %v", rec.ReferenceID, err)
				} else if streamData != nil {
					enrichEntryFromStreamData(entry, streamData)
				}
			}

			entries = append(entries, entry)

			// Send progress every 10 records or on last record
			if (i+1)%10 == 0 || i == len(batch.Records)-1 {
				sendEvent(tautulliProgressEvent{
					Type:      "progress",
					Processed: processed + i + 1,
					Total:     batch.Total,
					Inserted:  totalInserted,
					Skipped:   totalSkipped,
				})
			}
		}

		inserted, skipped, err := s.store.InsertHistoryBatch(ctx, entries)
		if err != nil {
			return err
		}

		totalInserted += inserted
		totalSkipped += skipped
		totalRecords = batch.Total
		processed += len(batch.Records)
		return nil
	})

	if err != nil {
		log.Printf("Tautulli import error: %v (imported %d, skipped %d)", err, totalInserted, totalSkipped)
		sendEvent(tautulliProgressEvent{
			Type:      "error",
			Processed: processed,
			Total:     totalRecords,
			Inserted:  totalInserted,
			Skipped:   totalSkipped,
			Error:     err.Error(),
		})
		return
	}

	log.Printf("Tautulli import completed: %d inserted, %d skipped, server_id=%d", totalInserted, totalSkipped, req.ServerID)

	sendEvent(tautulliProgressEvent{
		Type:      "complete",
		Processed: processed,
		Total:     totalRecords,
		Inserted:  totalInserted,
		Skipped:   totalSkipped,
	})
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
	if rec.Stopped == 0 {
		stoppedAt = startedAt.Add(time.Duration(rec.Duration) * time.Second)
	}

	durationMs := rec.Duration * 1000
	watchedMs := rec.PlayDuration * 1000

	if durationMs < 0 {
		durationMs = 0
	}
	if watchedMs < 0 {
		watchedMs = 0
	}

	const maxDurationMs = 24 * 60 * 60 * 1000
	if durationMs > maxDurationMs {
		durationMs = maxDurationMs
	}
	if watchedMs > maxDurationMs {
		watchedMs = maxDurationMs
	}

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
		entry.VideoResolution = tautulli.HeightToResolution(sd.VideoHeight)
	}
}
