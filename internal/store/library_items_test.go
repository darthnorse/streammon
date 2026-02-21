package store

import (
	"context"
	"testing"
	"time"

	"streammon/internal/models"
)

func findByItemID(t *testing.T, items []models.LibraryItemCache, itemID string) models.LibraryItemCache {
	t.Helper()
	for _, it := range items {
		if it.ItemID == itemID {
			return it
		}
	}
	t.Fatalf("item %s not found", itemID)
	return models.LibraryItemCache{}
}

func TestUpsertLibraryItems(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	// Seed server
	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{
			ServerID:        srv.ID,
			LibraryID:       "lib1",
			ItemID:          "item1",
			MediaType:       models.MediaTypeMovie,
			Title:           "Test Movie",
			Year:            2024,
			AddedAt:         now.AddDate(0, 0, -30),
			VideoResolution: "1080p",
			FileSize:        5 * 1024 * 1024 * 1024,
			SyncedAt:        now,
		},
	}

	count, err := s.UpsertLibraryItems(ctx, items)
	if err != nil {
		t.Fatalf("UpsertLibraryItems: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// Verify inserted
	got, err := s.ListLibraryItems(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}
	if got[0].Title != "Test Movie" {
		t.Errorf("title = %q, want %q", got[0].Title, "Test Movie")
	}
	if got[0].FileSize != 5*1024*1024*1024 {
		t.Errorf("fileSize = %d, want %d", got[0].FileSize, 5*1024*1024*1024)
	}
}

func TestUpsertLibraryItemsUpdate(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()

	// Insert initial
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "item1",
		MediaType: models.MediaTypeMovie,
		Title:     "Original Title",
		Year:      2024,
		AddedAt:   now,
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Update with same ItemID
	items[0].Title = "Updated Title"
	items[0].SyncedAt = now.Add(time.Hour)
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Verify updated
	got, _ := s.ListLibraryItems(ctx, srv.ID, "lib1")
	if len(got) != 1 {
		t.Fatalf("should still have 1 item after upsert")
	}
	if got[0].Title != "Updated Title" {
		t.Errorf("title = %q, want %q", got[0].Title, "Updated Title")
	}
}

func TestUpsertLibraryItemsEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	count, err := s.UpsertLibraryItems(ctx, nil)
	if err != nil {
		t.Fatalf("UpsertLibraryItems with empty slice: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestDeleteStaleLibraryItems(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()

	// Insert item (synced_at is set to current time during upsert)
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "item1",
		MediaType: models.MediaTypeMovie,
		Title:     "Stale Item",
		AddedAt:   now.AddDate(0, 0, -30),
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Delete items synced before future time (should delete the item)
	deleted, err := s.DeleteStaleLibraryItems(ctx, srv.ID, "lib1", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("DeleteStaleLibraryItems: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	// Verify deleted
	got, _ := s.ListLibraryItems(ctx, srv.ID, "lib1")
	if len(got) != 0 {
		t.Errorf("got %d items, want 0", len(got))
	}
}

func TestDeleteStaleLibraryItemsKeepsFresh(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()

	// Insert first item (will be "stale" - synced at time of first upsert)
	items1 := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "stale", MediaType: models.MediaTypeMovie, Title: "Stale", AddedAt: now},
	}
	if _, err := s.UpsertLibraryItems(ctx, items1); err != nil {
		t.Fatal(err)
	}

	// Record the cutoff time after first insert
	cutoff := time.Now().UTC()
	time.Sleep(10 * time.Millisecond) // Ensure time difference

	// Insert second item (will be "fresh" - synced after cutoff)
	items2 := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "fresh", MediaType: models.MediaTypeMovie, Title: "Fresh", AddedAt: now},
	}
	if _, err := s.UpsertLibraryItems(ctx, items2); err != nil {
		t.Fatal(err)
	}

	// Delete items synced before cutoff (should only delete "stale")
	deleted, err := s.DeleteStaleLibraryItems(ctx, srv.ID, "lib1", cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	// Verify only fresh remains
	got, _ := s.ListLibraryItems(ctx, srv.ID, "lib1")
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}
	if got[0].ItemID != "fresh" {
		t.Errorf("remaining item = %q, want fresh", got[0].ItemID)
	}
}

