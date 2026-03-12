package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"streammon/internal/httputil"
	"streammon/internal/models"
)

func parsePlaybackReportingTSV(data []byte, userMap map[string]string, serverID int64) []*models.WatchHistoryEntry {
	lines := strings.Split(string(data), "\n")
	var entries []*models.WatchHistoryEntry

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		nf := len(fields)
		if nf != 9 && nf != 12 {
			log.Printf("WARN playback-reporting: skipping line with %d fields", nf)
			continue
		}

		userName, ok := userMap[fields[1]]
		if !ok {
			log.Printf("WARN playback-reporting: unknown user %q, skipping row", fields[1])
			continue
		}

		startedAt, err := parseTimestamp(fields[0])
		if err != nil {
			log.Printf("WARN playback-reporting: bad timestamp %q: %v", fields[0], err)
			continue
		}

		durationSec, err := strconv.Atoi(strings.TrimSpace(fields[8]))
		if err != nil {
			log.Printf("WARN playback-reporting: bad duration %q: %v", fields[8], err)
			continue
		}

		watchedMs := clampMs(int64(durationSec)*1000, maxDurationMs)
		stoppedAt := startedAt.Add(time.Duration(durationSec) * time.Second)

		entry := &models.WatchHistoryEntry{
			ServerID:          serverID,
			ItemID:            fields[2],
			UserName:          userName,
			MediaType:         mapItemType(fields[3]),
			Title:             fields[4],
			TranscodeDecision: mapPlaybackMethod(fields[5]),
			Player:            fields[6],
			Platform:          fields[7],
			WatchedMs:         watchedMs,
			StartedAt:         startedAt,
			StoppedAt:         stoppedAt,
			CreatedAt:         startedAt,
		}

		// Extended fields present in 12-column format (Emby, some Jellyfin versions)
		if nf == 12 {
			if pauseSec, err := strconv.Atoi(strings.TrimSpace(fields[9])); err == nil {
				entry.PausedMs = clampMs(int64(pauseSec)*1000, maxDurationMs)
			}
			entry.IPAddress = strings.TrimSpace(fields[10])
			// fields[11] = TranscodeReasons (not mapped)
		}

		entries = append(entries, entry)
	}

	return entries
}

func parseTimestamp(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	// Try RFC3339Nano first (Emby format: 2024-03-15T20:30:00.0000000Z)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	// Jellyfin Playback Reporting exports timestamps without timezone info.
	// time.Parse treats these as UTC, which matches the plugin's default behaviour.
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp format: %q", s)
}

func mapItemType(s string) models.MediaType {
	switch strings.TrimSpace(s) {
	case "Movie":
		return models.MediaTypeMovie
	case "Episode":
		return models.MediaTypeTV
	case "TvChannel":
		return models.MediaTypeLiveTV
	case "Audio":
		return models.MediaTypeMusic
	default:
		return models.MediaTypeMovie
	}
}

func mapPlaybackMethod(s string) models.TranscodeDecision {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "Transcode") {
		return models.TranscodeDecisionTranscode
	}
	switch s {
	case "DirectStream":
		return models.TranscodeDecisionCopy
	case "DirectPlay":
		return models.TranscodeDecisionDirectPlay
	default:
		return models.TranscodeDecisionDirectPlay
	}
}

type playbackReportingUser struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

func fetchPlaybackReportingUsers(ctx context.Context, serverURL, apiKey string) (map[string]string, error) {
	url := strings.TrimRight(serverURL, "/") + "/Users"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-Emby-Token", apiKey)

	client := httputil.NewClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch users: %w", err)
	}
	defer httputil.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("users API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, httputil.MaxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("read users response: %w", err)
	}

	var users []playbackReportingUser
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}

	m := make(map[string]string, len(users))
	for _, u := range users {
		m[u.ID] = u.Name
	}
	return m, nil
}

func (s *Server) handlePlaybackReportingImport() http.HandlerFunc {
	var mu sync.Mutex
	return func(w http.ResponseWriter, r *http.Request) {
		if !mu.TryLock() {
			writeError(w, http.StatusConflict, "playback reporting import already in progress")
			return
		}
		defer mu.Unlock()

		// 50 MiB max upload
		const maxUpload = 50 << 20
		if err := r.ParseMultipartForm(maxUpload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid multipart form")
			return
		}

		serverIDStr := r.FormValue("server_id")
		serverID, err := strconv.ParseInt(serverIDStr, 10, 64)
		if err != nil || serverID == 0 {
			writeError(w, http.StatusBadRequest, "server_id is required")
			return
		}

		srv, err := s.store.GetServer(serverID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if srv.DeletedAt != nil {
			writeError(w, http.StatusBadRequest, "server has been deleted")
			return
		}
		if srv.Type != models.ServerTypeEmby && srv.Type != models.ServerTypeJellyfin {
			writeError(w, http.StatusBadRequest, "server must be Emby or Jellyfin type")
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file is required")
			return
		}
		defer file.Close()

		data, err := io.ReadAll(io.LimitReader(file, maxUpload))
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read file")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
		defer cancel()

		userMap, err := fetchPlaybackReportingUsers(ctx, srv.URL, srv.APIKey)
		if err != nil {
			log.Printf("ERROR playback-reporting import: fetch users: %v", err)
			writeError(w, http.StatusBadGateway, "failed to fetch users from media server")
			return
		}

		entries := parsePlaybackReportingTSV(data, userMap, serverID)

		if len(entries) == 0 {
			writeError(w, http.StatusBadRequest, "no valid records found in TSV file (check user IDs and format)")
			return
		}

		flusher, ok := sseFlusher(w)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming not supported")
			return
		}

		tracker := newImportTracker("playback-reporting", serverID, w, flusher)
		total := len(entries)
		const batchSize = 1000

		for i := 0; i < total; i += batchSize {
			end := i + batchSize
			if end > total {
				end = total
			}
			if err := tracker.insertBatch(ctx, s.store, entries[i:end], total); err != nil {
				tracker.fail(err)
				return
			}
		}

		tracker.complete()
	}
}
