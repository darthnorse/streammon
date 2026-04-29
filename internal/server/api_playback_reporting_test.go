package server

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/models"
)

func TestParsePlaybackReportingTSV_Emby(t *testing.T) {
	userMap := map[string]string{"user1": "Alice", "user2": "Bob"}
	input := []byte(
		"2024-03-15T20:30:00.0000000Z\tuser1\titem123\tMovie\tInception\tDirectPlay\tVLC\tDesktop\t7200\t120\t192.168.1.100\tVideoCodec\n" +
			"2024-03-16T10:00:00.0000000Z\tuser2\titem456\tEpisode\tBreaking Bad - S01E01\tTranscode\tPlex Web\tChrome\t3600\t60\t10.0.0.5\tVideoCodecLevel\n",
	)

	entries := parsePlaybackReportingTSV(input, userMap, 1)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	e := entries[0]
	if e.UserName != "Alice" {
		t.Errorf("expected UserName=Alice, got %q", e.UserName)
	}
	if e.MediaType != models.MediaTypeMovie {
		t.Errorf("expected MediaType=movie, got %q", e.MediaType)
	}
	if e.Title != "Inception" {
		t.Errorf("expected Title=Inception, got %q", e.Title)
	}
	if e.WatchedMs != 7200*1000 {
		t.Errorf("expected WatchedMs=7200000, got %d", e.WatchedMs)
	}
	if e.Player != "VLC" {
		t.Errorf("expected Player=VLC, got %q", e.Player)
	}
	if e.Platform != "Desktop" {
		t.Errorf("expected Platform=Desktop, got %q", e.Platform)
	}
	if e.IPAddress != "192.168.1.100" {
		t.Errorf("expected IPAddress=192.168.1.100, got %q", e.IPAddress)
	}
	if e.PausedMs != 120*1000 {
		t.Errorf("expected PausedMs=120000, got %d", e.PausedMs)
	}
	if e.TranscodeDecision != models.TranscodeDecisionDirectPlay {
		t.Errorf("expected TranscodeDecision=direct play, got %q", e.TranscodeDecision)
	}
	if e.ItemID != "item123" {
		t.Errorf("expected ItemID=item123, got %q", e.ItemID)
	}
	if e.ServerID != 1 {
		t.Errorf("expected ServerID=1, got %d", e.ServerID)
	}
	expectedStart := time.Date(2024, 3, 15, 20, 30, 0, 0, time.UTC)
	if !e.StartedAt.Equal(expectedStart) {
		t.Errorf("expected StartedAt=%v, got %v", expectedStart, e.StartedAt)
	}
	expectedStop := expectedStart.Add(7200 * time.Second)
	if !e.StoppedAt.Equal(expectedStop) {
		t.Errorf("expected StoppedAt=%v, got %v", expectedStop, e.StoppedAt)
	}

	// Second entry
	e2 := entries[1]
	if e2.UserName != "Bob" {
		t.Errorf("expected UserName=Bob, got %q", e2.UserName)
	}
	if e2.MediaType != models.MediaTypeTV {
		t.Errorf("expected MediaType=episode, got %q", e2.MediaType)
	}
	if e2.TranscodeDecision != models.TranscodeDecisionTranscode {
		t.Errorf("expected TranscodeDecision=transcode, got %q", e2.TranscodeDecision)
	}
}

func TestParsePlaybackReportingTSV_Jellyfin(t *testing.T) {
	userMap := map[string]string{"jfuser1": "Charlie"}
	input := []byte(
		"2024-06-01 14:00:00\tjfuser1\tjfitem789\tMovie\tThe Matrix\tDirectPlay\tmpv\tLinux\t5400\n",
	)

	entries := parsePlaybackReportingTSV(input, userMap, 2)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.UserName != "Charlie" {
		t.Errorf("expected UserName=Charlie, got %q", e.UserName)
	}
	if e.MediaType != models.MediaTypeMovie {
		t.Errorf("expected MediaType=movie, got %q", e.MediaType)
	}
	if e.Title != "The Matrix" {
		t.Errorf("expected Title=The Matrix, got %q", e.Title)
	}
	if e.WatchedMs != 5400*1000 {
		t.Errorf("expected WatchedMs=5400000, got %d", e.WatchedMs)
	}
	if e.IPAddress != "" {
		t.Errorf("expected empty IPAddress, got %q", e.IPAddress)
	}
	if e.PausedMs != 0 {
		t.Errorf("expected PausedMs=0, got %d", e.PausedMs)
	}
	if e.ServerID != 2 {
		t.Errorf("expected ServerID=2, got %d", e.ServerID)
	}
}

func TestParsePlaybackReportingTSV_UnknownUser(t *testing.T) {
	userMap := map[string]string{"known": "Alice"}
	input := []byte(
		"2024-01-01T00:00:00.0000000Z\tunknownuser\titem1\tMovie\tTest\tDirectPlay\tVLC\tPC\t100\t0\t1.2.3.4\t\n",
	)

	entries := parsePlaybackReportingTSV(input, userMap, 1)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for unknown user, got %d", len(entries))
	}
}

func TestParsePlaybackReportingTSV_EmptyFile(t *testing.T) {
	userMap := map[string]string{"u1": "Alice"}

	entries := parsePlaybackReportingTSV([]byte(""), userMap, 1)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}

	entries = parsePlaybackReportingTSV([]byte("\n\n\n"), userMap, 1)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for blank lines, got %d", len(entries))
	}
}

