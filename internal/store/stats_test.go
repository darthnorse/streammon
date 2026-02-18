package store

import (
	"context"
	"slices"
	"strings"
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

	ctx := context.Background()
	stats, err := s.TopMovies(ctx, 10, StatsFilter{})
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

	ctx := context.Background()
	stats, err := s.TopMovies(ctx, 10, StatsFilter{})
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

	ctx := context.Background()
	stats, err := s.TopTVShows(ctx, 10, StatsFilter{})
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

	ctx := context.Background()
	stats, err := s.TopUsers(ctx, 10, StatsFilter{})
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

	ctx := context.Background()
	stats, err := s.LibraryStats(ctx, StatsFilter{})
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

	ctx := context.Background()
	stats, err := s.LibraryStats(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("LibraryStats: %v", err)
	}
	if stats.TotalPlays != 0 {
		t.Fatalf("expected 0 total plays, got %d", stats.TotalPlays)
	}
}

func TestConcurrentStreamsPeakByType(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	ctx := context.Background()

	base := time.Now().UTC().Add(-24 * time.Hour)

	// Session 1: direct play, 0:00 - 2:00
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", TranscodeDecision: models.TranscodeDecisionDirectPlay,
		StartedAt: base, StoppedAt: base.Add(2 * time.Hour),
	})
	// Session 2: transcode, 0:30 - 1:30
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M2", TranscodeDecision: models.TranscodeDecisionTranscode,
		StartedAt: base.Add(30 * time.Minute), StoppedAt: base.Add(90 * time.Minute),
	})
	// Session 3: direct play, 0:45 - 1:15
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "carol", MediaType: models.MediaTypeMovie,
		Title: "M3", TranscodeDecision: models.TranscodeDecisionDirectPlay,
		StartedAt: base.Add(45 * time.Minute), StoppedAt: base.Add(75 * time.Minute),
	})
	// Session 4: copy (direct stream), 0:45 - 1:00
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "dave", MediaType: models.MediaTypeMovie,
		Title: "M4", TranscodeDecision: models.TranscodeDecisionCopy,
		StartedAt: base.Add(45 * time.Minute), StoppedAt: base.Add(60 * time.Minute),
	})

	peaks, err := s.ConcurrentStreamsPeakByType(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("ConcurrentStreamsPeakByType: %v", err)
	}

	// Peak total: all 4 overlap at 0:45
	if peaks.Total != 4 {
		t.Errorf("total peak = %d, want 4", peaks.Total)
	}
	// Peak direct play: alice + carol overlap = 2
	if peaks.DirectPlay != 2 {
		t.Errorf("direct_play peak = %d, want 2", peaks.DirectPlay)
	}
	// Peak direct stream (copy): only dave = 1
	if peaks.DirectStream != 1 {
		t.Errorf("direct_stream peak = %d, want 1", peaks.DirectStream)
	}
	// Peak transcode: only bob = 1
	if peaks.Transcode != 1 {
		t.Errorf("transcode peak = %d, want 1", peaks.Transcode)
	}
	// Peak time should be set
	if peaks.PeakAt == "" {
		t.Error("expected non-empty peak_at")
	}
}

func TestConcurrentStreamsPeakByTypeEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	peaks, err := s.ConcurrentStreamsPeakByType(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("ConcurrentStreamsPeakByType: %v", err)
	}
	if peaks.Total != 0 || peaks.DirectPlay != 0 || peaks.DirectStream != 0 || peaks.Transcode != 0 {
		t.Errorf("expected all zeros, got %+v", peaks)
	}
	if peaks.PeakAt != "" {
		t.Errorf("expected empty peak_at, got %q", peaks.PeakAt)
	}
}

