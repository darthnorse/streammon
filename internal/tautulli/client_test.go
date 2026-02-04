package tautulli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c, err := NewClient("http://localhost:8181", "mykey")
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c.baseURL != "http://localhost:8181" {
		t.Errorf("expected baseURL http://localhost:8181, got %s", c.baseURL)
	}
	if c.apiKey != "mykey" {
		t.Errorf("expected apiKey mykey, got %s", c.apiKey)
	}
}

func TestNewClientTrimsTrailingSlash(t *testing.T) {
	c, err := NewClient("http://localhost:8181/", "mykey")
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c.baseURL != "http://localhost:8181" {
		t.Errorf("expected baseURL without trailing slash, got %s", c.baseURL)
	}
}

func TestNewClientValidatesURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http", "http://localhost:8181", false},
		{"valid https", "https://tautulli.example.com", false},
		{"empty url", "", true},
		{"no scheme", "localhost:8181", true},
		{"invalid scheme", "ftp://localhost:8181", true},
		{"no host", "http://", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.url, "key")
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestTestConnection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cmd") != "get_server_info" {
			t.Errorf("expected cmd=get_server_info, got %s", r.URL.Query().Get("cmd"))
		}
		if r.URL.Query().Get("apikey") != "testkey" {
			t.Errorf("expected apikey=testkey, got %s", r.URL.Query().Get("apikey"))
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": map[string]interface{}{
				"result":  "success",
				"message": "",
			},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "testkey")
	err := c.TestConnection(context.Background())
	if err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}
}

func TestTestConnectionFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": map[string]interface{}{
				"result":  "error",
				"message": "Invalid apikey",
			},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "badkey")
	err := c.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid API key")
	}
}

func TestGetHistory(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cmd") != "get_history" {
			t.Errorf("expected cmd=get_history, got %s", r.URL.Query().Get("cmd"))
		}
		start := r.URL.Query().Get("start")
		length := r.URL.Query().Get("length")
		if start != "0" || length != "100" {
			t.Errorf("expected start=0, length=100, got start=%s, length=%s", start, length)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": map[string]interface{}{
				"result":  "success",
				"message": "",
				"data": map[string]interface{}{
					"recordsFiltered": 2,
					"recordsTotal":    2,
					"data": []map[string]interface{}{
						{
							"user":          "alice",
							"title":         "Test Movie",
							"media_type":    "movie",
							"year":          2023,
							"rating_key":    12345,
							"started":       1700000000,
							"stopped":       1700007200,
							"duration":      7200,
							"play_duration": 7000,
							"player":        "Chrome",
							"platform":      "Windows",
							"ip_address":    "192.168.1.100",
						},
						{
							"user":                   "bob",
							"title":                  "Episode 1",
							"media_type":             "episode",
							"grandparent_title":      "Test Show",
							"parent_title":           "Season 1",
							"year":                   2022,
							"rating_key":             67890,
							"grandparent_rating_key": 11111,
							"started":                1700010000,
							"stopped":                1700013600,
							"duration":               3600,
							"play_duration":          3500,
							"player":                 "Roku",
							"platform":               "Roku",
							"ip_address":             "192.168.1.101",
							"parent_media_index":     1,
							"media_index":            1,
						},
					},
				},
			},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "testkey")
	records, total, err := c.GetHistory(context.Background(), 0, 100)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.User != "alice" {
		t.Errorf("expected user=alice, got %s", r.User)
	}
	if r.Title != "Test Movie" {
		t.Errorf("expected title=Test Movie, got %s", r.Title)
	}
	if r.MediaType != "movie" {
		t.Errorf("expected media_type=movie, got %s", r.MediaType)
	}
	if r.Year != 2023 {
		t.Errorf("expected year=2023, got %d", r.Year)
	}
	if r.Started != 1700000000 {
		t.Errorf("expected started=1700000000, got %d", r.Started)
	}
	if r.Duration != 7200 {
		t.Errorf("expected duration=7200, got %d", r.Duration)
	}

	r2 := records[1]
	if r2.GrandparentTitle != "Test Show" {
		t.Errorf("expected grandparent_title=Test Show, got %s", r2.GrandparentTitle)
	}
	if r2.ParentMediaIndex != 1 {
		t.Errorf("expected parent_media_index=1, got %d", r2.ParentMediaIndex)
	}
	if r2.MediaIndex != 1 {
		t.Errorf("expected media_index=1, got %d", r2.MediaIndex)
	}
}

