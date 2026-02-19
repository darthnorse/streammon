package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"streammon/internal/models"
)

func seedServer(t *testing.T, s *Store) int64 {
	t.Helper()
	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "k", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatalf("seed server: %v", err)
	}
	return srv.ID
}

func makeHistoryEntry(serverID int64, user, title string, startedAt time.Time) *models.WatchHistoryEntry {
	return &models.WatchHistoryEntry{
		ServerID: serverID, UserName: user, MediaType: models.MediaTypeMovie,
		Title: title, StartedAt: startedAt, StoppedAt: startedAt.Add(2 * time.Hour),
	}
}

func TestInsertAndListHistory(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	serverID := seedServer(t, s)
	entry := &models.WatchHistoryEntry{
		ServerID:  serverID,
		UserName:  "alice",
		MediaType: models.MediaTypeMovie,
		Title:     "The Matrix",
		Year:      1999,
		StartedAt: time.Now().UTC().Add(-2 * time.Hour),
		StoppedAt: time.Now().UTC().Add(-1 * time.Hour),
	}
	err := s.InsertHistory(entry)
	if err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}
	if entry.ID == 0 {
		t.Fatal("expected ID to be set")
	}

	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected total 1, got %d", result.Total)
	}
	if result.Items[0].Title != "The Matrix" {
		t.Fatalf("expected The Matrix, got %s", result.Items[0].Title)
	}
}

func TestListHistoryWithUserFilter(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	serverID := seedServer(t, s)
	now := time.Now().UTC()
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "A", StartedAt: now, StoppedAt: now,
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeTV,
		Title: "B", StartedAt: now, StoppedAt: now,
	})

	result, err := s.ListHistory(1, 10, "alice", "", "", nil)
	if err != nil {
		t.Fatalf("ListHistory(alice): %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 for alice, got %d", result.Total)
	}
}

func TestListHistoryWithServerIDs(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	sid1 := seedServer(t, s)
	srv2 := &models.Server{Name: "Second", Type: models.ServerTypePlex, URL: "http://test2", APIKey: "k2", Enabled: true}
	if err := s.CreateServer(srv2); err != nil {
		t.Fatalf("seed server 2: %v", err)
	}
	sid2 := srv2.ID

	now := time.Now().UTC()
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: sid1, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Movie A", StartedAt: now, StoppedAt: now,
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: sid2, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Movie B", StartedAt: now, StoppedAt: now,
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: sid1, UserName: "bob", MediaType: models.MediaTypeTV,
		Title: "Show C", StartedAt: now, StoppedAt: now,
	})

	result, err := s.ListHistory(1, 10, "", "", "", []int64{sid1})
	if err != nil {
		t.Fatalf("ListHistory(serverIDs=[sid1]): %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("expected 2 entries for server 1, got %d", result.Total)
	}
	for _, item := range result.Items {
		if item.ServerID != sid1 {
			t.Errorf("expected server_id %d, got %d", sid1, item.ServerID)
		}
	}

	result, err = s.ListHistory(1, 10, "", "", "", []int64{sid2})
	if err != nil {
		t.Fatalf("ListHistory(serverIDs=[sid2]): %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 entry for server 2, got %d", result.Total)
	}
	if result.Items[0].Title != "Movie B" {
		t.Errorf("expected Movie B, got %s", result.Items[0].Title)
	}

	result, err = s.ListHistory(1, 10, "", "", "", []int64{sid1, sid2})
	if err != nil {
		t.Fatalf("ListHistory(serverIDs=[sid1,sid2]): %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected 3 entries for both servers, got %d", result.Total)
	}

	result, err = s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatalf("ListHistory(serverIDs=nil): %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected 3 entries with nil filter, got %d", result.Total)
	}

	// Empty slice — same as nil, returns all
	result, err = s.ListHistory(1, 10, "", "", "", []int64{})
	if err != nil {
		t.Fatalf("ListHistory(serverIDs=[]): %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected 3 entries with empty slice filter, got %d", result.Total)
	}
}

func TestListHistoryPagination(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	serverID := seedServer(t, s)
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		s.InsertHistory(&models.WatchHistoryEntry{
			ServerID: serverID, UserName: "u", MediaType: models.MediaTypeMovie,
			Title: fmt.Sprintf("Movie %d", i), StartedAt: now, StoppedAt: now,
		})
	}

	result, err := s.ListHistory(2, 2, "", "", "", nil)
	if err != nil {
		t.Fatalf("ListHistory page 2: %v", err)
	}
	if result.Total != 5 {
		t.Fatalf("expected total 5, got %d", result.Total)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items on page 2, got %d", len(result.Items))
	}
	if result.Page != 2 {
		t.Fatalf("expected page 2, got %d", result.Page)
	}
}

func TestDailyWatchCounts(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	serverID := seedServer(t, s)
	day1 := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2024, 6, 2, 14, 0, 0, 0, time.UTC)

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "u", MediaType: models.MediaTypeMovie,
		Title: "M1", StartedAt: day1, StoppedAt: day1.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "u", MediaType: models.MediaTypeTV,
		Title: "T1", StartedAt: day1, StoppedAt: day1.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "u", MediaType: models.MediaTypeMovie,
		Title: "M2", StartedAt: day2, StoppedAt: day2.Add(time.Hour),
	})

	start := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC)

	stats, err := s.DailyWatchCountsForUser(start, end, "", nil)
	if err != nil {
		t.Fatalf("DailyWatchCountsForUser: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 days, got %d", len(stats))
	}
}

func TestDailyWatchCountsEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	start := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC)

	stats, err := s.DailyWatchCountsForUser(start, end, "", nil)
	if err != nil {
		t.Fatalf("DailyWatchCountsForUser: %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("expected 0, got %d", len(stats))
	}
}