func TestConcurrentStreamsPeakByTypeIndependentPeaks(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	ctx := context.Background()

	base := time.Now().UTC().Add(-24 * time.Hour)

	// Phase 1 (0:00 - 1:00): 2 direct plays
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", TranscodeDecision: models.TranscodeDecisionDirectPlay,
		StartedAt: base, StoppedAt: base.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M2", TranscodeDecision: models.TranscodeDecisionDirectPlay,
		StartedAt: base, StoppedAt: base.Add(time.Hour),
	})

	// Phase 2 (2:00 - 3:00): 3 transcodes
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "carol", MediaType: models.MediaTypeMovie,
		Title: "M3", TranscodeDecision: models.TranscodeDecisionTranscode,
		StartedAt: base.Add(2 * time.Hour), StoppedAt: base.Add(3 * time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "dave", MediaType: models.MediaTypeMovie,
		Title: "M4", TranscodeDecision: models.TranscodeDecisionTranscode,
		StartedAt: base.Add(2 * time.Hour), StoppedAt: base.Add(3 * time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "eve", MediaType: models.MediaTypeMovie,
		Title: "M5", TranscodeDecision: models.TranscodeDecisionTranscode,
		StartedAt: base.Add(2 * time.Hour), StoppedAt: base.Add(3 * time.Hour),
	})

	peaks, err := s.ConcurrentStreamsPeakByType(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("ConcurrentStreamsPeakByType: %v", err)
	}

	// Peak total: 3 (the transcode phase)
	if peaks.Total != 3 {
		t.Errorf("total peak = %d, want 3", peaks.Total)
	}
	// Peak direct play: 2 (from phase 1, independent of total peak)
	if peaks.DirectPlay != 2 {
		t.Errorf("direct_play peak = %d, want 2", peaks.DirectPlay)
	}
	// Peak transcode: 3 (from phase 2)
	if peaks.Transcode != 3 {
		t.Errorf("transcode peak = %d, want 3", peaks.Transcode)
	}
}

func TestConcurrentStreamsPeakByTypeEmptyDecision(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	ctx := context.Background()

	base := time.Now().UTC().Add(-24 * time.Hour)

	// Session with empty transcode_decision (e.g. music, old imported data)
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMusic,
		Title: "Song1",
		StartedAt: base, StoppedAt: base.Add(5 * time.Minute),
	})
	// Session with explicit direct play
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M1", TranscodeDecision: models.TranscodeDecisionDirectPlay,
		StartedAt: base, StoppedAt: base.Add(time.Hour),
	})

	peaks, err := s.ConcurrentStreamsPeakByType(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("ConcurrentStreamsPeakByType: %v", err)
	}

	// Empty decision should count as direct play, so peak direct play = 2
	if peaks.DirectPlay != 2 {
		t.Errorf("direct_play peak = %d, want 2 (empty decision counts as direct play)", peaks.DirectPlay)
	}
	if peaks.Total != 2 {
		t.Errorf("total peak = %d, want 2", peaks.Total)
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

	ctx := context.Background()
	locs, err := s.AllWatchLocations(ctx, StatsFilter{})
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

	ctx := context.Background()
	locs, err := s.AllWatchLocations(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("AllWatchLocations: %v", err)
	}
	if len(locs) != 0 {
		t.Fatalf("expected 0 locations, got %d", len(locs))
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

	ctx := context.Background()

	// All time should return both
	all, err := s.TopMovies(ctx, 10, StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("all time: got %d movies, want 2", len(all))
	}

	// 30 days should return only recent
	month, err := s.TopMovies(ctx, 10, StatsFilter{Days: 30})
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
	week, err := s.TopMovies(ctx, 10, StatsFilter{Days: 7})
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

	ctx := context.Background()
	stats, err := s.TopMovies(ctx, 10, StatsFilter{})
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

	ctx := context.Background()
	stats, err := s.TopTVShows(ctx, 10, StatsFilter{})
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

	ctx := context.Background()
	stats, err := s.TopMovies(ctx, 10, StatsFilter{})
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

	ctx := context.Background()
	stats, err := s.TopTVShows(ctx, 10, StatsFilter{})
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

func TestUserDetailStats(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	// Alice watches from NYC and LA with different devices
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", WatchedMs: 7200000, IPAddress: "1.2.3.4",
		Player: "Chrome", Platform: "Windows",
		StartedAt: now.Add(-2 * time.Hour), StoppedAt: now.Add(-1 * time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M2", WatchedMs: 3600000, IPAddress: "1.2.3.4",
		Player: "Chrome", Platform: "Windows",
		StartedAt: now.Add(-1 * time.Hour), StoppedAt: now,
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeTV,
		Title: "E1", WatchedMs: 1800000, IPAddress: "5.6.7.8",
		Player: "Plex TV", Platform: "Android",
		StartedAt: now, StoppedAt: now.Add(30 * time.Minute),
	})

	// Add geo cache for the IPs
	s.SetCachedGeo(&models.GeoResult{IP: "1.2.3.4", City: "NYC", Country: "US", Lat: 40.7, Lng: -74.0})
	s.SetCachedGeo(&models.GeoResult{IP: "5.6.7.8", City: "LA", Country: "US", Lat: 34.0, Lng: -118.2})

	ctx := context.Background()
	stats, err := s.UserDetailStats(ctx, "alice")
	if err != nil {
		t.Fatalf("UserDetailStats: %v", err)
	}

	// Check session count
	if stats.SessionCount != 3 {
		t.Errorf("session_count = %d, want 3", stats.SessionCount)
	}

	// Check total hours: (7200000 + 3600000 + 1800000) / 3600000 = 3.5 hours
	if stats.TotalHours < 3.4 || stats.TotalHours > 3.6 {
		t.Errorf("total_hours = %f, want ~3.5", stats.TotalHours)
	}

	// Check locations (ordered by session_count DESC)
	if len(stats.Locations) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(stats.Locations))
	}
	if stats.Locations[0].City != "NYC" {
		t.Errorf("top location city = %q, want NYC", stats.Locations[0].City)
	}
	if stats.Locations[0].SessionCount != 2 {
		t.Errorf("NYC session_count = %d, want 2", stats.Locations[0].SessionCount)
	}
	// NYC: 2/3 = 66.67%
	if stats.Locations[0].Percentage < 66 || stats.Locations[0].Percentage > 67 {
		t.Errorf("NYC percentage = %f, want ~66.67", stats.Locations[0].Percentage)
	}
	if stats.Locations[0].LastSeen == "" {
		t.Error("NYC last_seen should not be empty")
	}

	// Check devices (ordered by session_count DESC)
	if len(stats.Devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(stats.Devices))
	}
	if stats.Devices[0].Player != "Chrome" {
		t.Errorf("top device player = %q, want Chrome", stats.Devices[0].Player)
	}
	if stats.Devices[0].Platform != "Windows" {
		t.Errorf("top device platform = %q, want Windows", stats.Devices[0].Platform)
	}
	if stats.Devices[0].SessionCount != 2 {
		t.Errorf("Chrome session_count = %d, want 2", stats.Devices[0].SessionCount)
	}
	// Chrome: 2/3 = 66.67%
	if stats.Devices[0].Percentage < 66 || stats.Devices[0].Percentage > 67 {
		t.Errorf("Chrome percentage = %f, want ~66.67", stats.Devices[0].Percentage)
	}
}

func TestUserDetailStatsEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// User with no watch history
	ctx := context.Background()
	stats, err := s.UserDetailStats(ctx, "nobody")
	if err != nil {
		t.Fatalf("UserDetailStats: %v", err)
	}

	if stats.SessionCount != 0 {
		t.Errorf("session_count = %d, want 0", stats.SessionCount)
	}
	if stats.TotalHours != 0 {
		t.Errorf("total_hours = %f, want 0", stats.TotalHours)
	}
	if len(stats.Locations) != 0 {
		t.Errorf("expected 0 locations, got %d", len(stats.Locations))
	}
	if len(stats.Devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(stats.Devices))
	}
}

func TestUserDetailStatsNoGeoData(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	// Alice has sessions but IP addresses not in geo cache
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", WatchedMs: 3600000, IPAddress: "9.9.9.9",
		Player: "Safari", Platform: "macOS",
		StartedAt: now, StoppedAt: now.Add(time.Hour),
	})

	ctx := context.Background()
	stats, err := s.UserDetailStats(ctx, "alice")
	if err != nil {
		t.Fatalf("UserDetailStats: %v", err)
	}

	// Should have session count and devices but no locations
	if stats.SessionCount != 1 {
		t.Errorf("session_count = %d, want 1", stats.SessionCount)
	}
	if len(stats.Locations) != 0 {
		t.Errorf("expected 0 locations (no geo data), got %d", len(stats.Locations))
	}
	if len(stats.Devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(stats.Devices))
	}
	if stats.Devices[0].Player != "Safari" {
		t.Errorf("device player = %q, want Safari", stats.Devices[0].Player)
	}
}