func TestParsePlaybackReportingTSV_MediaTypes(t *testing.T) {
	userMap := map[string]string{"u1": "Alice"}
	tests := []struct {
		itemType string
		want     models.MediaType
	}{
		{"Movie", models.MediaTypeMovie},
		{"Episode", models.MediaTypeTV},
		{"TvChannel", models.MediaTypeLiveTV},
		{"Audio", models.MediaTypeMusic},
		{"Unknown", models.MediaTypeMovie},
	}

	for _, tc := range tests {
		input := []byte(
			"2024-01-01 00:00:00\tu1\titem1\t" + tc.itemType + "\tTitle\tDirectPlay\tVLC\tPC\t100\n",
		)
		entries := parsePlaybackReportingTSV(input, userMap, 1)
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry for %s, got %d", tc.itemType, len(entries))
		}
		if entries[0].MediaType != tc.want {
			t.Errorf("for %s: expected MediaType=%q, got %q", tc.itemType, tc.want, entries[0].MediaType)
		}
	}
}

func TestParsePlaybackReportingTSV_PlaybackMethods(t *testing.T) {
	userMap := map[string]string{"u1": "Alice"}
	tests := []struct {
		method string
		want   models.TranscodeDecision
	}{
		{"Transcode (v:direct a:aac)", models.TranscodeDecisionTranscode},
		{"Transcode", models.TranscodeDecisionTranscode},
		{"DirectStream", models.TranscodeDecisionCopy},
		{"DirectPlay", models.TranscodeDecisionDirectPlay},
		{"SomethingUnknown", models.TranscodeDecisionDirectPlay},
	}

	for _, tc := range tests {
		input := []byte(
			"2024-01-01 00:00:00\tu1\titem1\tMovie\tTitle\t" + tc.method + "\tVLC\tPC\t100\n",
		)
		entries := parsePlaybackReportingTSV(input, userMap, 1)
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry for %s, got %d", tc.method, len(entries))
		}
		if entries[0].TranscodeDecision != tc.want {
			t.Errorf("for %s: expected %q, got %q", tc.method, tc.want, entries[0].TranscodeDecision)
		}
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Time
		wantErr bool
	}{
		// RFC3339Nano (Emby format)
		{"2024-03-15T20:30:00.0000000Z", time.Date(2024, 3, 15, 20, 30, 0, 0, time.UTC), false},
		// With fractional seconds
		{"2024-01-01T12:00:00.1234567Z", time.Date(2024, 1, 1, 12, 0, 0, 123456700, time.UTC), false},
		// With timezone offset
		{"2024-06-15T10:00:00+05:30", time.Date(2024, 6, 15, 4, 30, 0, 0, time.UTC), false},
		// Simple datetime (Jellyfin format)
		{"2024-06-01 14:00:00", time.Date(2024, 6, 1, 14, 0, 0, 0, time.UTC), false},
		// With leading/trailing whitespace
		{"  2024-01-01 00:00:00  ", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), false},
		// Malformed
		{"not-a-date", time.Time{}, true},
		{"", time.Time{}, true},
		{"2024-13-01 00:00:00", time.Time{}, true},
	}

	for _, tc := range tests {
		got, err := parseTimestamp(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseTimestamp(%q): expected error, got %v", tc.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseTimestamp(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if !got.Equal(tc.want) {
			t.Errorf("parseTimestamp(%q): got %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParsePlaybackReportingTSV_WrongFieldCount(t *testing.T) {
	userMap := map[string]string{"u1": "Alice"}
	input := []byte(
		"2024-01-01 00:00:00\tu1\titem1\tMovie\tTitle\n" + // 5 fields
			"2024-01-01 00:00:00\tu1\titem1\tMovie\tTitle\tDirectPlay\tVLC\tPC\t100\textra1\textra2\textra3\textra4\textra5\n", // 14 fields
	)

	entries := parsePlaybackReportingTSV(input, userMap, 1)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for wrong field counts, got %d", len(entries))
	}
}

func TestParsePlaybackReportingTSV_BadDuration(t *testing.T) {
	userMap := map[string]string{"u1": "Alice"}
	input := []byte(
		"2024-01-01 00:00:00\tu1\titem1\tMovie\tTitle\tDirectPlay\tVLC\tPC\tnot_a_number\n",
	)

	entries := parsePlaybackReportingTSV(input, userMap, 1)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for bad duration, got %d", len(entries))
	}
}

func TestPlaybackReportingImport_RejectsOversizedUpload(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	// Build a multipart body whose file part exceeds the 50 MiB cap.
	// 51 MiB of payload plus a small server_id field must be rejected
	// without ever being written to disk.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		defer pw.Close()
		defer mw.Close()
		mw.WriteField("server_id", "1")
		fw, _ := mw.CreateFormFile("file", "huge.tsv")
		oversize := bytes.Repeat([]byte("x"), (50<<20)+(1<<20)+1)
		fw.Write(oversize)
	}()

	req := httptest.NewRequest(http.MethodPost, "/api/settings/playback-reporting/import", pr)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized upload: status = %d, want 413", w.Code)
	}
}