func TestHistoryExists(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	startedAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID:  serverID,
		UserName:  "alice",
		MediaType: models.MediaTypeMovie,
		Title:     "The Matrix",
		StartedAt: startedAt,
		StoppedAt: startedAt.Add(2 * time.Hour),
	})

	exists, err := s.HistoryExists(serverID, "alice", "The Matrix", startedAt)
	if err != nil {
		t.Fatalf("HistoryExists: %v", err)
	}
	if !exists {
		t.Fatal("expected entry to exist")
	}

	exists, err = s.HistoryExists(serverID, "bob", "The Matrix", startedAt)
	if err != nil {
		t.Fatalf("HistoryExists (different user): %v", err)
	}
	if exists {
		t.Fatal("expected entry not to exist for different user")
	}

	exists, err = s.HistoryExists(serverID, "alice", "Different Movie", startedAt)
	if err != nil {
		t.Fatalf("HistoryExists (different title): %v", err)
	}
	if exists {
		t.Fatal("expected entry not to exist for different title")
	}

	nearTime := startedAt.Add(5 * time.Second)
	exists, err = s.HistoryExists(serverID, "alice", "The Matrix", nearTime)
	if err != nil {
		t.Fatalf("HistoryExists (near time): %v", err)
	}
	if !exists {
		t.Fatal("expected entry to exist for time within 60s window")
	}

	differentTime := startedAt.Add(time.Hour)
	exists, err = s.HistoryExists(serverID, "alice", "The Matrix", differentTime)
	if err != nil {
		t.Fatalf("HistoryExists (different time): %v", err)
	}
	if exists {
		t.Fatal("expected entry not to exist for different time")
	}
}

func TestInsertHistoryNormalizesThumbURL(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID:  serverID,
		UserName:  "alice",
		MediaType: models.MediaTypeMovie,
		Title:     "Test Movie",
		ThumbURL:  "/library/metadata/123/thumb/456",
		StartedAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
	})

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 1 {
		t.Fatalf("expected 1 entry, got %d", result.Total)
	}
	if result.Items[0].ThumbURL != "library/metadata/123/thumb/456" {
		t.Errorf("thumb_url = %q, want %q", result.Items[0].ThumbURL, "library/metadata/123/thumb/456")
	}
}

