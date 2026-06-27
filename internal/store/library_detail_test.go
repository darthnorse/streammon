package store

import (
	"context"
	"testing"
	"time"

	"streammon/internal/models"
)

// seedLibraryItem inserts one library_items row and returns its PK.
func seedLibraryItem(t *testing.T, s *Store, it models.LibraryItemCache) int64 {
	t.Helper()
	if _, err := s.UpsertLibraryItems(context.Background(), []models.LibraryItemCache{it}); err != nil {
		t.Fatalf("seed item: %v", err)
	}
	var id int64
	err := s.db.QueryRow(`SELECT id FROM library_items WHERE server_id=? AND item_id=?`,
		it.ServerID, it.ItemID).Scan(&id)
	if err != nil {
		t.Fatalf("lookup id: %v", err)
	}
	return id
}

func TestGetLibrarySummary(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Seed server to satisfy FK constraint (gets ID 1 in a fresh DB).
	if err := s.CreateServer(&models.Server{
		Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true,
	}); err != nil {
		t.Fatalf("seed server: %v", err)
	}

	// Two movies in library "1" on server 1: one watched, one never played.
	seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "m1",
		MediaType: models.MediaTypeMovie, Title: "Watched Movie", AddedAt: now, FileSize: 1000})
	seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "m2",
		MediaType: models.MediaTypeMovie, Title: "Never Movie", AddedAt: now, FileSize: 2000})

	if err := s.InsertHistory(&models.WatchHistoryEntry{ServerID: 1, ItemID: "m1",
		UserName: "alice", MediaType: models.MediaTypeMovie, Title: "Watched Movie",
		StartedAt: now, StoppedAt: now, WatchedMs: 3600000}); err != nil {
		t.Fatalf("insert history: %v", err)
	}

	got, err := s.GetLibrarySummary(ctx, 1, "1")
	if err != nil {
		t.Fatalf("GetLibrarySummary: %v", err)
	}
	if got.TotalTitles != 2 || got.WatchedTitles != 1 || got.NeverPlayed != 1 {
		t.Errorf("counts = %+v, want total=2 watched=1 never=1", got)
	}
	if got.TotalSize != 3000 || got.ReclaimableSize != 2000 {
		t.Errorf("sizes = total %d reclaimable %d, want 3000 / 2000", got.TotalSize, got.ReclaimableSize)
	}
}
