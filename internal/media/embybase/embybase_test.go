package embybase

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"streammon/internal/models"
)

func TestGetSessions(t *testing.T) {
	data, err := os.ReadFile("testdata/sessions.json")
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Emby-Token") != "test-key" {
			t.Error("missing X-Emby-Token header")
		}
		if r.URL.Path != "/Sessions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer ts.Close()

	c := New(models.Server{
		ID:     1,
		Name:   "TestEmby",
		URL:    ts.URL,
		APIKey: "test-key",
	}, models.ServerTypeEmby)

	sessions, err := c.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions (idle excluded), got %d", len(sessions))
	}

	s := sessions[0]
	if s.SessionID != "sess1" {
		t.Errorf("session id = %q, want sess1", s.SessionID)
	}
	if s.UserName != "alice" {
		t.Errorf("user = %q, want alice", s.UserName)
	}
	if s.MediaType != models.MediaTypeMovie {
		t.Errorf("media type = %q, want movie", s.MediaType)
	}
	if s.Title != "Inception" {
		t.Errorf("title = %q, want Inception", s.Title)
	}
	if s.DurationMs != 8880000 {
		t.Errorf("duration = %d, want 8880000", s.DurationMs)
	}
	if s.ProgressMs != 3600000 {
		t.Errorf("progress = %d, want 3600000", s.ProgressMs)
	}
	if s.VideoCodec != "h264" {
		t.Errorf("video codec = %q, want h264", s.VideoCodec)
	}
	if s.AudioCodec != "aac" {
		t.Errorf("audio codec = %q, want aac", s.AudioCodec)
	}
	if s.AudioChannels != 6 {
		t.Errorf("audio channels = %d, want 6", s.AudioChannels)
	}
	if s.Container != "mkv" {
		t.Errorf("container = %q, want mkv", s.Container)
	}
	if s.SubtitleCodec != "srt" {
		t.Errorf("subtitle codec = %q, want srt", s.SubtitleCodec)
	}
	if s.VideoDecision != models.TranscodeDecisionTranscode {
		t.Errorf("video decision = %q, want transcode", s.VideoDecision)
	}
	if s.AudioDecision != models.TranscodeDecisionDirectPlay {
		t.Errorf("audio decision = %q, want direct play", s.AudioDecision)
	}
	if !s.TranscodeHWAccel {
		t.Error("expected HW accel true (vaapi)")
	}
	if s.TranscodeProgress != 55.2 {
		t.Errorf("transcode progress = %f, want 55.2", s.TranscodeProgress)
	}
	if s.VideoResolution != "1080p" {
		t.Errorf("resolution = %q, want 1080p", s.VideoResolution)
	}

	s2 := sessions[1]
	if s2.MediaType != models.MediaTypeTV {
		t.Errorf("session 2 media type = %q, want episode", s2.MediaType)
	}
	if s2.GrandparentTitle != "Breaking Bad" {
		t.Errorf("grandparent = %q, want Breaking Bad", s2.GrandparentTitle)
	}
	if s2.VideoDecision != models.TranscodeDecisionDirectPlay {
		t.Errorf("s2 video decision = %q, want direct play", s2.VideoDecision)
	}
}

func TestTestConnection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/System/Info/Public" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "tok"}, models.ServerTypeEmby)
	if err := c.TestConnection(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestEmptySessionsList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "k"}, models.ServerTypeEmby)
	sessions, err := c.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestMediaTypeMappings(t *testing.T) {
	tests := []struct {
		embyType string
		want     models.MediaType
	}{
		{"Movie", models.MediaTypeMovie},
		{"Episode", models.MediaTypeTV},
		{"Audio", models.MediaTypeMusic},
		{"TvChannel", models.MediaTypeLiveTV},
		{"AudioBook", models.MediaTypeAudiobook},
		{"Book", models.MediaTypeBook},
		{"MusicVideo", models.MediaTypeMovie},
		{"Video", models.MediaTypeMovie},
	}
	for _, tt := range tests {
		t.Run(tt.embyType, func(t *testing.T) {
			data := fmt.Sprintf(`[{"Id":"s1","UserName":"u","Client":"c","DeviceName":"d","RemoteEndPoint":"1.2.3.4",
				"NowPlayingItem":{"Name":"Test","Type":"%s","RunTimeTicks":100000000},"PlayState":{"PositionTicks":0}}]`, tt.embyType)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(data))
			}))
			defer ts.Close()

			c := New(models.Server{URL: ts.URL, APIKey: "k"}, models.ServerTypeEmby)
			sessions, err := c.GetSessions(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(sessions) != 1 {
				t.Fatalf("expected 1 session, got %d", len(sessions))
			}
			if sessions[0].MediaType != tt.want {
				t.Errorf("got %q, want %q", sessions[0].MediaType, tt.want)
			}
		})
	}
}

func TestIdleSessionsExcluded(t *testing.T) {
	data := `[{"Id":"idle","UserName":"x","Client":"c","DeviceName":"d","RemoteEndPoint":"1.2.3.4","NowPlayingItem":null}]`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(data))
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "k"}, models.ServerTypeEmby)
	sessions, err := c.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestGetRecentlyAdded(t *testing.T) {
	data, err := os.ReadFile("testdata/recently_added.json")
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Emby-Token") != "test-key" {
			t.Error("missing X-Emby-Token header")
		}
		if r.URL.Path != "/Items" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("SortBy") != "DateCreated" {
			t.Error("missing SortBy=DateCreated query param")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer ts.Close()

	c := New(models.Server{
		ID:     1,
		Name:   "TestEmby",
		URL:    ts.URL,
		APIKey: "test-key",
	}, models.ServerTypeEmby)

	items, err := c.GetRecentlyAdded(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	item := items[0]
	if item.Title != "Oppenheimer" {
		t.Errorf("title = %q, want Oppenheimer", item.Title)
	}
	if item.Year != 2023 {
		t.Errorf("year = %d, want 2023", item.Year)
	}
	if item.MediaType != models.MediaTypeMovie {
		t.Errorf("media type = %q, want movie", item.MediaType)
	}
	if item.ThumbURL != "item1" {
		t.Errorf("thumb url = %q, want item1", item.ThumbURL)
	}
	if item.ServerID != 1 {
		t.Errorf("server id = %d, want 1", item.ServerID)
	}
	if item.ServerName != "TestEmby" {
		t.Errorf("server name = %q, want TestEmby", item.ServerName)
	}
	if item.ServerType != models.ServerTypeEmby {
		t.Errorf("server type = %q, want emby", item.ServerType)
	}

	item2 := items[1]
	if item2.Title != "Breaking Bad - Ozymandias" {
		t.Errorf("title = %q, want Breaking Bad - Ozymandias", item2.Title)
	}
	if item2.MediaType != models.MediaTypeTV {
		t.Errorf("media type = %q, want episode", item2.MediaType)
	}

	item3 := items[2]
	if item3.ThumbURL != "" {
		t.Errorf("thumb url = %q, want empty (no Primary tag)", item3.ThumbURL)
	}
}

func TestGetRecentlyAddedEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Items":[]}`))
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "k"}, models.ServerTypeEmby)
	items, err := c.GetRecentlyAdded(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestGetRecentlyAddedError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := New(models.Server{URL: ts.URL, APIKey: "bad"}, models.ServerTypeEmby)
	_, err := c.GetRecentlyAdded(context.Background(), 10)
	if err == nil {
		t.Error("expected error for 401")
	}
}
