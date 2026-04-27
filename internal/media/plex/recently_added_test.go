package plex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"streammon/internal/models"
)

func TestGetRecentlyAdded(t *testing.T) {
	movies, err := os.ReadFile("testdata/recently_added.xml")
	if err != nil {
		t.Fatal(err)
	}
	shows, err := os.ReadFile("testdata/recently_added_shows.xml")
	if err != nil {
		t.Fatal(err)
	}

	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "test-token" {
			t.Error("missing plex token header")
		}
		if r.URL.Path != "/hubs/home/recentlyAdded" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		requestCount++
		switch r.URL.Query().Get("type") {
		case "1":
			w.Write(movies)
		case "2":
			w.Write(shows)
		default:
			t.Errorf("unexpected type parameter: %s", r.URL.Query().Get("type"))
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

	items, err := srv.GetRecentlyAdded(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}

	if requestCount != 2 {
		t.Errorf("expected 2 requests (movies + shows), got %d", requestCount)
	}

	// 2 movies + 1 episode + 1 season directory = 4 total
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}

	item := items[0]
	if item.ItemID != "12345" {
		t.Errorf("item id = %q, want 12345", item.ItemID)
	}
	if item.Title != "Oppenheimer" {
		t.Errorf("title = %q, want Oppenheimer", item.Title)
	}
	if item.Year != 2023 {
		t.Errorf("year = %d, want 2023", item.Year)
	}
	if item.MediaType != models.MediaTypeMovie {
		t.Errorf("media type = %q, want movie", item.MediaType)
	}
	if item.ThumbURL != "12345" {
		t.Errorf("thumb url = %q, want 12345", item.ThumbURL)
	}
	if item.ServerID != 1 {
		t.Errorf("server id = %d, want 1", item.ServerID)
	}
	if item.ServerName != "TestPlex" {
		t.Errorf("server name = %q, want TestPlex", item.ServerName)
	}
	if item.ServerType != models.ServerTypePlex {
		t.Errorf("server type = %q, want plex", item.ServerType)
	}

	// Episode uses series poster (grandparentRatingKey) for thumb. Assert exactly
	// one match so we'd catch a regression that emits duplicates.
	episodeMatches := 0
	var episode models.LibraryItem
	for _, it := range items {
		if it.Title == "Breaking Bad - Ozymandias" {
			episodeMatches++
			episode = it
		}
	}
	if episodeMatches != 1 {
		t.Fatalf("expected exactly 1 'Breaking Bad - Ozymandias' entry, got %d", episodeMatches)
	}
	if episode.MediaType != models.MediaTypeTV {
		t.Errorf("episode media type = %q, want episode", episode.MediaType)
	}
	if episode.ThumbURL != "55555" {
		t.Errorf("episode thumb = %q, want 55555 (series grandparentRatingKey)", episode.ThumbURL)
	}
	if episode.SeasonNumber != 5 {
		t.Errorf("season number = %d, want 5", episode.SeasonNumber)
	}
	if episode.EpisodeNumber != 14 {
		t.Errorf("episode number = %d, want 14", episode.EpisodeNumber)
	}

	// Whole-season Directory entry: Plex returns these when an entire season is
	// added in a batch instead of per-episode <Video> entries. Assert exactly
	// one match so a future regression that emits the directory through the
	// movies hub as well would fail.
	seasonMatches := 0
	var season models.LibraryItem
	for _, it := range items {
		if it.SeriesTitle == "Secret Service" {
			seasonMatches++
			season = it
		}
	}
	if seasonMatches != 1 {
		t.Fatalf("expected exactly 1 'Secret Service' entry, got %d", seasonMatches)
	}
	if season.Title != "Secret Service" {
		t.Errorf("season title = %q, want Secret Service", season.Title)
	}
	if season.MediaType != models.MediaTypeTV {
		t.Errorf("season media type = %q, want episode", season.MediaType)
	}
	if season.SeasonNumber != 1 {
		t.Errorf("season number = %d, want 1", season.SeasonNumber)
	}
	if season.EpisodeNumber != 0 {
		t.Errorf("episode number = %d, want 0 (whole-season add)", season.EpisodeNumber)
	}
	if !season.SeasonBatch {
		t.Error("expected SeasonBatch=true for whole-season Directory entry")
	}
	if season.EpisodeCount != 5 {
		t.Errorf("episode count = %d, want 5 (leafCount)", season.EpisodeCount)
	}
	if season.ItemID != "33333" {
		t.Errorf("season item id = %q, want 33333 (parentRatingKey for show)", season.ItemID)
	}
	if season.ThumbURL != "33333" {
		t.Errorf("season thumb = %q, want 33333 (parentRatingKey for show poster)", season.ThumbURL)
	}
	if season.Year != 2026 {
		t.Errorf("season year = %d, want 2026", season.Year)
	}
}

func TestDirectoryToLibraryItem(t *testing.T) {
	t.Run("falls back to title when parentTitle is empty", func(t *testing.T) {
		got, ok := directoryToLibraryItem(recentlyAddedDirectory{
			Type:            "season",
			Title:           "Fallback Show",
			ParentRatingKey: "show-key",
			Index:           "1",
			AddedAt:         "1706000000",
		}, 1, "Plex")
		if !ok {
			t.Fatal("expected ok=true")
		}
		if got.Title != "Fallback Show" || got.SeriesTitle != "Fallback Show" {
			t.Errorf("title/series = %q/%q, want Fallback Show/Fallback Show", got.Title, got.SeriesTitle)
		}
	})

	t.Run("falls back to ratingKey when parentRatingKey is empty", func(t *testing.T) {
		got, ok := directoryToLibraryItem(recentlyAddedDirectory{
			Type:        "season",
			ParentTitle: "Show",
			RatingKey:   "season-key",
			Index:       "1",
			AddedAt:     "1706000000",
		}, 1, "Plex")
		if !ok {
			t.Fatal("expected ok=true")
		}
		if got.ItemID != "season-key" || got.ThumbURL != "season-key" {
			t.Errorf("itemID/thumb = %q/%q, want season-key/season-key", got.ItemID, got.ThumbURL)
		}
	})

	t.Run("ignores non-season directory types", func(t *testing.T) {
		_, ok := directoryToLibraryItem(recentlyAddedDirectory{
			Type:      "show",
			Title:     "Some Show",
			RatingKey: "show-key",
			AddedAt:   "1706000000",
		}, 1, "Plex")
		if ok {
			t.Error("expected ok=false for type=show (not yet supported)")
		}
	})
}

func TestGetRecentlyAddedEmpty(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?><MediaContainer size="0"></MediaContainer>`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(xml))
	}))
	defer ts.Close()

	srv := New(models.Server{ID: 1, URL: ts.URL, APIKey: "tok"})
	items, err := srv.GetRecentlyAdded(context.Background(), 10)
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

	srv := New(models.Server{URL: ts.URL, APIKey: "bad"})
	items, err := srv.GetRecentlyAdded(context.Background(), 10)
	if err != nil {
		t.Errorf("expected no error (errors logged, not returned), got: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items on error, got %d", len(items))
	}
}
