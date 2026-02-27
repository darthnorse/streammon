package jellystat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func mockJellystat(t *testing.T, apiKey string, pages [][]HistoryRecord) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-token") != apiKey {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"message": "Authentication failed."})
			return
		}

		switch {
		case r.URL.Path == "/api/getconfig":
			json.NewEncoder(w).Encode(map[string]any{
				"JF_HOST":  "http://localhost:8096",
				"APP_USER": "admin",
			})
		case r.URL.Path == "/api/getHistory":
			pageStr := r.URL.Query().Get("page")
			page := 1
			if pageStr != "" {
				if p, err := strconv.Atoi(pageStr); err == nil {
					page = p
				}
			}
			if page < 1 || page > len(pages) {
				json.NewEncoder(w).Encode(historyPage{
					CurrentPage: page,
					Pages:       len(pages),
					Results:     []HistoryRecord{},
				})
				return
			}
			json.NewEncoder(w).Encode(historyPage{
				CurrentPage: page,
				Pages:       len(pages),
				Size:        len(pages[page-1]),
				Results:     pages[page-1],
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestNewClient_ValidURL(t *testing.T) {
	c, err := NewClient("http://localhost:3000", "key")
	if err != nil {
		t.Fatal(err)
	}
	if c.baseURL != "http://localhost:3000" {
		t.Fatalf("unexpected baseURL: %s", c.baseURL)
	}
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	c, err := NewClient("http://localhost:3000/", "key")
	if err != nil {
		t.Fatal(err)
	}
	if c.baseURL != "http://localhost:3000" {
		t.Fatalf("expected trimmed URL, got %s", c.baseURL)
	}
}

func TestNewClient_InvalidURL(t *testing.T) {
	_, err := NewClient("not-a-url", "key")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestTestConnection_Success(t *testing.T) {
	ts := mockJellystat(t, "valid-key", nil)
	c, _ := NewClient(ts.URL, "valid-key")
	if err := c.TestConnection(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestConnection_BadKey(t *testing.T) {
	ts := mockJellystat(t, "valid-key", nil)
	c, _ := NewClient(ts.URL, "wrong-key")
	err := c.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected error for wrong key")
	}
}

func TestGetHistory_SinglePage(t *testing.T) {
	pages := [][]HistoryRecord{{
		{UserName: "alice", NowPlayingItemName: "Movie A", PlaybackDuration: 100},
		{UserName: "bob", NowPlayingItemName: "Movie B", PlaybackDuration: 200},
	}}
	ts := mockJellystat(t, "key", pages)
	c, _ := NewClient(ts.URL, "key")

	records, totalPages, err := c.GetHistory(context.Background(), 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if totalPages != 1 {
		t.Fatalf("expected 1 page, got %d", totalPages)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].UserName != "alice" {
		t.Fatalf("expected alice, got %s", records[0].UserName)
	}
}

func TestGetHistory_EmptyPage(t *testing.T) {
	ts := mockJellystat(t, "key", [][]HistoryRecord{})
	c, _ := NewClient(ts.URL, "key")

	records, _, err := c.GetHistory(context.Background(), 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

func TestStreamHistory_MultiPage(t *testing.T) {
	pages := [][]HistoryRecord{
		{{UserName: "alice", NowPlayingItemName: "Movie A"}},
		{{UserName: "bob", NowPlayingItemName: "Movie B"}},
	}
	ts := mockJellystat(t, "key", pages)
	c, _ := NewClient(ts.URL, "key")

	var batches []BatchResult
	err := c.StreamHistory(context.Background(), 10, func(batch BatchResult) error {
		batches = append(batches, batch)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}
	if batches[0].Records[0].UserName != "alice" {
		t.Fatalf("expected alice in batch 1, got %s", batches[0].Records[0].UserName)
	}
	if batches[1].Records[0].UserName != "bob" {
		t.Fatalf("expected bob in batch 2, got %s", batches[1].Records[0].UserName)
	}
}

func TestStreamHistory_ContextCancelled(t *testing.T) {
	pages := [][]HistoryRecord{
		{{UserName: "alice"}},
		{{UserName: "bob"}},
	}
	ts := mockJellystat(t, "key", pages)
	c, _ := NewClient(ts.URL, "key")

	ctx, cancel := context.WithCancel(context.Background())
	var count int
	err := c.StreamHistory(ctx, 10, func(batch BatchResult) error {
		count++
		cancel()
		return nil
	})
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
	if count != 1 {
		t.Fatalf("expected 1 batch before cancel, got %d", count)
	}
}

func TestHistoryRecord_NullFields(t *testing.T) {
	raw := `{
		"UserName": "alice",
		"NowPlayingItemName": "Die Hard",
		"SeriesName": null,
		"SeasonNumber": null,
		"EpisodeNumber": null,
		"TranscodingInfo": null,
		"PlaybackDuration": 120.5,
		"ActivityDateInserted": "2024-01-15T20:30:00.000Z"
	}`

	var rec HistoryRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.SeriesName != nil {
		t.Fatal("expected nil SeriesName")
	}
	if rec.SeasonNumber != nil {
		t.Fatal("expected nil SeasonNumber")
	}
	if rec.TranscodingInfo != nil {
		t.Fatal("expected nil TranscodingInfo")
	}
	if rec.PlaybackDuration != 120.5 {
		t.Fatalf("expected 120.5, got %f", rec.PlaybackDuration)
	}
}

func TestHistoryRecord_TVFields(t *testing.T) {
	seriesName := "Breaking Bad"
	seasonNum := 3
	episodeNum := 5
	raw := `{
		"UserName": "bob",
		"NowPlayingItemName": "Caballo sin Nombre",
		"SeriesName": "Breaking Bad",
		"SeasonNumber": 3,
		"EpisodeNumber": 5,
		"PlaybackDuration": 2800
	}`

	var rec HistoryRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatal(err)
	}
	if rec.SeriesName == nil || *rec.SeriesName != seriesName {
		t.Fatalf("expected %q, got %v", seriesName, rec.SeriesName)
	}
	if rec.SeasonNumber == nil || *rec.SeasonNumber != seasonNum {
		t.Fatalf("expected %d, got %v", seasonNum, rec.SeasonNumber)
	}
	if rec.EpisodeNumber == nil || *rec.EpisodeNumber != episodeNum {
		t.Fatalf("expected %d, got %v", episodeNum, rec.EpisodeNumber)
	}
}