func TestActivityByDayOfWeek(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	// Create entries on different days (using UTC)
	// Monday
	monday := time.Date(2024, 1, 8, 12, 0, 0, 0, time.UTC) // Monday
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", StartedAt: monday, StoppedAt: monday.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M2", StartedAt: monday.Add(2 * time.Hour), StoppedAt: monday.Add(3 * time.Hour),
	})
	// Friday
	friday := time.Date(2024, 1, 12, 12, 0, 0, 0, time.UTC) // Friday
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M3", StartedAt: friday, StoppedAt: friday.Add(time.Hour),
	})

	ctx := context.Background()
	stats, err := s.ActivityByDayOfWeek(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("ActivityByDayOfWeek: %v", err)
	}

	if len(stats) != 7 {
		t.Fatalf("expected 7 days, got %d", len(stats))
	}

	// Monday = 1 in strftime('%w')
	if stats[1].PlayCount != 2 {
		t.Errorf("Monday play_count = %d, want 2", stats[1].PlayCount)
	}
	if stats[1].DayName != "Mon" {
		t.Errorf("Monday day_name = %q, want Mon", stats[1].DayName)
	}

	// Friday = 5 in strftime('%w')
	if stats[5].PlayCount != 1 {
		t.Errorf("Friday play_count = %d, want 1", stats[5].PlayCount)
	}
}

func TestActivityByDayOfWeekEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	stats, err := s.ActivityByDayOfWeek(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("ActivityByDayOfWeek: %v", err)
	}

	if len(stats) != 7 {
		t.Fatalf("expected 7 days, got %d", len(stats))
	}

	for i, stat := range stats {
		if stat.PlayCount != 0 {
			t.Errorf("day %d play_count = %d, want 0", i, stat.PlayCount)
		}
	}
}

func TestActivityByHour(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	// Create entries at different hours
	hour9 := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.UTC)
	hour14 := time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 0, 0, time.UTC)

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", StartedAt: hour9, StoppedAt: hour9.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M2", StartedAt: hour14, StoppedAt: hour14.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "carol", MediaType: models.MediaTypeMovie,
		Title: "M3", StartedAt: hour14.Add(10 * time.Minute), StoppedAt: hour14.Add(70 * time.Minute),
	})

	ctx := context.Background()
	stats, err := s.ActivityByHour(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("ActivityByHour: %v", err)
	}

	if len(stats) != 24 {
		t.Fatalf("expected 24 hours, got %d", len(stats))
	}

	if stats[9].PlayCount != 1 {
		t.Errorf("hour 9 play_count = %d, want 1", stats[9].PlayCount)
	}
	if stats[14].PlayCount != 2 {
		t.Errorf("hour 14 play_count = %d, want 2", stats[14].PlayCount)
	}
}

func TestActivityByHourEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	stats, err := s.ActivityByHour(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("ActivityByHour: %v", err)
	}

	if len(stats) != 24 {
		t.Fatalf("expected 24 hours, got %d", len(stats))
	}

	for i, stat := range stats {
		if stat.PlayCount != 0 {
			t.Errorf("hour %d play_count = %d, want 0", i, stat.PlayCount)
		}
	}
}

func TestPlatformDistribution(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", Platform: "Windows", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M2", Platform: "Windows", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "carol", MediaType: models.MediaTypeMovie,
		Title: "M3", Platform: "Android", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})

	ctx := context.Background()
	stats, err := s.PlatformDistribution(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("PlatformDistribution: %v", err)
	}

	if len(stats) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(stats))
	}

	// Should be ordered by count DESC
	if stats[0].Name != "Windows" {
		t.Errorf("top platform = %q, want Windows", stats[0].Name)
	}
	if stats[0].Count != 2 {
		t.Errorf("Windows count = %d, want 2", stats[0].Count)
	}
	// 2/3 = 66.67%
	if stats[0].Percentage < 66 || stats[0].Percentage > 67 {
		t.Errorf("Windows percentage = %f, want ~66.67", stats[0].Percentage)
	}
}

func TestPlatformDistributionEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	stats, err := s.PlatformDistribution(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("PlatformDistribution: %v", err)
	}

	if len(stats) != 0 {
		t.Fatalf("expected 0 platforms, got %d", len(stats))
	}
}

func TestPlayerDistribution(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", Player: "Chrome", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M2", Player: "Plex for LG", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})

	ctx := context.Background()
	stats, err := s.PlayerDistribution(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("PlayerDistribution: %v", err)
	}

	if len(stats) != 2 {
		t.Fatalf("expected 2 players, got %d", len(stats))
	}
}

