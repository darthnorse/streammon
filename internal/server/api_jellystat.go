package server

import (
	"context"
	"log"
	"math"
	"time"

	"streammon/internal/jellystat"
	"streammon/internal/mediautil"
	"streammon/internal/models"
	"streammon/internal/store"
)

func (s *Server) jellystatDeps() integrationDeps {
	return integrationDeps{
		validateURL:  jellystat.ValidateURL,
		newClient:    func(url, apiKey string) (integrationClient, error) { return jellystat.NewClient(url, apiKey) },
		getConfig:    s.store.GetJellystatConfig,
		setConfig:    s.store.SetJellystatConfig,
		deleteConfig: s.store.DeleteJellystatConfig,
	}
}

func jellystatStreamer(cfg store.IntegrationConfig) (importStreamer, error) {
	client, err := jellystat.NewClient(cfg.URL, cfg.APIKey)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, serverID int64, pageSize int, handler func([]*models.WatchHistoryEntry, int) error) error {
		return client.StreamHistory(ctx, pageSize, func(batch jellystat.BatchResult) error {
			entries := make([]*models.WatchHistoryEntry, 0, len(batch.Records))
			for _, rec := range batch.Records {
				entries = append(entries, convertJellystatRecord(rec, serverID))
			}
			return handler(entries, batch.Total)
		})
	}, nil
}

func convertJellystatRecord(rec jellystat.HistoryRecord, serverID int64) *models.WatchHistoryEntry {
	mediaType := models.MediaTypeMovie
	var grandparentTitle string
	var seasonNumber, episodeNumber int

	if rec.SeriesName != nil && *rec.SeriesName != "" {
		mediaType = models.MediaTypeTV
		grandparentTitle = *rec.SeriesName
	}
	if rec.SeasonNumber != nil {
		seasonNumber = *rec.SeasonNumber
	}
	if rec.EpisodeNumber != nil {
		episodeNumber = *rec.EpisodeNumber
	}

	// ActivityDateInserted is when Jellystat recorded the activity (i.e. end time)
	stoppedAt, err := time.Parse(time.RFC3339Nano, rec.ActivityDateInserted)
	if err != nil {
		stoppedAt, err = time.Parse("2006-01-02T15:04:05", rec.ActivityDateInserted)
		if err != nil {
			stoppedAt = time.Now().UTC()
		}
	}
	stoppedAt = stoppedAt.UTC()

	durationSec := int64(math.Round(rec.PlaybackDuration))
	if durationSec < 0 {
		durationSec = 0
	}
	const maxDurationSec = 24 * 60 * 60
	if durationSec > maxDurationSec {
		log.Printf("WARN jellystat: suspiciously large PlaybackDuration=%d for item %q (user %s), clamping to 24h",
			durationSec, rec.NowPlayingItemName, rec.UserName)
		durationSec = maxDurationSec
	}
	startedAt := stoppedAt.Add(-time.Duration(durationSec) * time.Second)

	watchedMs := clampMs(durationSec*1000, maxDurationMs)

	// Use RuntimeTicks for total media duration if available (1 tick = 100ns)
	durationMs := watchedMs
	if rec.PlayState != nil && rec.PlayState.RuntimeTicks != nil && *rec.PlayState.RuntimeTicks > 0 {
		durationMs = clampMs(*rec.PlayState.RuntimeTicks/10000, maxDurationMs)
	}

	entry := &models.WatchHistoryEntry{
		ServerID:          serverID,
		ItemID:            rec.NowPlayingItemId,
		UserName:          rec.UserName,
		MediaType:         mediaType,
		Title:             rec.NowPlayingItemName,
		GrandparentTitle:  grandparentTitle,
		DurationMs:        durationMs,
		WatchedMs:         watchedMs,
		Player:            rec.Client,
		Platform:          rec.DeviceName,
		IPAddress:         rec.RemoteEndPoint,
		StartedAt:         startedAt,
		StoppedAt:         stoppedAt,
		SeasonNumber:      seasonNumber,
		EpisodeNumber:     episodeNumber,
		TranscodeDecision: convertJellystatPlayMethod(rec.PlayMethod),
	}

	if rec.TranscodingInfo != nil {
		ti := rec.TranscodingInfo
		entry.VideoCodec = ti.VideoCodec
		entry.AudioCodec = ti.AudioCodec
		entry.AudioChannels = ti.AudioChannels
		entry.Bandwidth = ti.Bitrate
		if ti.Height > 0 {
			entry.VideoResolution = mediautil.HeightToResolution(ti.Height)
		}
		if ti.IsVideoDirect {
			entry.VideoDecision = models.TranscodeDecisionDirectPlay
		} else {
			entry.VideoDecision = models.TranscodeDecisionTranscode
		}
		if ti.IsAudioDirect {
			entry.AudioDecision = models.TranscodeDecisionDirectPlay
		} else {
			entry.AudioDecision = models.TranscodeDecisionTranscode
		}
	}

	return entry
}

func convertJellystatPlayMethod(method string) models.TranscodeDecision {
	switch method {
	case "Transcode":
		return models.TranscodeDecisionTranscode
	case "DirectStream":
		return models.TranscodeDecisionCopy
	default:
		return models.TranscodeDecisionDirectPlay
	}
}