func TestInsertHistoryBatchNormalizesThumbURL(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	entries := []*models.WatchHistoryEntry{
		{
			ServerID:  serverID,
			UserName:  "alice",
			MediaType: models.MediaTypeMovie,
			Title:     "Movie 1",
			ThumbURL:  "/library/metadata/789/thumb/101",
			StartedAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
			StoppedAt: time.Date(2024, 6, 1, 14, 0, 0, 0, time.UTC),
		},
	}

	inserted, _, _, err := s.InsertHistoryBatch(context.Background(), entries)
	if err != nil {
		t.Fatalf("InsertHistoryBatch: %v", err)
	}
	if inserted != 1 {
		t.Fatalf("expected 1 inserted, got %d", inserted)
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Items[0].ThumbURL != "library/metadata/789/thumb/101" {
		t.Errorf("thumb_url = %q, want %q", result.Items[0].ThumbURL, "library/metadata/789/thumb/101")
	}
}

func TestInsertHistoryBatch(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	startedAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	entries := []*models.WatchHistoryEntry{
		{
			ServerID:  serverID,
			UserName:  "alice",
			MediaType: models.MediaTypeMovie,
			Title:     "Movie 1",
			StartedAt: startedAt,
			StoppedAt: startedAt.Add(2 * time.Hour),
		},
		{
			ServerID:  serverID,
			UserName:  "bob",
			MediaType: models.MediaTypeTV,
			Title:     "Episode 1",
			StartedAt: startedAt.Add(time.Hour),
			StoppedAt: startedAt.Add(2 * time.Hour),
		},
	}

	inserted, skipped, consolidated, err := s.InsertHistoryBatch(context.Background(), entries)
	if err != nil {
		t.Fatalf("InsertHistoryBatch: %v", err)
	}
	if inserted != 2 {
		t.Errorf("expected 2 inserted, got %d", inserted)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", skipped)
	}
	if consolidated != 0 {
		t.Errorf("expected 0 consolidated, got %d", consolidated)
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 2 {
		t.Fatalf("expected 2 entries in history, got %d", result.Total)
	}
}

func TestInsertHistoryBatchSkipsDuplicates(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	startedAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := s.InsertHistory(makeHistoryEntry(serverID, "alice", "Existing Movie", startedAt)); err != nil {
		t.Fatalf("pre-insert: %v", err)
	}

	entries := []*models.WatchHistoryEntry{
		makeHistoryEntry(serverID, "alice", "Existing Movie", startedAt),
		makeHistoryEntry(serverID, "bob", "New Episode", startedAt.Add(time.Hour)),
	}

	inserted, skipped, consolidated, err := s.InsertHistoryBatch(context.Background(), entries)
	if err != nil {
		t.Fatalf("InsertHistoryBatch: %v", err)
	}
	if inserted != 1 {
		t.Errorf("expected 1 inserted, got %d", inserted)
	}
	if skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", skipped)
	}
	if consolidated != 0 {
		t.Errorf("expected 0 consolidated, got %d", consolidated)
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 2 {
		t.Fatalf("expected 2 entries in history (1 original + 1 new), got %d", result.Total)
	}
}

func TestInsertHistoryDedupWithinWindow(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	startedAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	if err := s.InsertHistory(makeHistoryEntry(serverID, "alice", "Test Movie", startedAt)); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Second insert with started_at 5 seconds later (poller vs Tautulli) — should be skipped
	dup := makeHistoryEntry(serverID, "alice", "Test Movie", startedAt.Add(5*time.Second))
	if err := s.InsertHistory(dup); err != nil {
		t.Fatalf("dedup insert: %v", err)
	}
	if dup.ID != 0 {
		t.Error("expected duplicate to be silently skipped (ID should remain 0)")
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 1 {
		t.Errorf("expected 1 entry after dedup, got %d", result.Total)
	}
}

func TestInsertHistoryAllowsDifferentSessions(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	startedAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	if err := s.InsertHistory(makeHistoryEntry(serverID, "alice", "Test Movie", startedAt)); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Same user/title but 3 hours later — genuine rewatch, not a duplicate
	// (gap between first entry's StoppedAt and this StartedAt is 1 hour, outside 30min consolidation window)
	if err := s.InsertHistory(makeHistoryEntry(serverID, "alice", "Test Movie", startedAt.Add(3*time.Hour))); err != nil {
		t.Fatalf("rewatch insert: %v", err)
	}

	if err := s.InsertHistory(makeHistoryEntry(serverID, "bob", "Test Movie", startedAt)); err != nil {
		t.Fatalf("different user insert: %v", err)
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 3 {
		t.Errorf("expected 3 distinct entries, got %d", result.Total)
	}
}

func TestInsertHistoryDedupAndConsolidateBoundary(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	startedAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	if err := s.InsertHistory(makeHistoryEntry(serverID, "alice", "Test Movie", startedAt)); err != nil {
		t.Fatalf("initial insert: %v", err)
	}

	dup59 := makeHistoryEntry(serverID, "alice", "Test Movie", startedAt.Add(59*time.Second))
	if err := s.InsertHistory(dup59); err != nil {
		t.Fatalf("59s insert: %v", err)
	}
	if dup59.ID != 0 {
		t.Error("59s gap: expected duplicate to be skipped")
	}

	// Exactly 60 seconds — at boundary, BETWEEN is inclusive so should be deduped
	dup60 := makeHistoryEntry(serverID, "alice", "Test Movie", startedAt.Add(60*time.Second))
	if err := s.InsertHistory(dup60); err != nil {
		t.Fatalf("60s insert: %v", err)
	}
	if dup60.ID != 0 {
		t.Error("60s gap: expected duplicate to be skipped (boundary is inclusive)")
	}

	entry61 := makeHistoryEntry(serverID, "alice", "Test Movie", startedAt.Add(61*time.Second))
	if err := s.InsertHistory(entry61); err != nil {
		t.Fatalf("61s insert: %v", err)
	}
	// 61s is outside dedup but within consolidation window (overlapping sessions), so consolidated
	if entry61.ID != 0 {
		t.Error("61s gap: expected entry to be consolidated (overlapping session)")
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 1 {
		t.Errorf("expected 1 entry (61s consolidated into original), got %d", result.Total)
	}
}

func TestInsertHistoryBatchFuzzyDedup(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	startedAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	// Pre-insert via poller path (started_at is poll detection time)
	if err := s.InsertHistory(makeHistoryEntry(serverID, "alice", "Test Movie", startedAt.Add(3*time.Second))); err != nil {
		t.Fatalf("pre-insert: %v", err)
	}

	// Tautulli batch import with true start time (3 seconds earlier)
	entries := []*models.WatchHistoryEntry{
		makeHistoryEntry(serverID, "alice", "Test Movie", startedAt),
		makeHistoryEntry(serverID, "bob", "New Episode", startedAt),
	}

	inserted, skipped, consolidated, err := s.InsertHistoryBatch(context.Background(), entries)
	if err != nil {
		t.Fatalf("InsertHistoryBatch: %v", err)
	}
	if inserted != 1 {
		t.Errorf("expected 1 inserted (bob's), got %d", inserted)
	}
	if skipped != 1 {
		t.Errorf("expected 1 skipped (alice's duplicate), got %d", skipped)
	}
	if consolidated != 0 {
		t.Errorf("expected 0 consolidated, got %d", consolidated)
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 2 {
		t.Errorf("expected 2 entries total, got %d", result.Total)
	}
}

func TestInsertHistoryBatchEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	inserted, skipped, consolidated, err := s.InsertHistoryBatch(context.Background(), []*models.WatchHistoryEntry{})
	if err != nil {
		t.Fatalf("InsertHistoryBatch (empty): %v", err)
	}
	if inserted != 0 || skipped != 0 || consolidated != 0 {
		t.Errorf("expected 0/0/0, got %d/%d/%d", inserted, skipped, consolidated)
	}
}

func TestInsertHistoryBatchContextCancellation(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	startedAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	entries := make([]*models.WatchHistoryEntry, 100)
	for i := range entries {
		entries[i] = &models.WatchHistoryEntry{
			ServerID:  serverID,
			UserName:  "alice",
			MediaType: models.MediaTypeMovie,
			Title:     "Movie",
			StartedAt: startedAt.Add(time.Duration(i) * time.Hour),
			StoppedAt: startedAt.Add(time.Duration(i+1) * time.Hour),
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, _, err := s.InsertHistoryBatch(ctx, entries)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestGetLastStreamBeforeTime(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	entries := []models.WatchHistoryEntry{
		{ServerID: serverID, UserName: "alice", Title: "Movie 1", StartedAt: now.Add(-2 * time.Hour), IPAddress: "1.1.1.1", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
		{ServerID: serverID, UserName: "alice", Title: "Movie 2", StartedAt: now.Add(-1 * time.Hour), IPAddress: "2.2.2.2", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
		{ServerID: serverID, UserName: "bob", Title: "Movie 3", StartedAt: now.Add(-30 * time.Minute), IPAddress: "3.3.3.3", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
	}
	for i := range entries {
		if err := s.InsertHistory(&entries[i]); err != nil {
			t.Fatalf("InsertHistory: %v", err)
		}
	}

	entry, err := s.GetLastStreamBeforeTime("alice", now, 24)
	if err != nil {
		t.Fatalf("GetLastStreamBeforeTime: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.Title != "Movie 2" {
		t.Errorf("expected Movie 2, got %s", entry.Title)
	}
	if entry.IPAddress != "2.2.2.2" {
		t.Errorf("expected IP 2.2.2.2, got %s", entry.IPAddress)
	}

	entry, err = s.GetLastStreamBeforeTime("alice", now.Add(-1*time.Hour), 24)
	if err != nil {
		t.Fatalf("GetLastStreamBeforeTime (before second entry): %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.Title != "Movie 1" {
		t.Errorf("expected Movie 1, got %s", entry.Title)
	}

	entry, err = s.GetLastStreamBeforeTime("alice", now.Add(-3*time.Hour), 1)
	if err != nil {
		t.Fatalf("GetLastStreamBeforeTime (outside window): %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil entry for outside window, got %+v", entry)
	}

	entry, err = s.GetLastStreamBeforeTime("unknown", now, 24)
	if err != nil {
		t.Fatalf("GetLastStreamBeforeTime (unknown user): %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil entry for unknown user, got %+v", entry)
	}
}

func TestGetDeviceLastStream(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	entries := []models.WatchHistoryEntry{
		{ServerID: serverID, UserName: "alice", Title: "Movie 1", StartedAt: now.Add(-2 * time.Hour), IPAddress: "1.1.1.1", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
		{ServerID: serverID, UserName: "alice", Title: "Movie 2", StartedAt: now.Add(-1 * time.Hour), IPAddress: "2.2.2.2", Player: "Plex for iOS", Platform: "iOS", MediaType: models.MediaTypeMovie},
		{ServerID: serverID, UserName: "alice", Title: "Movie 3", StartedAt: now.Add(-30 * time.Minute), IPAddress: "3.3.3.3", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
	}
	for i := range entries {
		if err := s.InsertHistory(&entries[i]); err != nil {
			t.Fatalf("InsertHistory: %v", err)
		}
	}

	entry, err := s.GetDeviceLastStream("alice", "Plex Web", "Chrome", now, 24)
	if err != nil {
		t.Fatalf("GetDeviceLastStream: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.Title != "Movie 3" {
		t.Errorf("expected Movie 3, got %s", entry.Title)
	}

	entry, err = s.GetDeviceLastStream("alice", "Plex Web", "Chrome", now.Add(-30*time.Minute), 24)
	if err != nil {
		t.Fatalf("GetDeviceLastStream (before Movie 3): %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.Title != "Movie 1" {
		t.Errorf("expected Movie 1, got %s", entry.Title)
	}

	entry, err = s.GetDeviceLastStream("alice", "Plex for iOS", "iOS", now, 24)
	if err != nil {
		t.Fatalf("GetDeviceLastStream (iOS): %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.Title != "Movie 2" {
		t.Errorf("expected Movie 2, got %s", entry.Title)
	}

	entry, err = s.GetDeviceLastStream("alice", "Unknown", "Unknown", now, 24)
	if err != nil {
		t.Fatalf("GetDeviceLastStream (unknown device): %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil entry for unknown device, got %+v", entry)
	}
}

func TestHasDeviceBeenUsed(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	entry := models.WatchHistoryEntry{
		ServerID:  serverID,
		UserName:  "alice",
		Title:     "Movie 1",
		MediaType: models.MediaTypeMovie,
		StartedAt: now.Add(-1 * time.Hour),
		IPAddress: "1.1.1.1",
		Player:    "Plex Web",
		Platform:  "Chrome",
	}
	if err := s.InsertHistory(&entry); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}

	used, err := s.HasDeviceBeenUsed("alice", "Plex Web", "Chrome", now)
	if err != nil {
		t.Fatalf("HasDeviceBeenUsed: %v", err)
	}
	if !used {
		t.Error("expected device to have been used")
	}

	used, err = s.HasDeviceBeenUsed("alice", "Plex Web", "Chrome", now.Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("HasDeviceBeenUsed (before entry): %v", err)
	}
	if used {
		t.Error("expected device not to have been used before entry time")
	}

	used, err = s.HasDeviceBeenUsed("alice", "Plex for iOS", "iOS", now)
	if err != nil {
		t.Fatalf("HasDeviceBeenUsed (different device): %v", err)
	}
	if used {
		t.Error("expected different device not to have been used")
	}

	used, err = s.HasDeviceBeenUsed("bob", "Plex Web", "Chrome", now)
	if err != nil {
		t.Fatalf("HasDeviceBeenUsed (different user): %v", err)
	}
	if used {
		t.Error("expected different user not to have used this device")
	}
}

func TestGetUserDistinctIPs(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	entries := []models.WatchHistoryEntry{
		{ServerID: serverID, UserName: "alice", Title: "Movie 1", StartedAt: now.Add(-3 * time.Hour), IPAddress: "1.1.1.1", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
		{ServerID: serverID, UserName: "alice", Title: "Movie 2", StartedAt: now.Add(-2 * time.Hour), IPAddress: "2.2.2.2", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
		{ServerID: serverID, UserName: "alice", Title: "Movie 3", StartedAt: now.Add(-1 * time.Hour), IPAddress: "1.1.1.1", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
		{ServerID: serverID, UserName: "bob", Title: "Movie 4", StartedAt: now.Add(-30 * time.Minute), IPAddress: "3.3.3.3", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
	}
	for i := range entries {
		if err := s.InsertHistory(&entries[i]); err != nil {
			t.Fatalf("InsertHistory: %v", err)
		}
	}

	ips, err := s.GetUserDistinctIPs("alice", now, 100)
	if err != nil {
		t.Fatalf("GetUserDistinctIPs: %v", err)
	}
	if len(ips) != 2 {
		t.Errorf("expected 2 distinct IPs, got %d", len(ips))
	}
	ipSet := make(map[string]bool)
	for _, ip := range ips {
		ipSet[ip] = true
	}
	if !ipSet["1.1.1.1"] {
		t.Error("expected IP 1.1.1.1 to be in result")
	}
	if !ipSet["2.2.2.2"] {
		t.Error("expected IP 2.2.2.2 to be in result")
	}

	ips, err = s.GetUserDistinctIPs("alice", now, 1)
	if err != nil {
		t.Fatalf("GetUserDistinctIPs (limit 1): %v", err)
	}
	if len(ips) != 1 {
		t.Errorf("expected 1 IP with limit, got %d", len(ips))
	}

	ips, err = s.GetUserDistinctIPs("unknown", now, 100)
	if err != nil {
		t.Fatalf("GetUserDistinctIPs (unknown user): %v", err)
	}
	if len(ips) != 0 {
		t.Errorf("expected 0 IPs for unknown user, got %d", len(ips))
	}
}

func TestInsertHistoryWithPausedMsAndWatched(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	entry := &models.WatchHistoryEntry{
		ServerID:  serverID,
		UserName:  "alice",
		MediaType: models.MediaTypeMovie,
		Title:     "Test Movie",
		StartedAt: time.Now().UTC().Add(-2 * time.Hour),
		StoppedAt: time.Now().UTC().Add(-1 * time.Hour),
		PausedMs:  15000,
		Watched:   true,
	}
	if err := s.InsertHistory(entry); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}

	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 entry, got %d", result.Total)
	}
	if result.Items[0].PausedMs != 15000 {
		t.Errorf("paused_ms = %d, want 15000", result.Items[0].PausedMs)
	}
	if !result.Items[0].Watched {
		t.Error("expected watched = true")
	}
}

func TestInsertHistoryWatchedFalse(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	entry := &models.WatchHistoryEntry{
		ServerID:  serverID,
		UserName:  "bob",
		MediaType: models.MediaTypeMovie,
		Title:     "Abandoned Movie",
		StartedAt: time.Now().UTC(),
		StoppedAt: time.Now().UTC(),
		PausedMs:  0,
		Watched:   false,
	}
	if err := s.InsertHistory(entry); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Items[0].Watched {
		t.Error("expected watched = false")
	}
	if result.Items[0].PausedMs != 0 {
		t.Errorf("paused_ms = %d, want 0", result.Items[0].PausedMs)
	}
}

func TestCountUnenrichedHistory(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()

	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Unenriched 1", StartedAt: now, StoppedAt: now,
		TautulliReferenceID: 42,
	}); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}
	enrichedEntry := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Enriched 1", StartedAt: now.Add(time.Hour), StoppedAt: now.Add(2 * time.Hour),
		TautulliReferenceID: 43,
	}
	if err := s.InsertHistory(enrichedEntry); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}
	if err := s.MarkHistoryEnriched(context.Background(), enrichedEntry.ID); err != nil {
		t.Fatalf("MarkHistoryEnriched: %v", err)
	}
	// Insert record without reference_id (not from Tautulli)
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Native 1", StartedAt: now.Add(2 * time.Hour), StoppedAt: now.Add(3 * time.Hour),
	}); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}

	count, err := s.CountUnenrichedHistory(context.Background(), serverID)
	if err != nil {
		t.Fatalf("CountUnenrichedHistory: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 unenriched, got %d", count)
	}
}

func TestListUnenrichedHistory(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Unenriched 1", StartedAt: now, StoppedAt: now,
		TautulliReferenceID: 100,
	}); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Unenriched 2", StartedAt: now.Add(time.Hour), StoppedAt: now.Add(2 * time.Hour),
		TautulliReferenceID: 200,
	}); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}

	refs, err := s.ListUnenrichedHistory(context.Background(), serverID, 10)
	if err != nil {
		t.Fatalf("ListUnenrichedHistory: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].RefID != 100 {
		t.Errorf("expected ref_id 100, got %d", refs[0].RefID)
	}
	if refs[1].RefID != 200 {
		t.Errorf("expected ref_id 200, got %d", refs[1].RefID)
	}

	refs, err = s.ListUnenrichedHistory(context.Background(), serverID, 1)
	if err != nil {
		t.Fatalf("ListUnenrichedHistory (limit 1): %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref with limit, got %d", len(refs))
	}
}

func TestUpdateHistoryEnrichment(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	entry := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Test Movie", StartedAt: now, StoppedAt: now.Add(time.Hour),
		TautulliReferenceID: 55,
	}
	if err := s.InsertHistory(entry); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}

	enriched := &models.WatchHistoryEntry{
		VideoResolution:   "1080p",
		VideoCodec:        "h264",
		AudioCodec:        "aac",
		AudioChannels:     6,
		Bandwidth:         5000,
		TranscodeDecision: models.TranscodeDecisionDirectPlay,
		VideoDecision:     models.TranscodeDecisionDirectPlay,
		AudioDecision:     models.TranscodeDecisionDirectPlay,
		TranscodeHWDecode: true,
		DynamicRange:      "SDR",
	}
	if err := s.UpdateHistoryEnrichment(context.Background(), entry.ID, enriched); err != nil {
		t.Fatalf("UpdateHistoryEnrichment: %v", err)
	}

	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if result.Items[0].VideoCodec != "h264" {
		t.Errorf("expected h264, got %s", result.Items[0].VideoCodec)
	}
	if result.Items[0].VideoResolution != "1080p" {
		t.Errorf("expected 1080p, got %s", result.Items[0].VideoResolution)
	}
	if result.Items[0].AudioChannels != 6 {
		t.Errorf("expected 6 channels, got %d", result.Items[0].AudioChannels)
	}
	if !result.Items[0].TranscodeHWDecode {
		t.Errorf("expected TranscodeHWDecode true")
	}
	if result.Items[0].TranscodeHWEncode {
		t.Errorf("expected TranscodeHWEncode false")
	}

	count, err := s.CountUnenrichedHistory(context.Background(), serverID)
	if err != nil {
		t.Fatalf("CountUnenrichedHistory: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 unenriched after enrichment, got %d", count)
	}
}

func TestUpdateHistoryEnrichment_PreservesExistingFields(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	entry := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Test Movie", StartedAt: now, StoppedAt: now.Add(time.Hour),
		TautulliReferenceID: 56,
		VideoResolution:     "4K",
		TranscodeDecision:   models.TranscodeDecisionTranscode,
	}
	if err := s.InsertHistory(entry); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}

	// Enrich with empty VideoResolution and TranscodeDecision — should preserve originals
	enriched := &models.WatchHistoryEntry{
		VideoCodec:    "hevc",
		AudioCodec:    "eac3",
		AudioChannels: 8,
	}
	if err := s.UpdateHistoryEnrichment(context.Background(), entry.ID, enriched); err != nil {
		t.Fatalf("UpdateHistoryEnrichment: %v", err)
	}

	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if result.Items[0].VideoResolution != "4K" {
		t.Errorf("expected preserved VideoResolution '4K', got %q", result.Items[0].VideoResolution)
	}
	if result.Items[0].TranscodeDecision != models.TranscodeDecisionTranscode {
		t.Errorf("expected preserved TranscodeDecision 'transcode', got %q", result.Items[0].TranscodeDecision)
	}
	if result.Items[0].VideoCodec != "hevc" {
		t.Errorf("expected hevc, got %s", result.Items[0].VideoCodec)
	}
}

func TestMarkHistoryEnriched(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	entry := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Test Movie", StartedAt: now, StoppedAt: now.Add(time.Hour),
		TautulliReferenceID: 77,
	}
	if err := s.InsertHistory(entry); err != nil {
		t.Fatalf("InsertHistory: %v", err)
	}

	count, _ := s.CountUnenrichedHistory(context.Background(), serverID)
	if count != 1 {
		t.Fatalf("expected 1 unenriched, got %d", count)
	}

	if err := s.MarkHistoryEnriched(context.Background(), entry.ID); err != nil {
		t.Fatalf("MarkHistoryEnriched: %v", err)
	}

	count, _ = s.CountUnenrichedHistory(context.Background(), serverID)
	if count != 0 {
		t.Errorf("expected 0 unenriched after marking, got %d", count)
	}
}

func TestGetRecentISPs(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()

	// Seed geo cache with ISP data
	geoEntries := []*models.GeoResult{
		{IP: "1.1.1.1", Lat: 40.7, Lng: -74.0, City: "New York", Country: "US", ISP: "Comcast"},
		{IP: "2.2.2.2", Lat: 34.0, Lng: -118.2, City: "Los Angeles", Country: "US", ISP: "Verizon"},
		{IP: "3.3.3.3", Lat: 41.8, Lng: -87.6, City: "Chicago", Country: "US", ISP: "AT&T"},
		{IP: "4.4.4.4", Lat: 47.6, Lng: -122.3, City: "Seattle", Country: "US", ISP: ""}, // Empty ISP
	}
	for _, geo := range geoEntries {
		if err := s.SetCachedGeo(geo); err != nil {
			t.Fatalf("SetCachedGeo: %v", err)
		}
	}

	// Seed watch history
	entries := []models.WatchHistoryEntry{
		{ServerID: serverID, UserName: "alice", Title: "Movie 1", StartedAt: now.Add(-3 * time.Hour), IPAddress: "1.1.1.1", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
		{ServerID: serverID, UserName: "alice", Title: "Movie 2", StartedAt: now.Add(-2 * time.Hour), IPAddress: "2.2.2.2", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
		{ServerID: serverID, UserName: "alice", Title: "Movie 3", StartedAt: now.Add(-1 * time.Hour), IPAddress: "1.1.1.1", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
		{ServerID: serverID, UserName: "alice", Title: "Movie 4", StartedAt: now.Add(-30 * time.Minute), IPAddress: "4.4.4.4", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie}, // Empty ISP - should not be counted
		{ServerID: serverID, UserName: "bob", Title: "Movie 5", StartedAt: now.Add(-30 * time.Minute), IPAddress: "3.3.3.3", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
	}
	for i := range entries {
		if err := s.InsertHistory(&entries[i]); err != nil {
			t.Fatalf("InsertHistory: %v", err)
		}
	}

	isps, err := s.GetRecentISPs("alice", now, 24)
	if err != nil {
		t.Fatalf("GetRecentISPs: %v", err)
	}
	if len(isps) != 2 {
		t.Errorf("expected 2 distinct ISPs, got %d: %v", len(isps), isps)
	}
	ispSet := make(map[string]bool)
	for _, isp := range isps {
		ispSet[isp] = true
	}
	if !ispSet["Comcast"] {
		t.Error("expected ISP Comcast to be in result")
	}
	if !ispSet["Verizon"] {
		t.Error("expected ISP Verizon to be in result")
	}

	isps, err = s.GetRecentISPs("alice", now.Add(-2*time.Hour), 1) // Only entries from 3 hours ago
	if err != nil {
		t.Fatalf("GetRecentISPs (time window): %v", err)
	}
	if len(isps) != 1 {
		t.Errorf("expected 1 ISP in time window, got %d: %v", len(isps), isps)
	}

	isps, err = s.GetRecentISPs("unknown", now, 24)
	if err != nil {
		t.Fatalf("GetRecentISPs (unknown user): %v", err)
	}
	if len(isps) != 0 {
		t.Errorf("expected 0 ISPs for unknown user, got %d", len(isps))
	}

	for _, isp := range isps {
		if isp == "" {
			t.Error("empty ISP should not be in result")
		}
	}
}

func TestInsertHistoryConsolidates(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	entryA := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", DurationMs: 100000, WatchedMs: 30000, PausedMs: 5000,
		StartedAt: now.Add(-20 * time.Minute), StoppedAt: now.Add(-10 * time.Minute),
	}
	if err := s.InsertHistory(entryA); err != nil {
		t.Fatalf("insert A: %v", err)
	}

	entryB := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", DurationMs: 100000, WatchedMs: 20000, PausedMs: 3000,
		StartedAt: now.Add(-5 * time.Minute), StoppedAt: now,
	}
	if err := s.InsertHistory(entryB); err != nil {
		t.Fatalf("insert B: %v", err)
	}

	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 row after consolidation, got %d", result.Total)
	}
	row := result.Items[0]
	if row.ID != entryA.ID {
		t.Errorf("expected surviving row ID %d (entryA), got %d", entryA.ID, row.ID)
	}
	if !row.StartedAt.Equal(entryA.StartedAt) {
		t.Errorf("started_at = %v, want %v (entryA's start)", row.StartedAt, entryA.StartedAt)
	}
	if !row.StoppedAt.Equal(now) {
		t.Errorf("stopped_at = %v, want %v", row.StoppedAt, now)
	}
	if row.DurationMs != 100000 {
		t.Errorf("duration_ms = %d, want 100000", row.DurationMs)
	}
	if row.WatchedMs != 50000 {
		t.Errorf("watched_ms = %d, want 50000", row.WatchedMs)
	}
	if row.PausedMs != 8000 {
		t.Errorf("paused_ms = %d, want 8000", row.PausedMs)
	}
}

func TestInsertHistoryConsolidatesDifferentTitleNoMerge(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Movie 1", StartedAt: now.Add(-20 * time.Minute), StoppedAt: now.Add(-10 * time.Minute),
	}); err != nil {
		t.Fatalf("insert 1: %v", err)
	}
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Movie 2", StartedAt: now.Add(-5 * time.Minute), StoppedAt: now,
	}); err != nil {
		t.Fatalf("insert 2: %v", err)
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 2 {
		t.Fatalf("expected 2 rows (different titles), got %d", result.Total)
	}
}