func TestCountLibraryItems(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	// Empty initially
	count, err := s.CountLibraryItems(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	// Add items
	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "item1", MediaType: models.MediaTypeMovie, Title: "A", AddedAt: now, SyncedAt: now},
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "item2", MediaType: models.MediaTypeMovie, Title: "B", AddedAt: now, SyncedAt: now},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	count, err = s.CountLibraryItems(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestGetLibraryTotalSize(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "item1", MediaType: models.MediaTypeMovie, Title: "A", AddedAt: now, SyncedAt: now, FileSize: 1000},
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "item2", MediaType: models.MediaTypeMovie, Title: "B", AddedAt: now, SyncedAt: now, FileSize: 2000},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	size, err := s.GetLibraryTotalSize(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if size != 3000 {
		t.Errorf("size = %d, want 3000", size)
	}
}

func TestGetLibraryTotalSizeEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	size, err := s.GetLibraryTotalSize(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if size != 0 {
		t.Errorf("size = %d, want 0 for empty library", size)
	}
}

func TestGetLibraryItem(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:        srv.ID,
		LibraryID:       "lib1",
		ItemID:          "item1",
		MediaType:       models.MediaTypeMovie,
		Title:           "Test Movie",
		Year:            2024,
		AddedAt:         now,
		VideoResolution: "4k",
		FileSize:        10 * 1024 * 1024 * 1024,
		SyncedAt:        now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Get item ID from list
	list, _ := s.ListLibraryItems(ctx, srv.ID, "lib1")
	itemID := list[0].ID

	// Get single item
	got, err := s.GetLibraryItem(ctx, itemID)
	if err != nil {
		t.Fatalf("GetLibraryItem: %v", err)
	}
	if got.Title != "Test Movie" {
		t.Errorf("title = %q, want %q", got.Title, "Test Movie")
	}
	if got.VideoResolution != "4k" {
		t.Errorf("resolution = %q, want %q", got.VideoResolution, "4k")
	}
}

func TestGetLibraryItemNotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, err := s.GetLibraryItem(ctx, 99999)
	if err == nil {
		t.Error("expected error for not found")
	}
}

