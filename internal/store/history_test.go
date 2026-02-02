package store

import (
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

	result, err := s.ListHistory(1, 10, "")
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

	result, err := s.ListHistory(1, 10, "alice")
	if err != nil {
		t.Fatalf("ListHistory(alice): %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 for alice, got %d", result.Total)
	}
}

func TestListHistoryPagination(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	serverID := seedServer(t, s)
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		s.InsertHistory(&models.WatchHistoryEntry{
			ServerID: serverID, UserName: "u", MediaType: models.MediaTypeMovie,
			Title: "M", StartedAt: now, StoppedAt: now,
		})
	}

	result, err := s.ListHistory(2, 2, "")
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

	stats, err := s.DailyWatchCounts(start, end)
	if err != nil {
		t.Fatalf("DailyWatchCounts: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 days, got %d", len(stats))
	}
}

func TestDailyWatchCountsEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	start := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC)

	stats, err := s.DailyWatchCounts(start, end)
	if err != nil {
		t.Fatalf("DailyWatchCounts: %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("expected 0, got %d", len(stats))
	}
}