func TestInsertHistoryConsolidateBoundary(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	base := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	// Entry A: stopped at base
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", StartedAt: base.Add(-30 * time.Minute), StoppedAt: base,
	}); err != nil {
		t.Fatalf("insert A: %v", err)
	}

	// Entry B: starts exactly 30 minutes after A stopped — at boundary, should consolidate (<= is inclusive)
	entryB := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", StartedAt: base.Add(30 * time.Minute), StoppedAt: base.Add(40 * time.Minute),
	}
	if err := s.InsertHistory(entryB); err != nil {
		t.Fatalf("insert B (30min gap): %v", err)
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 1 {
		t.Fatalf("30min gap: expected 1 row (consolidated), got %d", result.Total)
	}

	// Entry C: starts 30min + 1s after the consolidated row's new stopped_at — outside window
	newStoppedAt := base.Add(40 * time.Minute)
	entryC := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", StartedAt: newStoppedAt.Add(30*time.Minute + time.Second), StoppedAt: newStoppedAt.Add(45 * time.Minute),
	}
	if err := s.InsertHistory(entryC); err != nil {
		t.Fatalf("insert C (30min+1s gap): %v", err)
	}

	result, _ = s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 2 {
		t.Fatalf("30min+1s gap: expected 2 rows (not consolidated), got %d", result.Total)
	}
}

