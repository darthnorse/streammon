package store

import (
	"testing"
	"time"

	"streammon/internal/models"
)

func TestTopMovies(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", Year: 1999, WatchedMs: 7200000,
		StartedAt: now, StoppedAt: now.Add(2 * time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", Year: 1999, WatchedMs: 7200000,
		StartedAt: now, StoppedAt: now.Add(2 * time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Inception", Year: 2010, WatchedMs: 5400000,
		StartedAt: now, StoppedAt: now.Add(90 * time.Minute),
	})

	stats, err := s.TopMovies(10)
	if err != nil {
		t.Fatalf("TopMovies: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 movies, got %d", len(stats))
	}
	if stats[0].Title != "The Matrix" {
		t.Fatalf("expected The Matrix first, got %s", stats[0].Title)
	}
	if stats[0].PlayCount != 2 {
		t.Fatalf("expected 2 plays, got %d", stats[0].PlayCount)
	}
}

func TestTopMoviesEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	stats, err := s.TopMovies(10)
	if err != nil {
		t.Fatalf("TopMovies: %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("expected 0 movies, got %d", len(stats))
	}
}

func TestTopTVShows(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "S01E01", GrandparentTitle: "Breaking Bad", WatchedMs: 3600000,
		StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "S01E02", GrandparentTitle: "Breaking Bad", WatchedMs: 3600000,
		StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeTV,
		Title: "S01E01", GrandparentTitle: "The Office", WatchedMs: 1800000,
		StartedAt: now, StoppedAt: now.Add(30 * time.Minute),
	})

	stats, err := s.TopTVShows(10)
	if err != nil {
		t.Fatalf("TopTVShows: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 shows, got %d", len(stats))
	}
	if stats[0].Title != "Breaking Bad" {
		t.Fatalf("expected Breaking Bad first, got %s", stats[0].Title)
	}
	if stats[0].PlayCount != 2 {
		t.Fatalf("expected 2 plays, got %d", stats[0].PlayCount)
	}
}

func TestTopUsers(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", WatchedMs: 7200000, StartedAt: now, StoppedAt: now.Add(2 * time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M2", WatchedMs: 3600000, StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M1", WatchedMs: 1800000, StartedAt: now, StoppedAt: now.Add(30 * time.Minute),
	})

	stats, err := s.TopUsers(10)
	if err != nil {
		t.Fatalf("TopUsers: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 users, got %d", len(stats))
	}
	if stats[0].UserName != "alice" {
		t.Fatalf("expected alice first, got %s", stats[0].UserName)
	}
	if stats[0].PlayCount != 2 {
		t.Fatalf("expected 2 plays for alice, got %d", stats[0].PlayCount)
	}
}

func TestLibraryStats(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "The Matrix", Year: 1999, WatchedMs: 7200000,
		StartedAt: now, StoppedAt: now.Add(2 * time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "Inception", Year: 2010, WatchedMs: 5400000,
		StartedAt: now, StoppedAt: now.Add(90 * time.Minute),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "S01E01", GrandparentTitle: "Breaking Bad", WatchedMs: 3600000,
		StartedAt: now, StoppedAt: now.Add(time.Hour),
	})

	stats, err := s.LibraryStats()
	if err != nil {
		t.Fatalf("LibraryStats: %v", err)
	}
	if stats.TotalPlays != 3 {
		t.Fatalf("expected 3 total plays, got %d", stats.TotalPlays)
	}
	if stats.UniqueUsers != 2 {
		t.Fatalf("expected 2 unique users, got %d", stats.UniqueUsers)
	}
	if stats.UniqueMovies != 2 {
		t.Fatalf("expected 2 unique movies, got %d", stats.UniqueMovies)
	}
	if stats.UniqueTVShows != 1 {
		t.Fatalf("expected 1 unique TV show, got %d", stats.UniqueTVShows)
	}
}

func TestLibraryStatsEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	stats, err := s.LibraryStats()
	if err != nil {
		t.Fatalf("LibraryStats: %v", err)
	}
	if stats.TotalPlays != 0 {
		t.Fatalf("expected 0 total plays, got %d", stats.TotalPlays)
	}
}

func TestConcurrentStreamsPeak(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	base := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", StartedAt: base, StoppedAt: base.Add(2 * time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M2", StartedAt: base.Add(30 * time.Minute), StoppedAt: base.Add(90 * time.Minute),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "carol", MediaType: models.MediaTypeMovie,
		Title: "M3", StartedAt: base.Add(45 * time.Minute), StoppedAt: base.Add(75 * time.Minute),
	})

	peak, _, err := s.ConcurrentStreamsPeak()
	if err != nil {
		t.Fatalf("ConcurrentStreamsPeak: %v", err)
	}
	if peak != 3 {
		t.Fatalf("expected peak of 3, got %d", peak)
	}
}

func TestConcurrentStreamsPeakEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	peak, _, err := s.ConcurrentStreamsPeak()
	if err != nil {
		t.Fatalf("ConcurrentStreamsPeak: %v", err)
	}
	if peak != 0 {
		t.Fatalf("expected peak of 0, got %d", peak)
	}
}

func TestAllWatchLocations(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", IPAddress: "1.2.3.4", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M2", IPAddress: "5.6.7.8", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})

	s.SetCachedGeo(&models.GeoResult{IP: "1.2.3.4", City: "NYC", Country: "US", Lat: 40.7, Lng: -74.0})
	s.SetCachedGeo(&models.GeoResult{IP: "5.6.7.8", City: "LA", Country: "US", Lat: 34.0, Lng: -118.2})

	locs, err := s.AllWatchLocations()
	if err != nil {
		t.Fatalf("AllWatchLocations: %v", err)
	}
	if len(locs) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(locs))
	}
}

func TestPotentialSharers(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", IPAddress: "1.1.1.1", StartedAt: now.Add(-1 * 24 * time.Hour), StoppedAt: now,
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M2", IPAddress: "2.2.2.2", StartedAt: now.Add(-2 * 24 * time.Hour), StoppedAt: now,
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M3", IPAddress: "3.3.3.3", StartedAt: now.Add(-3 * 24 * time.Hour), StoppedAt: now,
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M1", IPAddress: "4.4.4.4", StartedAt: now, StoppedAt: now,
	})

	s.SetCachedGeo(&models.GeoResult{IP: "1.1.1.1", City: "NYC", Country: "US", Lat: 40.7, Lng: -74.0})
	s.SetCachedGeo(&models.GeoResult{IP: "2.2.2.2", City: "LA", Country: "US", Lat: 34.0, Lng: -118.2})
	s.SetCachedGeo(&models.GeoResult{IP: "3.3.3.3", City: "Chicago", Country: "US", Lat: 41.9, Lng: -87.6})

	sharers, err := s.PotentialSharers(3, 30)
	if err != nil {
		t.Fatalf("PotentialSharers: %v", err)
	}
	if len(sharers) != 1 {
		t.Fatalf("expected 1 potential sharer, got %d", len(sharers))
	}
	if sharers[0].UserName != "alice" {
		t.Fatalf("expected alice, got %s", sharers[0].UserName)
	}
	if sharers[0].UniqueIPs != 3 {
		t.Fatalf("expected 3 unique IPs, got %d", sharers[0].UniqueIPs)
	}
}

func TestPotentialSharersNone(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", IPAddress: "1.1.1.1", StartedAt: now, StoppedAt: now,
	})

	sharers, err := s.PotentialSharers(3, 30)
	if err != nil {
		t.Fatalf("PotentialSharers: %v", err)
	}
	if len(sharers) != 0 {
		t.Fatalf("expected 0 potential sharers, got %d", len(sharers))
	}
}
