package server

import (
	"testing"
	"time"

	"streammon/internal/models"
)

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