func TestInsertHistoryConsolidatesWatchedFlag(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	entryA := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", DurationMs: 100000, WatchedMs: 50000,
		StartedAt: now.Add(-20 * time.Minute), StoppedAt: now.Add(-10 * time.Minute),
	}
	if err := s.InsertHistory(entryA); err != nil {
		t.Fatalf("insert A: %v", err)
	}

	entryB := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", DurationMs: 100000, WatchedMs: 40000,
		StartedAt: now.Add(-5 * time.Minute), StoppedAt: now,
	}
	if err := s.InsertHistory(entryB); err != nil {
		t.Fatalf("insert B: %v", err)
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 1 {
		t.Fatalf("expected 1 row, got %d", result.Total)
	}
	if result.Items[0].WatchedMs != 90000 {
		t.Errorf("watched_ms = %d, want 90000", result.Items[0].WatchedMs)
	}
	if !result.Items[0].Watched {
		t.Error("expected watched=true (90% >= 85%)")
	}
}

func TestInsertHistoryBatchConsolidates(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", DurationMs: 100000, WatchedMs: 30000,
		StartedAt: now.Add(-20 * time.Minute), StoppedAt: now.Add(-10 * time.Minute),
	}); err != nil {
		t.Fatalf("pre-insert: %v", err)
	}

	entries := []*models.WatchHistoryEntry{
		{
			ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
			Title: "The Matrix", DurationMs: 100000, WatchedMs: 20000,
			StartedAt: now.Add(-5 * time.Minute), StoppedAt: now,
		},
		{
			ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
			Title: "Different Movie",
			StartedAt: now, StoppedAt: now.Add(10 * time.Minute),
		},
	}

	inserted, _, consolidated, err := s.InsertHistoryBatch(context.Background(), entries)
	if err != nil {
		t.Fatalf("InsertHistoryBatch: %v", err)
	}
	if inserted != 1 {
		t.Errorf("expected 1 inserted, got %d", inserted)
	}
	if consolidated != 1 {
		t.Errorf("expected 1 consolidated, got %d", consolidated)
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 2 {
		t.Fatalf("expected 2 rows (1 consolidated + 1 new), got %d", result.Total)
	}
	for _, item := range result.Items {
		if item.Title == "The Matrix" && item.WatchedMs != 50000 {
			t.Errorf("consolidated row watched_ms = %d, want 50000", item.WatchedMs)
		}
	}
}