func TestStreamHistoryPagination(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		start := r.URL.Query().Get("start")

		var data []map[string]interface{}
		if start == "0" {
			for i := 0; i < 1000; i++ {
				data = append(data, map[string]interface{}{
					"user":       "user",
					"title":      "Movie",
					"media_type": "movie",
					"started":    1700000000 + i,
					"stopped":    1700001000 + i,
					"duration":   1000,
				})
			}
		} else {
			for i := 0; i < 500; i++ {
				data = append(data, map[string]interface{}{
					"user":       "user",
					"title":      "Movie",
					"media_type": "movie",
					"started":    1701000000 + i,
					"stopped":    1701001000 + i,
					"duration":   1000,
				})
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": map[string]interface{}{
				"result":  "success",
				"message": "",
				"data": map[string]interface{}{
					"recordsFiltered": len(data),
					"recordsTotal":    1500,
					"data":            data,
				},
			},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "testkey")
	var totalRecords int
	err := c.StreamHistory(context.Background(), 1000, func(batch BatchResult) error {
		totalRecords += len(batch.Records)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamHistory failed: %v", err)
	}
	if totalRecords != 1500 {
		t.Errorf("expected 1500 records, got %d", totalRecords)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestStreamHistoryContextCancellation(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var data []map[string]interface{}
		for i := 0; i < 100; i++ {
			data = append(data, map[string]interface{}{
				"user":       "user",
				"title":      "Movie",
				"media_type": "movie",
				"started":    1700000000 + i,
				"stopped":    1700001000 + i,
				"duration":   1000,
			})
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": map[string]interface{}{
				"result":  "success",
				"message": "",
				"data": map[string]interface{}{
					"recordsFiltered": len(data),
					"recordsTotal":    10000,
					"data":            data,
				},
			},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "testkey")
	ctx, cancel := context.WithCancel(context.Background())

	batchCount := 0
	err := c.StreamHistory(ctx, 100, func(batch BatchResult) error {
		batchCount++
		if batchCount >= 2 {
			cancel()
		}
		return nil
	})

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if batchCount < 2 {
		t.Errorf("expected at least 2 batches before cancellation, got %d", batchCount)
	}
}

func TestGetStreamData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cmd") != "get_stream_data" {
			t.Errorf("expected cmd=get_stream_data, got %s", r.URL.Query().Get("cmd"))
		}
		if r.URL.Query().Get("session_key") != "12345" {
			t.Errorf("expected session_key=12345, got %s", r.URL.Query().Get("session_key"))
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": map[string]interface{}{
				"result":  "success",
				"message": "",
				"data": map[string]interface{}{
					"video_codec":          "hevc",
					"video_width":          3840,
					"video_height":         2160,
					"video_bit_depth":      10,
					"video_dynamic_range":  "HDR",
					"audio_codec":          "truehd",
					"audio_channels":       8,
					"bandwidth":            50000,
					"transcode_decision":   "direct play",
					"video_decision":       "direct play",
					"audio_decision":       "transcode",
					"transcode_hw_decoding": true,
					"transcode_hw_encoding": false,
				},
			},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "testkey")
	sd, err := c.GetStreamData(context.Background(), "12345")
	if err != nil {
		t.Fatalf("GetStreamData failed: %v", err)
	}
	if sd == nil {
		t.Fatal("expected stream data, got nil")
	}
	if sd.VideoCodec != "hevc" {
		t.Errorf("video_codec = %q, want hevc", sd.VideoCodec)
	}
	if sd.VideoWidth != 3840 {
		t.Errorf("video_width = %d, want 3840", sd.VideoWidth)
	}
	if sd.VideoHeight != 2160 {
		t.Errorf("video_height = %d, want 2160", sd.VideoHeight)
	}
	if sd.VideoBitDepth != 10 {
		t.Errorf("video_bit_depth = %d, want 10", sd.VideoBitDepth)
	}
	if sd.VideoDynamicRange != "HDR" {
		t.Errorf("video_dynamic_range = %q, want HDR", sd.VideoDynamicRange)
	}
	if sd.AudioCodec != "truehd" {
		t.Errorf("audio_codec = %q, want truehd", sd.AudioCodec)
	}
	if sd.AudioChannels != 8 {
		t.Errorf("audio_channels = %d, want 8", sd.AudioChannels)
	}
	if sd.Bandwidth != 50000 {
		t.Errorf("bandwidth = %d, want 50000", sd.Bandwidth)
	}
	if sd.VideoDecision != "direct play" {
		t.Errorf("video_decision = %q, want direct play", sd.VideoDecision)
	}
	if sd.AudioDecision != "transcode" {
		t.Errorf("audio_decision = %q, want transcode", sd.AudioDecision)
	}
	if !sd.TranscodeHWDecode {
		t.Error("transcode_hw_decode should be true")
	}
	if sd.TranscodeHWEncode {
		t.Error("transcode_hw_encode should be false")
	}
}

func TestGetStreamDataEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": map[string]interface{}{
				"result":  "success",
				"message": "",
				"data":    nil,
			},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "testkey")
	sd, err := c.GetStreamData(context.Background(), "99999")
	if err != nil {
		t.Fatalf("GetStreamData failed: %v", err)
	}
	if sd != nil {
		t.Errorf("expected nil for empty data, got %+v", sd)
	}
}

func TestGetStreamDataFlexibleTypes(t *testing.T) {
	// Test that string values for numeric fields are handled correctly
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": map[string]interface{}{
				"result":  "success",
				"message": "",
				"data": map[string]interface{}{
					"video_codec":          "h264",
					"video_width":          "1920",  // string instead of int
					"video_height":         "1080",  // string instead of int
					"audio_channels":       "6",     // string instead of int
					"bandwidth":            "25000", // string instead of int
					"transcode_hw_decoding": "1",   // string "1" for boolean
					"transcode_hw_encoding": 0,     // int 0 for boolean
				},
			},
		})
	}))
	defer ts.Close()

	c, _ := NewClient(ts.URL, "testkey")
	sd, err := c.GetStreamData(context.Background(), "12345")
	if err != nil {
		t.Fatalf("GetStreamData failed: %v", err)
	}
	if sd.VideoWidth != 1920 {
		t.Errorf("video_width = %d, want 1920", sd.VideoWidth)
	}
	if sd.VideoHeight != 1080 {
		t.Errorf("video_height = %d, want 1080", sd.VideoHeight)
	}
	if sd.AudioChannels != 6 {
		t.Errorf("audio_channels = %d, want 6", sd.AudioChannels)
	}
	if sd.Bandwidth != 25000 {
		t.Errorf("bandwidth = %d, want 25000", sd.Bandwidth)
	}
	if !sd.TranscodeHWDecode {
		t.Error("transcode_hw_decode should be true for string '1'")
	}
	if sd.TranscodeHWEncode {
		t.Error("transcode_hw_encode should be false for int 0")
	}
}

func TestHeightToResolution(t *testing.T) {
	tests := []struct {
		height int
		want   string
	}{
		{2160, "4K"},
		{1080, "1080p"},
		{720, "720p"},
		{480, "480p"},
		{360, "360p"},
		{0, ""},
	}

	for _, tt := range tests {
		got := HeightToResolution(tt.height)
		if got != tt.want {
			t.Errorf("HeightToResolution(%d) = %q, want %q", tt.height, got, tt.want)
		}
	}
}
