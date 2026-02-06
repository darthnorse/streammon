package store

import (
	"context"
	"testing"
	"time"

	"streammon/internal/models"
)

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