func TestInsertHistoryConsolidatesOverlapping(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	entryA := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", DurationMs: 100000, WatchedMs: 60000,
		StartedAt: now.Add(-30 * time.Minute), StoppedAt: now.Add(-5 * time.Minute),
	}
	if err := s.InsertHistory(entryA); err != nil {
		t.Fatalf("insert A: %v", err)
	}

	entryB := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", DurationMs: 100000, WatchedMs: 20000,
		StartedAt: now.Add(-15 * time.Minute), StoppedAt: now,
	}
	if err := s.InsertHistory(entryB); err != nil {
		t.Fatalf("insert B: %v", err)
	}

	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 row after overlapping consolidation, got %d", result.Total)
	}
	row := result.Items[0]
	if row.WatchedMs != 80000 {
		t.Errorf("watched_ms = %d, want 80000", row.WatchedMs)
	}
	if !row.StoppedAt.Equal(now) {
		t.Errorf("stopped_at = %v, want %v (later of the two)", row.StoppedAt, now)
	}
	if !row.StartedAt.Equal(entryA.StartedAt) {
		t.Errorf("started_at = %v, want %v (entryA's start)", row.StartedAt, entryA.StartedAt)
	}
}

func TestInsertHistoryConsolidatesChain(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	base := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	entries := []struct {
		start, stop time.Time
		watchedMs   int64
	}{
		{base, base.Add(10 * time.Minute), 10000},
		{base.Add(10 * time.Minute), base.Add(20 * time.Minute), 10000},
		{base.Add(20 * time.Minute), base.Add(30 * time.Minute), 10000},
		{base.Add(30 * time.Minute), base.Add(40 * time.Minute), 10000},
	}
	for i, e := range entries {
		if err := s.InsertHistory(&models.WatchHistoryEntry{
			ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
			Title: "The Matrix", DurationMs: 100000, WatchedMs: e.watchedMs,
			StartedAt: e.start, StoppedAt: e.stop,
		}); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 row after 4-entry chain consolidation, got %d", result.Total)
	}
	row := result.Items[0]
	if row.WatchedMs != 40000 {
		t.Errorf("watched_ms = %d, want 40000", row.WatchedMs)
	}
	if !row.StoppedAt.Equal(base.Add(40 * time.Minute)) {
		t.Errorf("stopped_at = %v, want %v", row.StoppedAt, base.Add(40*time.Minute))
	}
}

