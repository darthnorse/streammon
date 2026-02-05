package store

import (
	"context"
	"errors"
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

	result, err := s.ListHistory(1, 10, "", "", "")
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

	result, err := s.ListHistory(1, 10, "alice", "", "")
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

	result, err := s.ListHistory(2, 2, "", "", "")
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

	differentTime := startedAt.Add(time.Hour)
	exists, err = s.HistoryExists(serverID, "alice", "The Matrix", differentTime)
	if err != nil {
		t.Fatalf("HistoryExists (different time): %v", err)
	}
	if exists {
		t.Fatal("expected entry not to exist for different time")
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

	inserted, skipped, err := s.InsertHistoryBatch(context.Background(), entries)
	if err != nil {
		t.Fatalf("InsertHistoryBatch: %v", err)
	}
	if inserted != 2 {
		t.Errorf("expected 2 inserted, got %d", inserted)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", skipped)
	}

	result, _ := s.ListHistory(1, 10, "", "", "")
	if result.Total != 2 {
		t.Fatalf("expected 2 entries in history, got %d", result.Total)
	}
}

func TestInsertHistoryBatchSkipsDuplicates(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	startedAt := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID:  serverID,
		UserName:  "alice",
		MediaType: models.MediaTypeMovie,
		Title:     "Existing Movie",
		StartedAt: startedAt,
		StoppedAt: startedAt.Add(2 * time.Hour),
	})

	entries := []*models.WatchHistoryEntry{
		{
			ServerID:  serverID,
			UserName:  "alice",
			MediaType: models.MediaTypeMovie,
			Title:     "Existing Movie",
			StartedAt: startedAt,
			StoppedAt: startedAt.Add(2 * time.Hour),
		},
		{
			ServerID:  serverID,
			UserName:  "bob",
			MediaType: models.MediaTypeTV,
			Title:     "New Episode",
			StartedAt: startedAt.Add(time.Hour),
			StoppedAt: startedAt.Add(2 * time.Hour),
		},
	}

	inserted, skipped, err := s.InsertHistoryBatch(context.Background(), entries)
	if err != nil {
		t.Fatalf("InsertHistoryBatch: %v", err)
	}
	if inserted != 1 {
		t.Errorf("expected 1 inserted, got %d", inserted)
	}
	if skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", skipped)
	}

	result, _ := s.ListHistory(1, 10, "", "", "")
	if result.Total != 2 {
		t.Fatalf("expected 2 entries in history (1 original + 1 new), got %d", result.Total)
	}
}

func TestInsertHistoryBatchEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	inserted, skipped, err := s.InsertHistoryBatch(context.Background(), []*models.WatchHistoryEntry{})
	if err != nil {
		t.Fatalf("InsertHistoryBatch (empty): %v", err)
	}
	if inserted != 0 || skipped != 0 {
		t.Errorf("expected 0/0, got %d/%d", inserted, skipped)
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

	_, _, err := s.InsertHistoryBatch(ctx, entries)
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

	// Insert test history entries
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

	// Test: Get alice's last stream before now
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

	// Test: Get alice's last stream before the second entry
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

	// Test: No entry found outside window
	// Query from 3 hours ago with 1 hour window - no entries exist before 3 hours ago
	entry, err = s.GetLastStreamBeforeTime("alice", now.Add(-3*time.Hour), 1)
	if err != nil {
		t.Fatalf("GetLastStreamBeforeTime (outside window): %v", err)
	}
	if entry != nil {
		t.Errorf("expected nil entry for outside window, got %+v", entry)
	}

	// Test: Unknown user returns nil
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

	// Test: Get last Chrome stream before now
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

	// Test: Get last Chrome stream before Movie 3
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

	// Test: Get last iOS stream
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

	// Test: Unknown device returns nil
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

	// Test: Device has been used before now
	used, err := s.HasDeviceBeenUsed("alice", "Plex Web", "Chrome", now)
	if err != nil {
		t.Fatalf("HasDeviceBeenUsed: %v", err)
	}
	if !used {
		t.Error("expected device to have been used")
	}

	// Test: Device not used before the entry was created
	used, err = s.HasDeviceBeenUsed("alice", "Plex Web", "Chrome", now.Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("HasDeviceBeenUsed (before entry): %v", err)
	}
	if used {
		t.Error("expected device not to have been used before entry time")
	}

	// Test: Different device not used
	used, err = s.HasDeviceBeenUsed("alice", "Plex for iOS", "iOS", now)
	if err != nil {
		t.Fatalf("HasDeviceBeenUsed (different device): %v", err)
	}
	if used {
		t.Error("expected different device not to have been used")
	}

	// Test: Different user not used this device
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
		{ServerID: serverID, UserName: "alice", Title: "Movie 3", StartedAt: now.Add(-1 * time.Hour), IPAddress: "1.1.1.1", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie}, // duplicate IP
		{ServerID: serverID, UserName: "bob", Title: "Movie 4", StartedAt: now.Add(-30 * time.Minute), IPAddress: "3.3.3.3", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
	}
	for i := range entries {
		if err := s.InsertHistory(&entries[i]); err != nil {
			t.Fatalf("InsertHistory: %v", err)
		}
	}

	// Test: Get alice's distinct IPs
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

	// Test: Limit works
	ips, err = s.GetUserDistinctIPs("alice", now, 1)
	if err != nil {
		t.Fatalf("GetUserDistinctIPs (limit 1): %v", err)
	}
	if len(ips) != 1 {
		t.Errorf("expected 1 IP with limit, got %d", len(ips))
	}

	// Test: Unknown user returns empty
	ips, err = s.GetUserDistinctIPs("unknown", now, 100)
	if err != nil {
		t.Fatalf("GetUserDistinctIPs (unknown user): %v", err)
	}
	if len(ips) != 0 {
		t.Errorf("expected 0 IPs for unknown user, got %d", len(ips))
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
		{ServerID: serverID, UserName: "alice", Title: "Movie 3", StartedAt: now.Add(-1 * time.Hour), IPAddress: "1.1.1.1", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie}, // duplicate ISP
		{ServerID: serverID, UserName: "alice", Title: "Movie 4", StartedAt: now.Add(-30 * time.Minute), IPAddress: "4.4.4.4", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie}, // Empty ISP - should not be counted
		{ServerID: serverID, UserName: "bob", Title: "Movie 5", StartedAt: now.Add(-30 * time.Minute), IPAddress: "3.3.3.3", Player: "Plex Web", Platform: "Chrome", MediaType: models.MediaTypeMovie},
	}
	for i := range entries {
		if err := s.InsertHistory(&entries[i]); err != nil {
			t.Fatalf("InsertHistory: %v", err)
		}
	}

	// Test: Get alice's recent ISPs (within time window)
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

	// Test: Time window filtering
	isps, err = s.GetRecentISPs("alice", now.Add(-2*time.Hour), 1) // Only entries from 3 hours ago
	if err != nil {
		t.Fatalf("GetRecentISPs (time window): %v", err)
	}
	if len(isps) != 1 {
		t.Errorf("expected 1 ISP in time window, got %d: %v", len(isps), isps)
	}

	// Test: Unknown user returns empty
	isps, err = s.GetRecentISPs("unknown", now, 24)
	if err != nil {
		t.Fatalf("GetRecentISPs (unknown user): %v", err)
	}
	if len(isps) != 0 {
		t.Errorf("expected 0 ISPs for unknown user, got %d", len(isps))
	}

	// Test: Empty ISPs are not included
	// Alice used IP 4.4.4.4 which has empty ISP - should not appear
	for _, isp := range isps {
		if isp == "" {
			t.Error("empty ISP should not be in result")
		}
	}
}
