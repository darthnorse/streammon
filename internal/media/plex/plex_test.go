package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"streammon/internal/models"
)

func TestGetSessions(t *testing.T) {
	data, err := os.ReadFile("testdata/sessions.xml")
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "test-token" {
			t.Error("missing plex token header")
		}
		if r.URL.Path != "/status/sessions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write(data)
	}))
	defer ts.Close()

	srv := New(models.Server{
		ID:     1,
		Name:   "TestPlex",
		Type:   models.ServerTypePlex,
		URL:    ts.URL,
		APIKey: "test-token",
	})

	sessions, err := srv.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	s := sessions[0]
	if s.SessionID != "abc123" {
		t.Errorf("session id = %q, want abc123", s.SessionID)
	}
	if s.ServerID != 1 {
		t.Errorf("server id = %d, want 1", s.ServerID)
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
	if s.Year != 2010 {
		t.Errorf("year = %d, want 2010", s.Year)
	}
	if s.DurationMs != 8880000 {
		t.Errorf("duration = %d, want 8880000", s.DurationMs)
	}
	if s.ProgressMs != 3600000 {
		t.Errorf("progress = %d, want 3600000", s.ProgressMs)
	}
	if s.Player != "Chrome" {
		t.Errorf("player = %q, want Chrome", s.Player)
	}
	if s.Platform != "Plex Web" {
		t.Errorf("platform = %q, want Plex Web", s.Platform)
	}
	if s.IPAddress != "192.168.1.10" {
		t.Errorf("ip = %q, want 192.168.1.10", s.IPAddress)
	}
	if s.VideoCodec != "h264" {
		t.Errorf("video codec = %q, want h264", s.VideoCodec)
	}
	if s.AudioCodec != "aac" {
		t.Errorf("audio codec = %q, want aac", s.AudioCodec)
	}
	if s.VideoResolution != "1080p" {
		t.Errorf("resolution = %q, want 1080p", s.VideoResolution)
	}
	if s.Container != "mkv" {
		t.Errorf("container = %q, want mkv", s.Container)
	}
	if s.Bitrate != 10000000 {
		t.Errorf("bitrate = %d, want 10000000", s.Bitrate)
	}
	if s.AudioChannels != 6 {
		t.Errorf("audio channels = %d, want 6", s.AudioChannels)
	}
	if s.SubtitleCodec != "srt" {
		t.Errorf("subtitle codec = %q, want srt", s.SubtitleCodec)
	}
	if s.VideoDecision != models.TranscodeDecisionCopy {
		t.Errorf("video decision = %q, want copy", s.VideoDecision)
	}
	if s.TranscodeHWAccel != true {
		t.Error("expected HW accel to be true")
	}
	if s.TranscodeProgress != 40.5 {
		t.Errorf("transcode progress = %f, want 40.5", s.TranscodeProgress)
	}
	if s.Bandwidth != 12000000 {
		t.Errorf("bandwidth = %d, want 12000000", s.Bandwidth)
	}

	s2 := sessions[1]
	if s2.MediaType != models.MediaTypeTV {
		t.Errorf("session 2 media type = %q, want episode", s2.MediaType)
	}
	if s2.GrandparentTitle != "Breaking Bad" {
		t.Errorf("grandparent = %q, want Breaking Bad", s2.GrandparentTitle)
	}
	if s2.VideoCodec != "hevc" {
		t.Errorf("s2 video codec = %q, want hevc", s2.VideoCodec)
	}
	if s2.VideoResolution != "4K" {
		t.Errorf("s2 resolution = %q, want 4K", s2.VideoResolution)
	}
	if s2.VideoDecision != models.TranscodeDecisionDirectPlay {
		t.Errorf("s2 video decision = %q, want direct play (no TranscodeSession)", s2.VideoDecision)
	}
}

func TestTestConnection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Plex-Token") == "" {
			t.Error("missing X-Plex-Token header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	srv := New(models.Server{URL: ts.URL, APIKey: "tok"})
	if err := srv.TestConnection(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTestConnectionFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	srv := New(models.Server{URL: ts.URL, APIKey: "bad"})
	if err := srv.TestConnection(context.Background()); err == nil {
		t.Error("expected error for 401")
	}
}

func TestSessionIDFallsBackToSessionKey(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer>
  <Video sessionKey="42" type="movie" title="Test">
    <Player title="TV" product="Plex" address="10.0.0.1"/>
    <User title="bob"/>
  </Video>
</MediaContainer>`
	sessions, err := parseSessions([]byte(xml), 1, "srv")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].SessionID != "42" {
		t.Errorf("session id = %q, want 42 (from sessionKey fallback)", sessions[0].SessionID)
	}
}

func TestEmptyContainer(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?><MediaContainer size="0"></MediaContainer>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(xml))
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	sessions, err := srv.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}