func TestInsertHistoryConsolidatesDifferentUser(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	now := time.Now().UTC()
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", StartedAt: now.Add(-20 * time.Minute), StoppedAt: now.Add(-10 * time.Minute),
	}); err != nil {
		t.Fatalf("insert alice: %v", err)
	}
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", StartedAt: now.Add(-5 * time.Minute), StoppedAt: now,
	}); err != nil {
		t.Fatalf("insert bob: %v", err)
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 2 {
		t.Fatalf("expected 2 rows (different users), got %d", result.Total)
	}
}

func TestInsertHistoryBatchOutOfOrder(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	base := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	// Entries arrive out of chronological order (C, A, B)
	entries := []*models.WatchHistoryEntry{
		{
			ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
			Title: "The Matrix", DurationMs: 100000, WatchedMs: 10000,
			StartedAt: base.Add(20 * time.Minute), StoppedAt: base.Add(30 * time.Minute),
		},
		{
			ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
			Title: "The Matrix", DurationMs: 100000, WatchedMs: 10000,
			StartedAt: base, StoppedAt: base.Add(10 * time.Minute),
		},
		{
			ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
			Title: "The Matrix", DurationMs: 100000, WatchedMs: 10000,
			StartedAt: base.Add(10 * time.Minute), StoppedAt: base.Add(20 * time.Minute),
		},
	}

	inserted, _, consolidated, err := s.InsertHistoryBatch(context.Background(), entries)
	if err != nil {
		t.Fatalf("InsertHistoryBatch: %v", err)
	}

	// C is inserted first. A arrives next: A.started_at < C.started_at, so A
	// won't find C as a predecessor (consolidation looks for entries that started
	// BEFORE the new entry). A is inserted. B arrives: B finds A as predecessor
	// (A.stopped_at == B.started_at, within window). B is consolidated into A.
	// Result: 2 rows (A+B merged, C separate), 1 consolidated.
	if inserted != 2 {
		t.Errorf("expected 2 inserted, got %d", inserted)
	}
	if consolidated != 1 {
		t.Errorf("expected 1 consolidated, got %d", consolidated)
	}

	result, _ := s.ListHistory(1, 10, "", "", "", nil)
	if result.Total != 2 {
		t.Errorf("expected 2 rows (out-of-order leaves gap), got %d", result.Total)
	}
}

