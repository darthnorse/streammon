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
	sessionsData, err := os.ReadFile("testdata/sessions.xml")
	if err != nil {
		t.Fatal(err)
	}

	// Metadata response for ratingKey 12345 - provides original media info
	metadataXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer>
  <Video ratingKey="12345">
    <Media id="1001" container="mkv" videoCodec="h264" audioCodec="aac" videoResolution="1080" bitrate="10000" audioChannels="6" />
  </Video>
</MediaContainer>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "test-token" {
			t.Error("missing plex token header")
		}
		switch r.URL.Path {
		case "/status/sessions":
			w.Write(sessionsData)
		case "/library/metadata/12345":
			w.Write([]byte(metadataXML))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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
	if !s.TranscodeHWDecode {
		t.Error("expected HW decode to be true")
	}
	if s.TranscodeHWEncode {
		t.Error("expected HW encode to be false")
	}
	if s.TranscodeProgress != 40.5 {
		t.Errorf("transcode progress = %f, want 40.5", s.TranscodeProgress)
	}
	if s.Bandwidth != 12000000 {
		t.Errorf("bandwidth = %d, want 12000000", s.Bandwidth)
	}
	if s.TranscodeVideoResolution != "720p" {
		t.Errorf("transcode resolution = %q, want 720p", s.TranscodeVideoResolution)
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
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer machineIdentifier="abc123" version="1.40.0"/>`))
	}))
	defer ts.Close()

	srv := New(models.Server{URL: ts.URL, APIKey: "tok"})
	if err := srv.TestConnection(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestGetIdentity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer machineIdentifier="xyz789" version="1.40.0"/>`))
	}))
	defer ts.Close()

	srv := New(models.Server{URL: ts.URL, APIKey: "tok"})
	info, err := srv.GetIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.MachineIdentifier != "xyz789" {
		t.Errorf("machine_id = %q, want xyz789", info.MachineIdentifier)
	}
	if info.Version != "1.40.0" {
		t.Errorf("version = %q, want 1.40.0", info.Version)
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
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer>
  <Video sessionKey="42" type="movie" title="Test">
    <Player title="TV" product="Plex" address="10.0.0.1"/>
    <User title="bob"/>
  </Video>
</MediaContainer>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(xmlData))
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, Name: "srv", URL: ts.URL, APIKey: "tok"})
	sessions, err := srv.GetSessions(context.Background())
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

func TestMetadataFallbackOnNotFound(t *testing.T) {
	// Session with TranscodeSession but metadata returns 404
	// Should fall back to TranscodeSession.SourceVideoCodec
	sessionsXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer>
  <Video sessionKey="1" ratingKey="99999" type="movie" title="Test">
    <Media videoCodec="output_codec" videoResolution="720" />
    <TranscodeSession videoDecision="transcode" sourceVideoCodec="h265" sourceAudioCodec="dts" />
    <Player title="TV" product="Plex" address="10.0.0.1"/>
    <User title="bob"/>
  </Video>
</MediaContainer>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status/sessions":
			w.Write([]byte(sessionsXML))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	sessions, err := srv.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	// Should use fallback to TranscodeSession.SourceVideoCodec
	if sessions[0].VideoCodec != "h265" {
		t.Errorf("video codec = %q, want h265 (from TranscodeSession fallback)", sessions[0].VideoCodec)
	}
	if sessions[0].AudioCodec != "dts" {
		t.Errorf("audio codec = %q, want dts (from TranscodeSession fallback)", sessions[0].AudioCodec)
	}
}