func TestQualityDistribution(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	now := time.Now().UTC()

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", VideoResolution: "1080p", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M2", VideoResolution: "1080p", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "carol", MediaType: models.MediaTypeMovie,
		Title: "M3", VideoResolution: "4K", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "dave", MediaType: models.MediaTypeMovie,
		Title: "M4", VideoResolution: "", StartedAt: now, StoppedAt: now.Add(time.Hour),
	})

	ctx := context.Background()
	stats, err := s.QualityDistribution(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if len(stats) != 3 {
		t.Fatalf("expected 3 qualities (1080p, 4K, Unknown), got %d", len(stats))
	}

	// Should be ordered by count DESC
	if stats[0].Name != "1080p" {
		t.Errorf("top quality = %q, want 1080p", stats[0].Name)
	}
	if stats[0].Count != 2 {
		t.Errorf("1080p count = %d, want 2", stats[0].Count)
	}

	// Check Unknown exists for empty resolution
	found := false
	for _, stat := range stats {
		if stat.Name == "Unknown" {
			found = true
			if stat.Count != 1 {
				t.Errorf("Unknown count = %d, want 1", stat.Count)
			}
		}
	}
	if !found {
		t.Error("expected Unknown quality for empty resolution")
	}
}

func TestConcurrentStreamsOverTime(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	base := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Hour)

	// Three overlapping sessions
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "M1", TranscodeDecision: models.TranscodeDecisionDirectPlay,
		StartedAt: base, StoppedAt: base.Add(2 * time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "M2", TranscodeDecision: models.TranscodeDecisionTranscode,
		StartedAt: base.Add(30 * time.Minute), StoppedAt: base.Add(90 * time.Minute),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "carol", MediaType: models.MediaTypeMovie,
		Title: "M3", TranscodeDecision: models.TranscodeDecisionDirectPlay,
		StartedAt: base.Add(45 * time.Minute), StoppedAt: base.Add(75 * time.Minute),
	})

	ctx := context.Background()
	points, err := s.ConcurrentStreamsOverTime(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("ConcurrentStreamsOverTime: %v", err)
	}

	if len(points) == 0 {
		t.Fatal("expected at least one data point")
	}

	// Find the peak hour bucket
	var maxTotal int
	for _, p := range points {
		if p.Total > maxTotal {
			maxTotal = p.Total
		}
	}

	if maxTotal < 3 {
		t.Errorf("max concurrent = %d, want at least 3", maxTotal)
	}
}

func TestConcurrentStreamsOverTimeEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	points, err := s.ConcurrentStreamsOverTime(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("ConcurrentStreamsOverTime: %v", err)
	}

	if len(points) != 0 {
		t.Fatalf("expected 0 points, got %d", len(points))
	}
}

func TestConcurrentStreamsOverTimeHourlyBucketing(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	base := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Hour)

	// Create many sessions to verify bucketing reduces data points
	for i := 0; i < 10; i++ {
		s.InsertHistory(&models.WatchHistoryEntry{
			ServerID: serverID, UserName: "user", MediaType: models.MediaTypeMovie,
			Title: "M", TranscodeDecision: models.TranscodeDecisionDirectPlay,
			StartedAt: base.Add(time.Duration(i) * 5 * time.Minute),
			StoppedAt: base.Add(time.Duration(i)*5*time.Minute + 30*time.Minute),
		})
	}

	ctx := context.Background()
	points, err := s.ConcurrentStreamsOverTime(ctx, StatsFilter{})
	if err != nil {
		t.Fatalf("ConcurrentStreamsOverTime: %v", err)
	}

	// With 10 sessions starting/stopping in the same hour window,
	// we should have far fewer than 20 data points due to hourly bucketing
	if len(points) > 5 {
		t.Errorf("expected hourly bucketing to reduce points, got %d", len(points))
	}
}

func TestDistributionInvalidColumn(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	// Invalid column should be rejected
	_, err := s.distribution(ctx, StatsFilter{}, "evil'; DROP TABLE watch_history; --", "test")
	if err == nil {
		t.Error("expected error for invalid column")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid column") {
		t.Errorf("expected 'invalid column' error, got: %v", err)
	}

	// Valid columns should work
	_, err = s.PlatformDistribution(ctx, StatsFilter{})
	if err != nil {
		t.Errorf("PlatformDistribution should succeed: %v", err)
	}
	_, err = s.PlayerDistribution(ctx, StatsFilter{})
	if err != nil {
		t.Errorf("PlayerDistribution should succeed: %v", err)
	}
	_, err = s.QualityDistribution(ctx, StatsFilter{})
	if err != nil {
		t.Errorf("QualityDistribution should succeed: %v", err)
	}
}

