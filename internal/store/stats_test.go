package store

import (
	"slices"
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

	stats, err := s.TopMovies(10, 0)
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
	// 2 plays * 7200000ms = 14400000ms = 4 hours
	if stats[0].TotalHours < 3.9 || stats[0].TotalHours > 4.1 {
		t.Fatalf("expected ~4 total hours, got %f", stats[0].TotalHours)
	}
}

func TestTopMoviesEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	stats, err := s.TopMovies(10, 0)
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

	stats, err := s.TopTVShows(10, 0)
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
	// 2 plays * 3600000ms = 7200000ms = 2 hours
	if stats[0].TotalHours < 1.9 || stats[0].TotalHours > 2.1 {
		t.Fatalf("expected ~2 total hours, got %f", stats[0].TotalHours)
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

	stats, err := s.TopUsers(10, 0)
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
	// alice: 7200000ms + 3600000ms = 10800000ms = 3 hours
	if stats[0].TotalHours < 2.9 || stats[0].TotalHours > 3.1 {
		t.Fatalf("expected ~3 total hours for alice, got %f", stats[0].TotalHours)
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
	// 7200000 + 5400000 + 3600000 = 16200000ms = 4.5 hours
	if stats.TotalHours < 4.4 || stats.TotalHours > 4.6 {
		t.Fatalf("expected ~4.5 total hours, got %f", stats.TotalHours)
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

	base := time.Now().UTC().Add(-24 * time.Hour)
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

	// Alice watches from NYC
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", IPAddress: "1.2.3.4", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	// Bob watches from LA
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M2", IPAddress: "5.6.7.8", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	// Carol also watches from NYC (same location as Alice, different IP)
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "carol", MediaType: models.MediaTypeMovie,
		Title: "M3", IPAddress: "1.2.3.5", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})

	// NYC has two different ISPs - MAX() will pick "Verizon" (alphabetically last)
	s.SetCachedGeo(&models.GeoResult{IP: "1.2.3.4", City: "NYC", Country: "US", Lat: 40.7, Lng: -74.0, ISP: "Comcast"})
	s.SetCachedGeo(&models.GeoResult{IP: "1.2.3.5", City: "NYC", Country: "US", Lat: 40.7, Lng: -74.0, ISP: "Verizon"})
	s.SetCachedGeo(&models.GeoResult{IP: "5.6.7.8", City: "LA", Country: "US", Lat: 34.0, Lng: -118.2, ISP: "AT&T"})

	locs, err := s.AllWatchLocations()
	if err != nil {
		t.Fatalf("AllWatchLocations: %v", err)
	}
	if len(locs) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(locs))
	}

	// Results are ordered by country, city - so LA comes before NYC
	laLoc := locs[0]
	nycLoc := locs[1]

	if laLoc.City != "LA" {
		t.Fatalf("expected first location to be LA, got %s", laLoc.City)
	}
	if laLoc.ISP != "AT&T" {
		t.Fatalf("expected LA ISP to be AT&T, got %s", laLoc.ISP)
	}
	if len(laLoc.Users) != 1 || laLoc.Users[0] != "bob" {
		t.Fatalf("expected LA to have user bob, got %v", laLoc.Users)
	}

	if nycLoc.City != "NYC" {
		t.Fatalf("expected second location to be NYC, got %s", nycLoc.City)
	}
	// NYC has two ISPs (Comcast, Verizon) - MAX() picks "Verizon" (alphabetically last)
	if nycLoc.ISP != "Verizon" {
		t.Fatalf("expected NYC ISP to be Verizon (MAX of multiple ISPs), got %s", nycLoc.ISP)
	}
	if len(nycLoc.Users) != 2 {
		t.Fatalf("expected NYC to have 2 users, got %d", len(nycLoc.Users))
	}
	if !slices.Contains(nycLoc.Users, "alice") || !slices.Contains(nycLoc.Users, "carol") {
		t.Fatalf("expected NYC to have alice and carol, got %v", nycLoc.Users)
	}
}

func TestAllWatchLocationsEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	locs, err := s.AllWatchLocations()
	if err != nil {
		t.Fatalf("AllWatchLocations: %v", err)
	}
	if len(locs) != 0 {
		t.Fatalf("expected 0 locations, got %d", len(locs))
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

	s.SetCachedGeo(&models.GeoResult{IP: "1.1.1.1", City: "NYC", Country: "US", Lat: 40.7, Lng: -74.0, ISP: "Verizon"})
	s.SetCachedGeo(&models.GeoResult{IP: "2.2.2.2", City: "LA", Country: "US", Lat: 34.0, Lng: -118.2, ISP: "AT&T"})
	s.SetCachedGeo(&models.GeoResult{IP: "3.3.3.3", City: "Chicago", Country: "US", Lat: 41.9, Lng: -87.6, ISP: "Comcast"})

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

func TestTopMoviesWithTimeFilter(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	// Insert old record (40 days ago)
	oldDate := time.Now().UTC().AddDate(0, 0, -40)
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "user1", MediaType: models.MediaTypeMovie,
		Title: "Old Movie", Year: 2020, DurationMs: 7200000, WatchedMs: 7200000,
		StartedAt: oldDate, StoppedAt: oldDate.Add(2 * time.Hour),
	})

	// Insert recent record (5 days ago)
	recentDate := time.Now().UTC().AddDate(0, 0, -5)
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "user1", MediaType: models.MediaTypeMovie,
		Title: "Recent Movie", Year: 2023, DurationMs: 7200000, WatchedMs: 7200000,
		StartedAt: recentDate, StoppedAt: recentDate.Add(2 * time.Hour),
	})

	// All time should return both
	all, err := s.TopMovies(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("all time: got %d movies, want 2", len(all))
	}

	// 30 days should return only recent
	month, err := s.TopMovies(10, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(month) != 1 {
		t.Errorf("30 days: got %d movies, want 1", len(month))
	}
	if month[0].Title != "Recent Movie" {
		t.Errorf("30 days: got %q, want Recent Movie", month[0].Title)
	}

	// 7 days should return only recent
	week, err := s.TopMovies(10, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(week) != 1 {
		t.Errorf("7 days: got %d movies, want 1", len(week))
	}
}

func TestTopMoviesWithThumbURL(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	// Insert entry without thumb_url
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Movie A", Year: 2020, WatchedMs: 7200000,
		StartedAt: now.Add(-2 * time.Hour), StoppedAt: now.Add(-1 * time.Hour),
		ThumbURL: "",
	})

	// Insert entry with thumb_url (more recent)
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "Movie A", Year: 2020, WatchedMs: 7200000,
		StartedAt: now, StoppedAt: now.Add(1 * time.Hour),
		ThumbURL: "library/metadata/123/thumb",
	})

	stats, err := s.TopMovies(10, 0)
	if err != nil {
		t.Fatalf("TopMovies: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 movie, got %d", len(stats))
	}
	if stats[0].ThumbURL != "library/metadata/123/thumb" {
		t.Errorf("thumb_url = %q, want library/metadata/123/thumb", stats[0].ThumbURL)
	}
	if stats[0].ServerID != serverID {
		t.Errorf("server_id = %d, want %d", stats[0].ServerID, serverID)
	}
}

func TestTopTVShowsWithThumbURL(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	// Insert TV show entry with thumb_url
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "Episode 1", GrandparentTitle: "Breaking Bad", WatchedMs: 3600000,
		StartedAt: now, StoppedAt: now.Add(1 * time.Hour),
		ThumbURL: "library/metadata/456/thumb",
	})

	stats, err := s.TopTVShows(10, 0)
	if err != nil {
		t.Fatalf("TopTVShows: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 show, got %d", len(stats))
	}
	if stats[0].Title != "Breaking Bad" {
		t.Errorf("title = %q, want Breaking Bad", stats[0].Title)
	}
	if stats[0].ThumbURL != "library/metadata/456/thumb" {
		t.Errorf("thumb_url = %q, want library/metadata/456/thumb", stats[0].ThumbURL)
	}
	if stats[0].ServerID != serverID {
		t.Errorf("server_id = %d, want %d", stats[0].ServerID, serverID)
	}
}

func TestTopMoviesWithItemID(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	// Insert entry without item_id
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Movie A", Year: 2020, WatchedMs: 7200000,
		StartedAt: now.Add(-2 * time.Hour), StoppedAt: now.Add(-1 * time.Hour),
		ItemID: "",
	})

	// Insert entry with item_id (more recent)
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "Movie A", Year: 2020, WatchedMs: 7200000,
		StartedAt: now, StoppedAt: now.Add(1 * time.Hour),
		ItemID:   "12345",
		ThumbURL: "library/metadata/12345/thumb",
	})

	stats, err := s.TopMovies(10, 0)
	if err != nil {
		t.Fatalf("TopMovies: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 movie, got %d", len(stats))
	}
	if stats[0].ItemID != "12345" {
		t.Errorf("item_id = %q, want 12345", stats[0].ItemID)
	}
	if stats[0].ServerID != serverID {
		t.Errorf("server_id = %d, want %d", stats[0].ServerID, serverID)
	}
}

func TestTopTVShowsWithItemID(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	// Insert TV show entry with grandparent_item_id (series ID) and item_id (episode ID)
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "Ozymandias", GrandparentTitle: "Breaking Bad", WatchedMs: 3600000,
		StartedAt: now, StoppedAt: now.Add(1 * time.Hour),
		ItemID:            "67890",
		GrandparentItemID: "12345",
		ThumbURL:          "library/metadata/67890/thumb",
	})

	stats, err := s.TopTVShows(10, 0)
	if err != nil {
		t.Fatalf("TopTVShows: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 show, got %d", len(stats))
	}
	if stats[0].Title != "Breaking Bad" {
		t.Errorf("title = %q, want Breaking Bad", stats[0].Title)
	}
	// item_id returns the series ID (grandparent_item_id) for TV shows
	// This allows clicking to open the series detail, not the episode
	if stats[0].ItemID != "12345" {
		t.Errorf("item_id = %q, want 12345 (series ID)", stats[0].ItemID)
	}
	if stats[0].ServerID != serverID {
		t.Errorf("server_id = %d, want %d", stats[0].ServerID, serverID)
	}
}