func TestGetLastSyncTime(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	// Empty library should return nil
	syncTime, err := s.GetLastSyncTime(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if syncTime != nil {
		t.Errorf("expected nil for empty library, got %v", syncTime)
	}

	// Add items
	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "item1", MediaType: models.MediaTypeMovie, Title: "A", AddedAt: now, SyncedAt: now},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Should return sync time
	syncTime, err = s.GetLastSyncTime(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if syncTime == nil {
		t.Fatal("expected non-nil sync time")
	}
}

func TestUpsertLibraryItemsLastWatchedAt(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	watchedAt := now.AddDate(0, 0, -30).Truncate(time.Second)

	// Insert item with LastWatchedAt set
	items := []models.LibraryItemCache{{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		ItemID:        "movie1",
		MediaType:     models.MediaTypeMovie,
		Title:         "Watched Movie",
		Year:          2024,
		AddedAt:       now.AddDate(0, 0, -90),
		LastWatchedAt: &watchedAt,
		SyncedAt:      now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Insert item without LastWatchedAt (nil)
	items2 := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "movie2",
		MediaType: models.MediaTypeMovie,
		Title:     "Unwatched Movie",
		Year:      2024,
		AddedAt:   now.AddDate(0, 0, -60),
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items2); err != nil {
		t.Fatal(err)
	}

	got, err := s.ListLibraryItems(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2", len(got))
	}

	// Find each item by title
	var watched, unwatched *models.LibraryItemCache
	for i := range got {
		switch got[i].Title {
		case "Watched Movie":
			watched = &got[i]
		case "Unwatched Movie":
			unwatched = &got[i]
		}
	}

	// Verify watched item round-trips LastWatchedAt
	if watched == nil {
		t.Fatal("watched movie not found")
	}
	if watched.LastWatchedAt == nil {
		t.Fatal("expected non-nil LastWatchedAt for watched movie")
	}
	if watched.LastWatchedAt.Sub(watchedAt).Abs() > time.Second {
		t.Errorf("LastWatchedAt = %v, want ~%v", *watched.LastWatchedAt, watchedAt)
	}

	// Verify unwatched item has nil LastWatchedAt
	if unwatched == nil {
		t.Fatal("unwatched movie not found")
	}
	if unwatched.LastWatchedAt != nil {
		t.Errorf("expected nil LastWatchedAt for unwatched movie, got %v", *unwatched.LastWatchedAt)
	}

	// Verify update: change LastWatchedAt on second sync
	newWatchedAt := now.AddDate(0, 0, -5).Truncate(time.Second)
	items[0].LastWatchedAt = &newWatchedAt
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	updated, err := s.GetLibraryItem(ctx, watched.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.LastWatchedAt == nil {
		t.Fatal("expected non-nil LastWatchedAt after update")
	}
	if updated.LastWatchedAt.Sub(newWatchedAt).Abs() > time.Second {
		t.Errorf("updated LastWatchedAt = %v, want ~%v", *updated.LastWatchedAt, newWatchedAt)
	}

	// Re-sync with nil LastWatchedAt must NOT wipe the existing value
	items[0].LastWatchedAt = nil
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}
	afterNilSync, err := s.GetLibraryItem(ctx, watched.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterNilSync.LastWatchedAt == nil {
		t.Fatal("LastWatchedAt was wiped to nil — upsert must never downgrade watch data")
	}
	if afterNilSync.LastWatchedAt.Sub(newWatchedAt).Abs() > time.Second {
		t.Errorf("LastWatchedAt = %v, want ~%v (should be preserved)", *afterNilSync.LastWatchedAt, newWatchedAt)
	}

	// Re-sync with older LastWatchedAt must NOT downgrade
	olderTime := now.AddDate(0, 0, -60).Truncate(time.Second)
	items[0].LastWatchedAt = &olderTime
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}
	afterOlderSync, err := s.GetLibraryItem(ctx, watched.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterOlderSync.LastWatchedAt == nil {
		t.Fatal("LastWatchedAt was wiped after older sync")
	}
	if afterOlderSync.LastWatchedAt.Sub(newWatchedAt).Abs() > time.Second {
		t.Errorf("LastWatchedAt = %v, want ~%v (should not be downgraded)", *afterOlderSync.LastWatchedAt, newWatchedAt)
	}
}

func seedMultiServerItems(t *testing.T, s *Store) (srvPlex, srvJelly *models.Server) {
	t.Helper()

	srvPlex = &models.Server{Name: "Plex", Type: models.ServerTypePlex, URL: "http://plex", APIKey: "key1", Enabled: true}
	if err := s.CreateServer(srvPlex); err != nil {
		t.Fatal(err)
	}
	srvJelly = &models.Server{Name: "Jellyfin", Type: models.ServerTypeJellyfin, URL: "http://jelly", APIKey: "key2", Enabled: true}
	if err := s.CreateServer(srvJelly); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	watchedRecently := now.AddDate(0, 0, -5).Truncate(time.Second)
	watchedLongAgo := now.AddDate(0, -6, 0).Truncate(time.Second)

	items := []models.LibraryItemCache{
		// Plex lib1: Inception (tmdb=27205), watched recently
		{ServerID: srvPlex.ID, LibraryID: "lib1", ItemID: "plex-1", MediaType: models.MediaTypeMovie, Title: "Inception", Year: 2010, AddedAt: now.AddDate(0, 0, -90), LastWatchedAt: &watchedRecently, TMDBID: "27205", SyncedAt: now},
		// Plex lib1: The Matrix (tmdb=603), never watched
		{ServerID: srvPlex.ID, LibraryID: "lib1", ItemID: "plex-2", MediaType: models.MediaTypeMovie, Title: "The Matrix", Year: 1999, AddedAt: now.AddDate(0, 0, -60), TMDBID: "603", SyncedAt: now},
		// Plex lib2: Breaking Bad (tvdb=81189), watched long ago
		{ServerID: srvPlex.ID, LibraryID: "lib2", ItemID: "plex-3", MediaType: models.MediaTypeTV, Title: "Breaking Bad", Year: 2008, AddedAt: now.AddDate(0, 0, -120), LastWatchedAt: &watchedLongAgo, TVDBID: "81189", SyncedAt: now},
		// Jellyfin lib1: Inception (same tmdb=27205), watched long ago
		{ServerID: srvJelly.ID, LibraryID: "lib1", ItemID: "jelly-1", MediaType: models.MediaTypeMovie, Title: "Inception", Year: 2010, AddedAt: now.AddDate(0, 0, -80), LastWatchedAt: &watchedLongAgo, TMDBID: "27205", SyncedAt: now},
		// Jellyfin lib1: Interstellar (tmdb=157336), no external match on other server
		{ServerID: srvJelly.ID, LibraryID: "lib1", ItemID: "jelly-2", MediaType: models.MediaTypeMovie, Title: "Interstellar", Year: 2014, AddedAt: now.AddDate(0, 0, -50), TMDBID: "157336", SyncedAt: now},
		// Jellyfin lib2: Breaking Bad (same tvdb=81189), never watched on this server
		{ServerID: srvJelly.ID, LibraryID: "lib2", ItemID: "jelly-3", MediaType: models.MediaTypeTV, Title: "Breaking Bad", Year: 2008, AddedAt: now.AddDate(0, 0, -100), TVDBID: "81189", SyncedAt: now},
		// Plex lib1: No external IDs at all
		{ServerID: srvPlex.ID, LibraryID: "lib1", ItemID: "plex-4", MediaType: models.MediaTypeMovie, Title: "Home Video", Year: 2023, AddedAt: now.AddDate(0, 0, -10), SyncedAt: now},
		// Jellyfin lib1: No external IDs at all (should NOT match plex-4)
		{ServerID: srvJelly.ID, LibraryID: "lib1", ItemID: "jelly-4", MediaType: models.MediaTypeMovie, Title: "Another Home Video", Year: 2023, AddedAt: now.AddDate(0, 0, -10), SyncedAt: now},
	}
	if _, err := s.UpsertLibraryItems(context.Background(), items); err != nil {
		t.Fatal(err)
	}

	return srvPlex, srvJelly
}

func TestListItemsForLibraries(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srvPlex, srvJelly := seedMultiServerItems(t, s)

	t.Run("multiple libraries", func(t *testing.T) {
		libs := []models.RuleLibrary{
			{ServerID: srvPlex.ID, LibraryID: "lib1"},
			{ServerID: srvJelly.ID, LibraryID: "lib1"},
		}
		got, err := s.ListItemsForLibraries(ctx, libs)
		if err != nil {
			t.Fatal(err)
		}
		// Plex lib1 has 3 items (plex-1, plex-2, plex-4), Jellyfin lib1 has 3 items (jelly-1, jelly-2, jelly-4)
		if len(got) != 6 {
			t.Errorf("got %d items, want 6", len(got))
		}
		// Verify ordered by added_at DESC
		for i := 1; i < len(got); i++ {
			if got[i].AddedAt.After(got[i-1].AddedAt) {
				t.Errorf("items not sorted by added_at DESC: %v after %v", got[i].AddedAt, got[i-1].AddedAt)
			}
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got, err := s.ListItemsForLibraries(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Error("expected non-nil empty slice, got nil")
		}
		if len(got) != 0 {
			t.Errorf("got %d items, want 0", len(got))
		}
	})

	t.Run("single library", func(t *testing.T) {
		libs := []models.RuleLibrary{
			{ServerID: srvPlex.ID, LibraryID: "lib2"},
		}
		got, err := s.ListItemsForLibraries(ctx, libs)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Errorf("got %d items, want 1", len(got))
		}
		if len(got) > 0 && got[0].Title != "Breaking Bad" {
			t.Errorf("title = %q, want %q", got[0].Title, "Breaking Bad")
		}
	})

	t.Run("nonexistent library", func(t *testing.T) {
		libs := []models.RuleLibrary{
			{ServerID: 9999, LibraryID: "nope"},
		}
		got, err := s.ListItemsForLibraries(ctx, libs)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("got %d items, want 0", len(got))
		}
	})

}

func TestGetCrossServerWatchTimes(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srvPlex, srvJelly := seedMultiServerItems(t, s)

	// Get all items so we have their IDs
	plexLib1, _ := s.ListLibraryItems(ctx, srvPlex.ID, "lib1")
	jellyLib1, _ := s.ListLibraryItems(ctx, srvJelly.ID, "lib1")
	jellyLib2, _ := s.ListLibraryItems(ctx, srvJelly.ID, "lib2")

	plexInception := findByItemID(t, plexLib1, "plex-1")
	jellyInception := findByItemID(t, jellyLib1, "jelly-1")
	jellyInterstellar := findByItemID(t, jellyLib1, "jelly-2")
	jellyBB := findByItemID(t, jellyLib2, "jelly-3")
	plexNoIDs := findByItemID(t, plexLib1, "plex-4")
	jellyNoIDs := findByItemID(t, jellyLib1, "jelly-4")

	t.Run("cross-server propagation via tmdb_id", func(t *testing.T) {
		// Jellyfin Inception should get Plex's more recent watch time
		result, err := s.GetCrossServerWatchTimes(ctx, []int64{jellyInception.ID})
		if err != nil {
			t.Fatal(err)
		}
		watchTime, ok := result[jellyInception.ID]
		if !ok {
			t.Fatal("expected result for jellyInception")
		}
		if watchTime == nil {
			t.Fatal("expected non-nil watch time")
		}
		// Plex Inception was watched more recently, so cross-server should return that time
		if plexInception.LastWatchedAt == nil {
			t.Fatal("plexInception should have LastWatchedAt")
		}
		if watchTime.Sub(*plexInception.LastWatchedAt).Abs() > time.Second {
			t.Errorf("cross-server watch time = %v, want ~%v (from Plex)", *watchTime, *plexInception.LastWatchedAt)
		}
	})

	t.Run("item watched on both servers returns later time", func(t *testing.T) {
		// Plex Inception (watched recently) + Jellyfin Inception (watched long ago)
		// For plexInception, cross-server check should still return its own time (the later one)
		result, err := s.GetCrossServerWatchTimes(ctx, []int64{plexInception.ID})
		if err != nil {
			t.Fatal(err)
		}
		watchTime := result[plexInception.ID]
		if watchTime == nil {
			t.Fatal("expected non-nil watch time for plexInception")
		}
		// The effective time should be the Plex one since it's more recent
		if watchTime.Sub(*plexInception.LastWatchedAt).Abs() > time.Second {
			t.Errorf("effective time = %v, want ~%v", *watchTime, *plexInception.LastWatchedAt)
		}
	})

	t.Run("no external IDs returns nil", func(t *testing.T) {
		result, err := s.GetCrossServerWatchTimes(ctx, []int64{plexNoIDs.ID})
		if err != nil {
			t.Fatal(err)
		}
		if result[plexNoIDs.ID] != nil {
			t.Errorf("expected nil for item with no external IDs, got %v", result[plexNoIDs.ID])
		}
	})

	t.Run("empty external IDs do not false match", func(t *testing.T) {
		// plex-4 and jelly-4 both have empty external IDs; they should NOT match
		result, err := s.GetCrossServerWatchTimes(ctx, []int64{plexNoIDs.ID, jellyNoIDs.ID})
		if err != nil {
			t.Fatal(err)
		}
		if result[plexNoIDs.ID] != nil {
			t.Errorf("expected nil for plexNoIDs, got %v", result[plexNoIDs.ID])
		}
		if result[jellyNoIDs.ID] != nil {
			t.Errorf("expected nil for jellyNoIDs, got %v", result[jellyNoIDs.ID])
		}
	})

	t.Run("cross-server propagation via tvdb_id", func(t *testing.T) {
		// Jellyfin Breaking Bad should get Plex's watch time via tvdb_id
		result, err := s.GetCrossServerWatchTimes(ctx, []int64{jellyBB.ID})
		if err != nil {
			t.Fatal(err)
		}
		watchTime := result[jellyBB.ID]
		if watchTime == nil {
			t.Fatal("expected non-nil watch time for jellyBB via tvdb_id cross-server")
		}
	})

	t.Run("no match returns nil", func(t *testing.T) {
		// Interstellar only exists on Jellyfin, no cross-server match, and never watched
		result, err := s.GetCrossServerWatchTimes(ctx, []int64{jellyInterstellar.ID})
		if err != nil {
			t.Fatal(err)
		}
		if result[jellyInterstellar.ID] != nil {
			t.Errorf("expected nil for jellyInterstellar, got %v", result[jellyInterstellar.ID])
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result, err := s.GetCrossServerWatchTimes(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})
}

func TestFindMatchingItems(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srvPlex, srvJelly := seedMultiServerItems(t, s)

	plexLib1, _ := s.ListLibraryItems(ctx, srvPlex.ID, "lib1")
	jellyLib1, _ := s.ListLibraryItems(ctx, srvJelly.ID, "lib1")

	t.Run("finds match on other server", func(t *testing.T) {
		plexInception := findByItemID(t, plexLib1, "plex-1")
		matches, err := s.FindMatchingItems(ctx, &plexInception)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Fatalf("got %d matches, want 1", len(matches))
		}
		if matches[0].ServerID != srvJelly.ID {
			t.Errorf("match server_id = %d, want %d", matches[0].ServerID, srvJelly.ID)
		}
		if matches[0].TMDBID != "27205" {
			t.Errorf("match tmdb_id = %q, want %q", matches[0].TMDBID, "27205")
		}
	})

	t.Run("excludes self", func(t *testing.T) {
		plexInception := findByItemID(t, plexLib1, "plex-1")
		matches, err := s.FindMatchingItems(ctx, &plexInception)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range matches {
			if m.ID == plexInception.ID {
				t.Error("result should not include the input item itself")
			}
		}
	})

	t.Run("no external IDs returns empty", func(t *testing.T) {
		plexNoIDs := findByItemID(t, plexLib1, "plex-4")
		matches, err := s.FindMatchingItems(ctx, &plexNoIDs)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Errorf("got %d matches, want 0 for item with no external IDs", len(matches))
		}
	})

	t.Run("unique item returns empty", func(t *testing.T) {
		jellyInterstellar := findByItemID(t, jellyLib1, "jelly-2")
		matches, err := s.FindMatchingItems(ctx, &jellyInterstellar)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Errorf("got %d matches, want 0 for unique item", len(matches))
		}
	})

	t.Run("contradicting external IDs rejected", func(t *testing.T) {
		// Insert two items that share IMDB but have different TMDB IDs (bad metadata).
		// The OR query would match them, but the contradiction filter should reject.
		now := time.Now().UTC()
		badItems := []models.LibraryItemCache{
			{ServerID: srvPlex.ID, LibraryID: "lib1", ItemID: "plex-bad-1", MediaType: models.MediaTypeMovie, Title: "Movie A", Year: 2020, TMDBID: "111", IMDBID: "tt9999", AddedAt: now, SyncedAt: now},
			{ServerID: srvJelly.ID, LibraryID: "lib1", ItemID: "jelly-bad-1", MediaType: models.MediaTypeMovie, Title: "Movie B", Year: 2020, TMDBID: "222", IMDBID: "tt9999", AddedAt: now, SyncedAt: now},
		}
		if _, err := s.UpsertLibraryItems(ctx, badItems); err != nil {
			t.Fatal(err)
		}

		plexItems, _ := s.ListLibraryItems(ctx, srvPlex.ID, "lib1")
		plexBad := findByItemID(t, plexItems, "plex-bad-1")
		matches, err := s.FindMatchingItems(ctx, &plexBad)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range matches {
			if m.ItemID == "jelly-bad-1" {
				t.Error("should NOT match jelly-bad-1: shared IMDB but contradicting TMDB")
			}
		}
	})

	t.Run("consistent partial IDs still match", func(t *testing.T) {
		// Source has tmdb+imdb, match has only imdb (no tmdb set) → should still match.
		now := time.Now().UTC()
		partialItems := []models.LibraryItemCache{
			{ServerID: srvPlex.ID, LibraryID: "lib1", ItemID: "plex-partial-1", MediaType: models.MediaTypeMovie, Title: "Partial A", Year: 2021, TMDBID: "333", IMDBID: "tt8888", AddedAt: now, SyncedAt: now},
			{ServerID: srvJelly.ID, LibraryID: "lib1", ItemID: "jelly-partial-1", MediaType: models.MediaTypeMovie, Title: "Partial A", Year: 2021, IMDBID: "tt8888", AddedAt: now, SyncedAt: now},
		}
		if _, err := s.UpsertLibraryItems(ctx, partialItems); err != nil {
			t.Fatal(err)
		}

		plexItems, _ := s.ListLibraryItems(ctx, srvPlex.ID, "lib1")
		plexPartial := findByItemID(t, plexItems, "plex-partial-1")
		matches, err := s.FindMatchingItems(ctx, &plexPartial)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, m := range matches {
			if m.ItemID == "jelly-partial-1" {
				found = true
			}
		}
		if !found {
			t.Error("should match jelly-partial-1: IMDB matches, no contradicting IDs")
		}
	})

}

func TestGetStreamMonWatchTimes(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srvPlex, _ := seedMultiServerItems(t, s)

	plexLib1, _ := s.ListLibraryItems(ctx, srvPlex.ID, "lib1")

	plexMatrix := findByItemID(t, plexLib1, "plex-2") // never watched per media server
	plexInception := findByItemID(t, plexLib1, "plex-1")

	now := time.Now().UTC()
	recentWatch := now.Add(-2 * time.Hour).Truncate(time.Second)

	// Insert watch_history for Matrix (movie) - direct item_id match
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID:  srvPlex.ID,
		ItemID:    "plex-2",
		UserName:  "familymember",
		MediaType: models.MediaTypeMovie,
		Title:     "The Matrix",
		StartedAt: recentWatch.Add(-2 * time.Hour),
		StoppedAt: recentWatch,
	})

	t.Run("movie matched by item_id", func(t *testing.T) {
		result, err := s.GetStreamMonWatchTimes(ctx, []int64{plexMatrix.ID})
		if err != nil {
			t.Fatal(err)
		}
		watchTime := result[plexMatrix.ID]
		if watchTime == nil {
			t.Fatal("expected non-nil watch time from StreamMon history")
		}
		if watchTime.Sub(recentWatch).Abs() > time.Second {
			t.Errorf("watch time = %v, want ~%v", *watchTime, recentWatch)
		}
	})

	t.Run("no history returns nil", func(t *testing.T) {
		result, err := s.GetStreamMonWatchTimes(ctx, []int64{plexInception.ID})
		if err != nil {
			t.Fatal(err)
		}
		// Inception has media server LastWatchedAt but no watch_history record
		if result[plexInception.ID] != nil {
			t.Errorf("expected nil for item with no StreamMon history, got %v", result[plexInception.ID])
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result, err := s.GetStreamMonWatchTimes(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %d entries", len(result))
		}
	})

	t.Run("TV show matched by grandparent_item_id", func(t *testing.T) {
		// Create a TV show library item
		tvItems := []models.LibraryItemCache{
			{ServerID: srvPlex.ID, LibraryID: "lib1", ItemID: "plex-show-1", MediaType: models.MediaTypeTV, Title: "Test Show", Year: 2024, AddedAt: now.AddDate(0, 0, -30), SyncedAt: now},
		}
		if _, err := s.UpsertLibraryItems(ctx, tvItems); err != nil {
			t.Fatal(err)
		}

		items, _ := s.ListLibraryItems(ctx, srvPlex.ID, "lib1")
		var showItem models.LibraryItemCache
		for _, it := range items {
			if it.ItemID == "plex-show-1" {
				showItem = it
				break
			}
		}
		if showItem.ID == 0 {
			t.Fatal("show item not found")
		}

		episodeWatch := now.Add(-1 * time.Hour).Truncate(time.Second)
		// Insert watch_history for an episode — grandparent_item_id is the show's item_id
		s.InsertHistory(&models.WatchHistoryEntry{
			ServerID:          srvPlex.ID,
			ItemID:            "plex-ep-1",
			GrandparentItemID: "plex-show-1",
			UserName:          "familymember",
			MediaType:         models.MediaTypeTV,
			Title:             "Episode 1",
			GrandparentTitle:  "Test Show",
			StartedAt:         episodeWatch.Add(-30 * time.Minute),
			StoppedAt:         episodeWatch,
		})

		result, err := s.GetStreamMonWatchTimes(ctx, []int64{showItem.ID})
		if err != nil {
			t.Fatal(err)
		}
		watchTime := result[showItem.ID]
		if watchTime == nil {
			t.Fatal("expected non-nil watch time from episode history via grandparent_item_id")
		}
		if watchTime.Sub(episodeWatch).Abs() > time.Second {
			t.Errorf("watch time = %v, want ~%v", *watchTime, episodeWatch)
		}
	})

	t.Run("TV show matched by title when grandparent_item_id empty", func(t *testing.T) {
		// Create a TV show that has a different item_id than what's in watch_history
		tvItems := []models.LibraryItemCache{
			{ServerID: srvPlex.ID, LibraryID: "lib1", ItemID: "plex-show-99", MediaType: models.MediaTypeTV, Title: "Succession", Year: 2018, AddedAt: now.AddDate(0, 0, -730), SyncedAt: now},
		}
		if _, err := s.UpsertLibraryItems(ctx, tvItems); err != nil {
			t.Fatal(err)
		}

		items, _ := s.ListLibraryItems(ctx, srvPlex.ID, "lib1")
		var showItem models.LibraryItemCache
		for _, it := range items {
			if it.ItemID == "plex-show-99" {
				showItem = it
				break
			}
		}
		if showItem.ID == 0 {
			t.Fatal("show item not found")
		}

		titleWatch := now.Add(-3 * time.Hour).Truncate(time.Second)
		// Insert watch_history with empty grandparent_item_id (simulates old/broken data)
		s.InsertHistory(&models.WatchHistoryEntry{
			ServerID:          srvPlex.ID,
			ItemID:            "plex-ep-500",
			GrandparentItemID: "", // empty — ID match won't work
			UserName:          "Rerun717",
			MediaType:         models.MediaTypeTV,
			Title:             "Austerlitz",
			GrandparentTitle:  "Succession",
			StartedAt:         titleWatch.Add(-45 * time.Minute),
			StoppedAt:         titleWatch,
		})

		result, err := s.GetStreamMonWatchTimes(ctx, []int64{showItem.ID})
		if err != nil {
			t.Fatal(err)
		}
		watchTime := result[showItem.ID]
		if watchTime == nil {
			t.Fatal("expected non-nil watch time from title-based fallback")
		}
		if watchTime.Sub(titleWatch).Abs() > time.Second {
			t.Errorf("watch time = %v, want ~%v", *watchTime, titleWatch)
		}
	})

	t.Run("movie matched by title when item_id empty", func(t *testing.T) {
		movieItems := []models.LibraryItemCache{
			{ServerID: srvPlex.ID, LibraryID: "lib1", ItemID: "plex-movie-99", MediaType: models.MediaTypeMovie, Title: "Orphan Movie", Year: 2020, AddedAt: now.AddDate(0, 0, -500), SyncedAt: now},
		}
		if _, err := s.UpsertLibraryItems(ctx, movieItems); err != nil {
			t.Fatal(err)
		}

		items, _ := s.ListLibraryItems(ctx, srvPlex.ID, "lib1")
		var movieItem models.LibraryItemCache
		for _, it := range items {
			if it.ItemID == "plex-movie-99" {
				movieItem = it
				break
			}
		}
		if movieItem.ID == 0 {
			t.Fatal("movie item not found")
		}

		movieWatch := now.Add(-5 * time.Hour).Truncate(time.Second)
		// Insert watch_history with empty item_id (old data)
		s.InsertHistory(&models.WatchHistoryEntry{
			ServerID:  srvPlex.ID,
			ItemID:    "", // empty — ID match won't work
			UserName:  "viewer1",
			MediaType: models.MediaTypeMovie,
			Title:     "Orphan Movie",
			StartedAt: movieWatch.Add(-2 * time.Hour),
			StoppedAt: movieWatch,
		})

		result, err := s.GetStreamMonWatchTimes(ctx, []int64{movieItem.ID})
		if err != nil {
			t.Fatal(err)
		}
		watchTime := result[movieItem.ID]
		if watchTime == nil {
			t.Fatal("expected non-nil watch time from title-based fallback for movie")
		}
		if watchTime.Sub(movieWatch).Abs() > time.Second {
			t.Errorf("watch time = %v, want ~%v", *watchTime, movieWatch)
		}
	})
}