func TestMetadataFallbackOnMalformedXML(t *testing.T) {
	sessionsXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer>
  <Video sessionKey="1" ratingKey="12345" type="movie" title="Test">
    <Media videoCodec="session_codec" videoResolution="720" />
    <TranscodeSession videoDecision="transcode" sourceVideoCodec="original_codec" />
    <Player title="TV" product="Plex" address="10.0.0.1"/>
    <User title="bob"/>
  </Video>
</MediaContainer>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status/sessions":
			w.Write([]byte(sessionsXML))
		case "/library/metadata/12345":
			w.Write([]byte("not valid xml {{{"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	sessions, err := srv.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	// Should use fallback to TranscodeSession.SourceVideoCodec
	if sessions[0].VideoCodec != "original_codec" {
		t.Errorf("video codec = %q, want original_codec (from TranscodeSession fallback)", sessions[0].VideoCodec)
	}
}

func TestMetadataCaching(t *testing.T) {
	sessionsXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer>
  <Video sessionKey="1" ratingKey="cached123" type="movie" title="Test">
    <Media videoCodec="session_codec" videoResolution="720" />
    <TranscodeSession videoDecision="transcode" />
    <Player title="TV" product="Plex" address="10.0.0.1"/>
    <User title="bob"/>
  </Video>
</MediaContainer>`

	metadataXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer>
  <Video ratingKey="cached123">
    <Media videoCodec="cached_codec" videoResolution="1080" />
  </Video>
</MediaContainer>`

	metadataCallCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status/sessions":
			w.Write([]byte(sessionsXML))
		case "/library/metadata/cached123":
			metadataCallCount++
			w.Write([]byte(metadataXML))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})

	// First call
	sessions, err := srv.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sessions[0].VideoCodec != "cached_codec" {
		t.Errorf("first call: video codec = %q, want cached_codec", sessions[0].VideoCodec)
	}

	// Second call - should use cache
	sessions, err = srv.GetSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sessions[0].VideoCodec != "cached_codec" {
		t.Errorf("second call: video codec = %q, want cached_codec", sessions[0].VideoCodec)
	}

	// Metadata should only be fetched once due to caching
	if metadataCallCount != 1 {
		t.Errorf("metadata fetched %d times, want 1 (should be cached)", metadataCallCount)
	}
}

func TestContextCancellation(t *testing.T) {
	sessionsXML := `<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer>
  <Video sessionKey="1" ratingKey="slow123" type="movie" title="Test1">
    <Media videoCodec="codec1" />
    <TranscodeSession videoDecision="transcode" />
    <Player title="TV" product="Plex" address="10.0.0.1"/>
    <User title="bob"/>
  </Video>
  <Video sessionKey="2" ratingKey="slow456" type="movie" title="Test2">
    <Media videoCodec="codec2" />
    <TranscodeSession videoDecision="transcode" />
    <Player title="TV2" product="Plex" address="10.0.0.2"/>
    <User title="alice"/>
  </Video>
</MediaContainer>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status/sessions":
			w.Write([]byte(sessionsXML))
		default:
			// Metadata endpoints - don't respond, let context cancel
			<-r.Context().Done()
		}
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := srv.GetSessions(ctx)
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestDeleteItem(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/library/metadata/12345" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Plex-Token") != "tok" {
			t.Error("missing auth header")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	srv := New(models.Server{URL: ts.URL, APIKey: "tok"})
	err := srv.DeleteItem(context.Background(), "12345")
	if err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
}

func TestDeleteItemError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	srv := New(models.Server{URL: ts.URL, APIKey: "tok"})
	err := srv.DeleteItem(context.Background(), "12345")
	if err == nil {
		t.Error("expected error for 403")
	}
}

func TestDeriveDynamicRange(t *testing.T) {
	tests := []struct {
		name   string
		stream plexStream
		want   string
	}{
		{
			name:   "SDR default",
			stream: plexStream{},
			want:   "SDR",
		},
		{
			name:   "Dolby Vision with profile",
			stream: plexStream{DOVIPresent: "1", DOVIProfile: "5"},
			want:   "Dolby Vision 5",
		},
		{
			name:   "Dolby Vision without profile",
			stream: plexStream{DOVIPresent: "1"},
			want:   "Dolby Vision",
		},
		{
			name:   "HDR10 via colorTrc smpte2084",
			stream: plexStream{ColorSpace: "bt2020", ColorTrc: "smpte2084"},
			want:   "HDR10",
		},
		{
			name:   "HLG via colorTrc arib-std-b67",
			stream: plexStream{ColorSpace: "bt2020", ColorTrc: "arib-std-b67"},
			want:   "HLG",
		},
		{
			name:   "Generic HDR via bt2020 colorspace",
			stream: plexStream{ColorSpace: "bt2020"},
			want:   "HDR",
		},
		{
			name:   "HDR10 via bit depth only",
			stream: plexStream{BitDepth: "10", ColorTrc: "smpte2084"},
			want:   "HDR10",
		},
		{
			name:   "SDR with 8-bit depth",
			stream: plexStream{BitDepth: "8"},
			want:   "SDR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveDynamicRange(tt.stream)
			if got != tt.want {
				t.Errorf("deriveDynamicRange() = %q, want %q", got, tt.want)
			}
		})
	}
}