func TestActivityCountsInvalidFormat(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	// Test that invalid strftime formats are rejected
	_, err := s.activityCounts(ctx, StatsFilter{}, "malicious'; DROP TABLE watch_history; --", "test")
	if err == nil {
		t.Error("expected error for invalid strftime format")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid strftime format") {
		t.Errorf("expected 'invalid strftime format' error, got: %v", err)
	}

	// Valid formats should work
	_, err = s.activityCounts(ctx, StatsFilter{}, "%w", "test")
	if err != nil {
		t.Errorf("expected success for %%w format: %v", err)
	}
	_, err = s.activityCounts(ctx, StatsFilter{}, "%H", "test")
	if err != nil {
		t.Errorf("expected success for %%H format: %v", err)
	}
}

func TestStatsFilterServerIDs(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	srv1 := &models.Server{Name: "S1", Type: models.ServerTypePlex, URL: "http://s1", APIKey: "k1", Enabled: true}
	s.CreateServer(srv1)
	srv2 := &models.Server{Name: "S2", Type: models.ServerTypePlex, URL: "http://s2", APIKey: "k2", Enabled: true}
	s.CreateServer(srv2)

	now := time.Now().UTC()

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: srv1.ID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Movie A", Year: 2020, WatchedMs: 7200000,
		StartedAt: now, StoppedAt: now.Add(2 * time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: srv1.ID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "Movie B", Year: 2021, WatchedMs: 3600000,
		StartedAt: now, StoppedAt: now.Add(time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: srv2.ID, UserName: "carol", MediaType: models.MediaTypeMovie,
		Title: "Movie C", Year: 2022, WatchedMs: 5400000,
		StartedAt: now, StoppedAt: now.Add(90 * time.Minute),
	})

	ctx := context.Background()

	all, err := s.TopMovies(ctx, 10, StatsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("all: got %d, want 3", len(all))
	}

	s1, err := s.TopMovies(ctx, 10, StatsFilter{ServerIDs: []int64{srv1.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s1) != 2 {
		t.Fatalf("server1: got %d, want 2", len(s1))
	}

	s2, err := s.TopMovies(ctx, 10, StatsFilter{ServerIDs: []int64{srv2.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s2) != 1 {
		t.Fatalf("server2: got %d, want 1", len(s2))
	}
	if s2[0].Title != "Movie C" {
		t.Fatalf("server2: got %q, want Movie C", s2[0].Title)
	}

	lib1, err := s.LibraryStats(ctx, StatsFilter{ServerIDs: []int64{srv1.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if lib1.TotalPlays != 2 {
		t.Fatalf("lib server1: got %d plays, want 2", lib1.TotalPlays)
	}

	users2, err := s.TopUsers(ctx, 10, StatsFilter{ServerIDs: []int64{srv2.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(users2) != 1 {
		t.Fatalf("users server2: got %d, want 1", len(users2))
	}
	if users2[0].UserName != "carol" {
		t.Fatalf("users server2: got %q, want carol", users2[0].UserName)
	}
}

func TestStatsFilterDateRange(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)

	jan15 := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	feb15 := time.Date(2024, 2, 15, 12, 0, 0, 0, time.UTC)
	mar15 := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Jan Movie", Year: 2024, WatchedMs: 7200000,
		StartedAt: jan15, StoppedAt: jan15.Add(2 * time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Feb Movie", Year: 2024, WatchedMs: 7200000,
		StartedAt: feb15, StoppedAt: feb15.Add(2 * time.Hour),
	})
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: serverID, UserName: "alice", MediaType: models.MediaTypeMovie,
		Title: "Mar Movie", Year: 2024, WatchedMs: 7200000,
		StartedAt: mar15, StoppedAt: mar15.Add(2 * time.Hour),
	})

	ctx := context.Background()

	filter := StatsFilter{
		StartDate: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
	}
	movies, err := s.TopMovies(ctx, 10, filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 1 {
		t.Fatalf("feb range: got %d, want 1", len(movies))
	}
	if movies[0].Title != "Feb Movie" {
		t.Fatalf("feb range: got %q, want Feb Movie", movies[0].Title)
	}

	filter2 := StatsFilter{
		StartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
	}
	movies2, err := s.TopMovies(ctx, 10, filter2)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies2) != 2 {
		t.Fatalf("jan-feb range: got %d, want 2", len(movies2))
	}
}