func TestGetLibraryItemTMDBID(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "item1", MediaType: models.MediaTypeMovie, Title: "A", AddedAt: now, SyncedAt: now, TMDBID: "550"},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	t.Run("found", func(t *testing.T) {
		id, err := s.GetLibraryItemTMDBID(ctx, srv.ID, "item1")
		if err != nil {
			t.Fatal(err)
		}
		if id != "550" {
			t.Errorf("got %q, want 550", id)
		}
	})

	t.Run("not found", func(t *testing.T) {
		id, err := s.GetLibraryItemTMDBID(ctx, srv.ID, "missing")
		if err != nil {
			t.Fatal(err)
		}
		if id != "" {
			t.Errorf("got %q, want empty", id)
		}
	})
}

func TestGetAllLibraryTotalSizes(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "item1", MediaType: models.MediaTypeMovie, Title: "A", AddedAt: now, SyncedAt: now, FileSize: 1000},
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "item2", MediaType: models.MediaTypeMovie, Title: "B", AddedAt: now, SyncedAt: now, FileSize: 2000},
		{ServerID: srv.ID, LibraryID: "lib2", ItemID: "item3", MediaType: models.MediaTypeTV, Title: "C", AddedAt: now, SyncedAt: now, FileSize: 5000},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	sizes, err := s.GetAllLibraryTotalSizes(ctx)
	if err != nil {
		t.Fatal(err)
	}

	key1 := "1-lib1"
	key2 := "1-lib2"
	if sizes[key1] != 3000 {
		t.Errorf("lib1 size = %d, want 3000", sizes[key1])
	}
	if sizes[key2] != 5000 {
		t.Errorf("lib2 size = %d, want 5000", sizes[key2])
	}
}