func TestMigration037ConsolidatesChains(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	base := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	// Insert chain data directly via SQL, bypassing runtime consolidation
	for i := 0; i < 5; i++ {
		start := base.Add(time.Duration(i*10) * time.Minute)
		stop := start.Add(10 * time.Minute)
		_, err := s.db.Exec(
			`INSERT INTO watch_history (server_id, user_name, title, media_type, duration_ms, watched_ms, started_at, stopped_at, watched)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			serverID, "alice", "Chain Movie", "movie", 100000, 15000, start, stop, 0,
		)
		if err != nil {
			t.Fatalf("insert chain entry %d: %v", i, err)
		}
	}

	// Also insert a separate entry (different title) to verify it's not touched
	_, err := s.db.Exec(
		`INSERT INTO watch_history (server_id, user_name, title, media_type, duration_ms, watched_ms, started_at, stopped_at, watched)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		serverID, "alice", "Other Movie", "movie", 100000, 90000, base, base.Add(time.Hour), 1,
	)
	if err != nil {
		t.Fatalf("insert other: %v", err)
	}

	var countBefore int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM watch_history WHERE title = 'Chain Movie'`).Scan(&countBefore); err != nil {
		t.Fatalf("count before: %v", err)
	}
	if countBefore != 5 {
		t.Fatalf("expected 5 chain entries before migration, got %d", countBefore)
	}

	if _, err := s.db.Exec(`DELETE FROM schema_migrations WHERE version = 37`); err != nil {
		t.Fatalf("delete migration record: %v", err)
	}
	if err := s.Migrate(migrationsDir()); err != nil {
		t.Fatalf("re-run migration: %v", err)
	}

	var countAfter int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM watch_history WHERE title = 'Chain Movie'`).Scan(&countAfter); err != nil {
		t.Fatalf("count after: %v", err)
	}
	if countAfter != 1 {
		t.Errorf("expected 1 chain entry after migration, got %d", countAfter)
	}

	var watchedMs int64
	var stoppedAt string
	if err := s.db.QueryRow(`SELECT watched_ms, stopped_at FROM watch_history WHERE title = 'Chain Movie'`).Scan(&watchedMs, &stoppedAt); err != nil {
		t.Fatalf("query chain result: %v", err)
	}
	if watchedMs != 75000 {
		t.Errorf("consolidated watched_ms = %d, want 75000 (5 * 15000)", watchedMs)
	}

	expectedStop := base.Add(50 * time.Minute).Format(time.RFC3339)
	if stoppedAt != expectedStop {
		t.Errorf("stopped_at = %s, want %s", stoppedAt, expectedStop)
	}

	var watched int
	if err := s.db.QueryRow(`SELECT watched FROM watch_history WHERE title = 'Chain Movie'`).Scan(&watched); err != nil {
		t.Fatalf("query watched: %v", err)
	}
	if watched != 0 {
		t.Errorf("watched = %d, want 0 (75%% < 85%%)", watched)
	}

	var otherCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM watch_history WHERE title = 'Other Movie'`).Scan(&otherCount); err != nil {
		t.Fatalf("query other count: %v", err)
	}
	if otherCount != 1 {
		t.Errorf("other movie count = %d, want 1 (untouched)", otherCount)
	}
	var otherWatched int
	if err := s.db.QueryRow(`SELECT watched FROM watch_history WHERE title = 'Other Movie'`).Scan(&otherWatched); err != nil {
		t.Fatalf("query other watched: %v", err)
	}
	if otherWatched != 1 {
		t.Errorf("other movie watched = %d, want 1 (untouched)", otherWatched)
	}
}

