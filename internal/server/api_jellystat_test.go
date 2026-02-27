package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"streammon/internal/jellystat"
	"streammon/internal/models"
	"streammon/internal/store"
)

func mockJellystatServer(t *testing.T) *httptest.Server {
	return mockJellystatServerWithHistory(t, nil)
}

func mockJellystatServerWithHistory(t *testing.T, historyRecords []jellystat.HistoryRecord) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-token") != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"message": "Authentication failed."})
			return
		}
		switch r.URL.Path {
		case "/api/getconfig":
			json.NewEncoder(w).Encode(map[string]any{
				"JF_HOST":  "http://localhost:8096",
				"APP_USER": "admin",
			})
		case "/api/getHistory":
			page := 1
			if p := r.URL.Query().Get("page"); p != "" {
				if v, err := strconv.Atoi(p); err == nil {
					page = v
				}
			}
			size := 1000
			if s := r.URL.Query().Get("size"); s != "" {
				if v, err := strconv.Atoi(s); err == nil && v > 0 {
					size = v
				}
			}
			records := historyRecords
			if records == nil {
				records = []jellystat.HistoryRecord{}
			}
			totalPages := (len(records) + size - 1) / size
			if totalPages == 0 {
				totalPages = 1
			}
			start := (page - 1) * size
			end := start + size
			if start > len(records) {
				start = len(records)
			}
			if end > len(records) {
				end = len(records)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"current_page": page,
				"pages":        totalPages,
				"size":         size,
				"results":      records[start:end],
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func configureJellystat(t *testing.T, st *store.Store, mockURL string) {
	t.Helper()
	if err := st.SetJellystatConfig(store.JellystatConfig{
		URL:     mockURL,
		APIKey:  "test-api-key",
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestJellystatIntegrationSettings(t *testing.T) {
	testIntegrationSettingsCRUD(t, integrationTestConfig{
		name:         "jellystat",
		settingsPath: "/api/settings/jellystat",
		testPath:     "/api/settings/jellystat/test",
		configure:    configureJellystat,
		getConfig:    func(st *store.Store) (store.IntegrationConfig, error) { return st.GetJellystatConfig() },
		setConfig:    func(st *store.Store, c store.IntegrationConfig) error { return st.SetJellystatConfig(c) },
		mockServer:   mockJellystatServer,
	})
}

func TestJellystatImport_MissingServerID(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/jellystat/import", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestJellystatImport_ServerNotFound(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/jellystat/import", strings.NewReader(`{"server_id":999}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestJellystatImport_NoJellystatConfigured(t *testing.T) {
	srv, st := newTestServerWrapped(t)

	jf := &models.Server{Name: "Test", Type: models.ServerTypeJellyfin, URL: "http://test", APIKey: "k", Enabled: true}
	st.CreateServer(jf)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/jellystat/import", strings.NewReader(fmt.Sprintf(`{"server_id":%d}`, jf.ID)))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConvertJellystatRecord_Movie(t *testing.T) {
	rec := jellystat.HistoryRecord{
		UserName:             "alice",
		NowPlayingItemName:   "Die Hard",
		NowPlayingItemId:     "abc123",
		PlaybackDuration:     3600,
		ActivityDateInserted: "2024-06-15T20:00:00.000Z",
		Client:               "Jellyfin Web",
		DeviceName:           "Chrome",
		RemoteEndPoint:       "192.168.1.10",
		PlayMethod:           "DirectPlay",
		TranscodingInfo: &jellystat.TranscodingInfo{
			VideoCodec:    "h264",
			AudioCodec:    "aac",
			AudioChannels: 6,
			Bitrate:       15000000,
			Width:         1920,
			Height:        1080,
			IsVideoDirect: true,
			IsAudioDirect: false,
		},
	}

	entry := convertJellystatRecord(rec, 42)

	if entry.ServerID != 42 {
		t.Errorf("ServerID = %d, want 42", entry.ServerID)
	}
	if entry.UserName != "alice" {
		t.Errorf("UserName = %q, want alice", entry.UserName)
	}
	if entry.Title != "Die Hard" {
		t.Errorf("Title = %q, want Die Hard", entry.Title)
	}
	if entry.MediaType != models.MediaTypeMovie {
		t.Errorf("MediaType = %q, want movie", entry.MediaType)
	}
	if entry.GrandparentTitle != "" {
		t.Errorf("GrandparentTitle = %q, want empty", entry.GrandparentTitle)
	}
	if entry.DurationMs != 3600000 {
		t.Errorf("DurationMs = %d, want 3600000", entry.DurationMs)
	}
	if entry.WatchedMs != 3600000 {
		t.Errorf("WatchedMs = %d, want 3600000", entry.WatchedMs)
	}
	if entry.TranscodeDecision != models.TranscodeDecisionDirectPlay {
		t.Errorf("TranscodeDecision = %q, want direct play", entry.TranscodeDecision)
	}
	if entry.VideoCodec != "h264" {
		t.Errorf("VideoCodec = %q, want h264", entry.VideoCodec)
	}
	if entry.AudioCodec != "aac" {
		t.Errorf("AudioCodec = %q, want aac", entry.AudioCodec)
	}
	if entry.VideoResolution != "1080p" {
		t.Errorf("VideoResolution = %q, want 1080p", entry.VideoResolution)
	}
	if entry.VideoDecision != models.TranscodeDecisionDirectPlay {
		t.Errorf("VideoDecision = %q, want direct play", entry.VideoDecision)
	}
	if entry.AudioDecision != models.TranscodeDecisionTranscode {
		t.Errorf("AudioDecision = %q, want transcode", entry.AudioDecision)
	}
	if entry.Bandwidth != 15000000 {
		t.Errorf("Bandwidth = %d, want 15000000", entry.Bandwidth)
	}
	if entry.Player != "Jellyfin Web" {
		t.Errorf("Player = %q, want Jellyfin Web", entry.Player)
	}
	if entry.Platform != "Chrome" {
		t.Errorf("Platform = %q, want Chrome", entry.Platform)
	}
	if entry.IPAddress != "192.168.1.10" {
		t.Errorf("IPAddress = %q, want 192.168.1.10", entry.IPAddress)
	}

	// ActivityDateInserted is the end time; startedAt = endTime - duration
	expectedStop := time.Date(2024, 6, 15, 20, 0, 0, 0, time.UTC)
	if !entry.StoppedAt.Equal(expectedStop) {
		t.Errorf("StoppedAt = %v, want %v", entry.StoppedAt, expectedStop)
	}
	expectedStart := expectedStop.Add(-3600 * time.Second)
	if !entry.StartedAt.Equal(expectedStart) {
		t.Errorf("StartedAt = %v, want %v", entry.StartedAt, expectedStart)
	}
}

func TestConvertJellystatRecord_TVShow(t *testing.T) {
	seriesName := "Breaking Bad"
	seasonNum := 3
	episodeNum := 5
	rec := jellystat.HistoryRecord{
		UserName:             "bob",
		NowPlayingItemName:   "Caballo sin Nombre",
		NowPlayingItemId:     "ep123",
		SeriesName:           &seriesName,
		SeasonNumber:         &seasonNum,
		EpisodeNumber:        &episodeNum,
		PlaybackDuration:     2800,
		ActivityDateInserted: "2024-06-15T21:00:00.000Z",
		PlayMethod:           "Transcode",
	}

	entry := convertJellystatRecord(rec, 1)

	if entry.MediaType != models.MediaTypeTV {
		t.Errorf("MediaType = %q, want episode", entry.MediaType)
	}
	if entry.GrandparentTitle != "Breaking Bad" {
		t.Errorf("GrandparentTitle = %q, want Breaking Bad", entry.GrandparentTitle)
	}
	if entry.SeasonNumber != 3 {
		t.Errorf("SeasonNumber = %d, want 3", entry.SeasonNumber)
	}
	if entry.EpisodeNumber != 5 {
		t.Errorf("EpisodeNumber = %d, want 5", entry.EpisodeNumber)
	}
	if entry.TranscodeDecision != models.TranscodeDecisionTranscode {
		t.Errorf("TranscodeDecision = %q, want transcode", entry.TranscodeDecision)
	}
}

func TestConvertJellystatRecord_NullTranscodingInfo(t *testing.T) {
	rec := jellystat.HistoryRecord{
		UserName:             "alice",
		NowPlayingItemName:   "Movie",
		PlaybackDuration:     100,
		ActivityDateInserted: "2024-01-01T00:00:00.000Z",
		PlayMethod:           "DirectPlay",
	}

	entry := convertJellystatRecord(rec, 1)

	if entry.VideoCodec != "" {
		t.Errorf("VideoCodec = %q, want empty", entry.VideoCodec)
	}
	if entry.VideoResolution != "" {
		t.Errorf("VideoResolution = %q, want empty", entry.VideoResolution)
	}
}

func TestConvertJellystatRecord_RuntimeTicks(t *testing.T) {
	// 2-hour movie, user watched 30 minutes
	runtimeTicks := int64(72000000000) // 2h in 100ns ticks
	rec := jellystat.HistoryRecord{
		UserName:             "alice",
		NowPlayingItemName:   "Long Movie",
		NowPlayingItemId:     "lm1",
		PlaybackDuration:     1800, // 30 minutes watched
		ActivityDateInserted: "2024-06-15T20:00:00.000Z",
		PlayMethod:           "DirectPlay",
		PlayState:            &jellystat.PlayState{RuntimeTicks: &runtimeTicks},
	}

	entry := convertJellystatRecord(rec, 1)

	if entry.DurationMs != 7200000 {
		t.Errorf("DurationMs = %d, want 7200000 (from RuntimeTicks)", entry.DurationMs)
	}
	if entry.WatchedMs != 1800000 {
		t.Errorf("WatchedMs = %d, want 1800000 (from PlaybackDuration)", entry.WatchedMs)
	}
}

func TestConvertJellystatRecord_NoPlayState(t *testing.T) {
	rec := jellystat.HistoryRecord{
		UserName:             "alice",
		NowPlayingItemName:   "Movie",
		NowPlayingItemId:     "m1",
		PlaybackDuration:     3600,
		ActivityDateInserted: "2024-06-15T20:00:00.000Z",
		PlayMethod:           "DirectPlay",
	}

	entry := convertJellystatRecord(rec, 1)

	// Without PlayState, DurationMs falls back to WatchedMs
	if entry.DurationMs != 3600000 {
		t.Errorf("DurationMs = %d, want 3600000 (fallback to WatchedMs)", entry.DurationMs)
	}
	if entry.WatchedMs != 3600000 {
		t.Errorf("WatchedMs = %d, want 3600000", entry.WatchedMs)
	}
}

func TestConvertJellystatRecord_BadTimestamp(t *testing.T) {
	rec := jellystat.HistoryRecord{
		UserName:             "alice",
		NowPlayingItemName:   "Movie",
		PlaybackDuration:     100,
		ActivityDateInserted: "not-a-timestamp",
	}

	entry := convertJellystatRecord(rec, 1)

	if entry.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero; expected fallback to time.Now()")
	}
}

func TestConvertJellystatPlayMethod(t *testing.T) {
	tests := []struct {
		method string
		want   models.TranscodeDecision
	}{
		{"Transcode", models.TranscodeDecisionTranscode},
		{"DirectPlay", models.TranscodeDecisionDirectPlay},
		{"DirectStream", models.TranscodeDecisionCopy},
		{"", models.TranscodeDecisionDirectPlay},
	}

	for _, tt := range tests {
		got := convertJellystatPlayMethod(tt.method)
		if got != tt.want {
			t.Errorf("convertJellystatPlayMethod(%q) = %q, want %q", tt.method, got, tt.want)
		}
	}
}

type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {}

func TestJellystatImport_SSEStreaming(t *testing.T) {
	records := []jellystat.HistoryRecord{
		{
			UserName:             "alice",
			NowPlayingItemName:   "Movie A",
			NowPlayingItemId:     "item-1",
			PlaybackDuration:     3600,
			ActivityDateInserted: "2024-06-15T20:00:00.000Z",
			PlayMethod:           "DirectPlay",
		},
		{
			UserName:             "bob",
			NowPlayingItemName:   "Movie B",
			NowPlayingItemId:     "item-2",
			PlaybackDuration:     1800,
			ActivityDateInserted: "2024-06-15T21:00:00.000Z",
			PlayMethod:           "Transcode",
		},
	}

	mockJS := mockJellystatServerWithHistory(t, records)
	mux, st := newTestServerWrapped(t)

	jf := &models.Server{Name: "JF Test", Type: models.ServerTypeJellyfin, URL: "http://test", APIKey: "k", Enabled: true}
	st.CreateServer(jf)
	configureJellystat(t, st, mockJS.URL)

	body := fmt.Sprintf(`{"server_id":%d}`, jf.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/settings/jellystat/import", strings.NewReader(body))
	w := &flushRecorder{httptest.NewRecorder()}
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	var events []importProgressEvent
	scanner := bufio.NewScanner(w.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var evt importProgressEvent
		if err := json.Unmarshal([]byte(line[6:]), &evt); err != nil {
			t.Fatalf("parsing SSE event: %v", err)
		}
		events = append(events, evt)
	}

	if len(events) == 0 {
		t.Fatal("expected at least one SSE event")
	}

	last := events[len(events)-1]
	if last.Type != "complete" {
		t.Errorf("last event type = %q, want complete", last.Type)
	}
	if last.Inserted != 2 {
		t.Errorf("last event inserted = %d, want 2", last.Inserted)
	}
	if last.Total != 2 {
		t.Errorf("last event total = %d, want 2", last.Total)
	}
	if last.Processed != 2 {
		t.Errorf("last event processed = %d, want 2", last.Processed)
	}
}
