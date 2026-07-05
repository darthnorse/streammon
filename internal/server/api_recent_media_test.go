package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streammon/internal/models"
)

// TestRecentMediaNilPoller guards against a nil-pointer panic when the
// server has no poller configured (e.g. no media servers set up yet); it
// should degrade to an empty list like sibling dashboard/library handlers.
func TestRecentMediaNilPoller(t *testing.T) {
	srv, _ := newTestServerWrapped(t)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/recent-media", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var items []models.LibraryItem
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list, got %d items", len(items))
	}
}

func TestDedupeLibraryItemsSeparatesSeasonBatchFromEpisodeZero(t *testing.T) {
	// A whole-season batch entry (SeasonBatch=true) and a real S1E0 special
	// from another server must not dedupe against each other — they describe
	// different things at different granularities.
	now := time.Now().UTC()
	items := []models.LibraryItem{
		{
			ServerID:     1,
			ServerType:   models.ServerTypePlex,
			SeriesTitle:  "My Show",
			Title:        "My Show",
			MediaType:    models.MediaTypeTV,
			SeasonNumber: 1,
			SeasonBatch:  true,
			AddedAt:      now,
		},
		{
			ServerID:      2,
			ServerType:    models.ServerTypeEmby,
			SeriesTitle:   "My Show",
			Title:         "My Show - Special",
			MediaType:     models.MediaTypeTV,
			SeasonNumber:  1,
			EpisodeNumber: 0,
			AddedAt:       now,
		},
	}

	got := dedupeLibraryItems(items)
	if len(got) != 2 {
		t.Fatalf("expected 2 items (season-batch and S1E0 special are distinct), got %d", len(got))
	}
}

func TestDedupeLibraryItemsCollapsesDuplicateSeasonBatch(t *testing.T) {
	// Two season-batch entries for the same show/season (e.g. Plex returning
	// the same season directory twice across hub calls) should still collapse.
	now := time.Now().UTC()
	items := []models.LibraryItem{
		{
			ServerID:     1,
			ServerType:   models.ServerTypePlex,
			SeriesTitle:  "My Show",
			Title:        "My Show",
			MediaType:    models.MediaTypeTV,
			SeasonNumber: 1,
			SeasonBatch:  true,
			AddedAt:      now,
		},
		{
			ServerID:     1,
			ServerType:   models.ServerTypePlex,
			SeriesTitle:  "My Show",
			Title:        "My Show",
			MediaType:    models.MediaTypeTV,
			SeasonNumber: 1,
			SeasonBatch:  true,
			AddedAt:      now,
		},
	}

	got := dedupeLibraryItems(items)
	if len(got) != 1 {
		t.Fatalf("expected 1 item after dedupe, got %d", len(got))
	}
}

func TestDedupeLibraryItemsCollapsesIdenticalEpisodes(t *testing.T) {
	// Same episode from two servers should still dedupe.
	now := time.Now().UTC()
	items := []models.LibraryItem{
		{
			ServerID:      1,
			ServerType:    models.ServerTypePlex,
			SeriesTitle:   "My Show",
			Title:         "My Show - Pilot",
			MediaType:     models.MediaTypeTV,
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AddedAt:       now,
		},
		{
			ServerID:      2,
			ServerType:    models.ServerTypeEmby,
			SeriesTitle:   "My Show",
			Title:         "My Show - The Pilot",
			MediaType:     models.MediaTypeTV,
			SeasonNumber:  1,
			EpisodeNumber: 1,
			AddedAt:       now,
		},
	}

	got := dedupeLibraryItems(items)
	if len(got) != 1 {
		t.Fatalf("expected 1 item after dedupe (same series s1e1), got %d", len(got))
	}
}
