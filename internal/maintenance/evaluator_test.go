package maintenance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

func TestParseResolutionHeight(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		// Standard resolutions
		{"4K", 2160},
		{"4k", 2160},
		{"2160p", 2160},
		{"2160", 2160},
		{"1080p", 1080},
		{"1080", 1080},
		{"720p", 720},
		{"720", 720},
		{"480p", 480},
		{"480", 480},
		{"360p", 360},
		{"360", 360},
		{"240p", 240},
		{"240", 240},

		// Named resolutions
		{"FHD", 1080},
		{"fhd", 1080},
		{"HD", 720},
		{"hd", 720},
		{"SD", 480},
		{"sd", 480},
		{"UHD", 2160},
		{"uhd", 2160},
		{"8K", 4320},
		{"8k", 4320},

		// Non-standard resolutions (issue #5 fix)
		{"576p", 576},
		{"540p", 540},
		{"544p", 544},
		{"1440p", 1440},

		// Unknown/empty
		{"", 0},
		{"unknown", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseResolutionHeight(tt.input)
			if result != tt.expected {
				t.Errorf("parseResolutionHeight(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDefaultConstants(t *testing.T) {
	// Verify defaults are sensible
	if DefaultDays != 365 {
		t.Errorf("DefaultDays = %d, want 365", DefaultDays)
	}
	if DefaultMaxPercent != 10 {
		t.Errorf("DefaultMaxPercent = %d, want 10", DefaultMaxPercent)
	}
	if DefaultMaxHeight != 720 {
		t.Errorf("DefaultMaxHeight = %d, want 720", DefaultMaxHeight)
	}
	if DefaultMinSizeGB != 10.0 {
		t.Errorf("DefaultMinSizeGB = %f, want 10.0", DefaultMinSizeGB)
	}
}

// Test helpers for integration tests

func migrationsDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "migrations")
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("New(:memory:) failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newTestStoreWithMigrations(t *testing.T) *store.Store {
	t.Helper()
	s := newTestStore(t)
	dir := migrationsDir()
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("migrations dir not found: %v", err)
	}
	if err := s.Migrate(dir); err != nil {
		t.Fatalf("Migrate() failed: %v", err)
	}
	return s
}

func seedTestServer(t *testing.T, s *store.Store) *models.Server {
	t.Helper()
	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}
	return srv
}

// Integration tests

func TestEvaluateUnwatchedMovie(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	// Add a movie that's 100 days old and never watched
	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "movie1",
		MediaType: models.MediaTypeMovie,
		Title:     "Old Unwatched Movie",
		Year:      2024,
		AddedAt:   now.AddDate(0, 0, -100),
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Reason == "" {
		t.Error("expected non-empty reason")
	}
	if !strings.Contains(results[0].Reason, "Never watched") {
		t.Errorf("expected 'Never watched' in reason, got %q", results[0].Reason)
	}
}

func TestEvaluateUnwatchedMovieRecentNotFlagged(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	// Add a movie that's only 10 days old (should not be flagged with 30 day threshold)
	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "movie1",
		MediaType: models.MediaTypeMovie,
		Title:     "Recent Movie",
		Year:      2024,
		AddedAt:   now.AddDate(0, 0, -10),
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (movie is too recent)", len(results))
	}
}

func TestEvaluateMovieWatchedLongAgoFlagged(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "movie1",
		MediaType: models.MediaTypeMovie,
		Title:     "Old Movie",
		Year:      2020,
		AddedAt:   now.AddDate(-2, 0, 0),
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Movie was watched 60 days ago
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: srv.ID, UserName: "alice", MediaType: models.MediaTypeMovie,
		ItemID: "movie1", Title: "Old Movie",
		StartedAt: now.AddDate(0, 0, -60), StoppedAt: now.AddDate(0, 0, -60).Add(2 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (watched 60 days ago, threshold 30)", len(results))
	}
	if !strings.Contains(results[0].Reason, "Not watched in") {
		t.Errorf("expected 'Not watched in' reason, got %q", results[0].Reason)
	}
}

func TestEvaluateMovieWatchedRecentlyNotFlagged(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "movie1",
		MediaType: models.MediaTypeMovie,
		Title:     "Old Movie",
		Year:      2020,
		AddedAt:   now.AddDate(-2, 0, 0),
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Movie was watched 5 days ago — should NOT be flagged with 30-day threshold
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: srv.ID, UserName: "alice", MediaType: models.MediaTypeMovie,
		ItemID: "movie1", Title: "Old Movie",
		StartedAt: now.AddDate(0, 0, -5), StoppedAt: now.AddDate(0, 0, -5).Add(2 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (movie watched 5 days ago, threshold 30)", len(results))
	}
}

func TestEvaluateUnwatchedTVNoneNeverWatched(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "show1",
		MediaType: models.MediaTypeTV,
		Title:     "Old Show",
		AddedAt:   now.AddDate(0, 0, -100),
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionUnwatchedTVNone,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !strings.Contains(results[0].Reason, "No episodes watched") {
		t.Errorf("expected 'No episodes watched' in reason, got %q", results[0].Reason)
	}
}

func TestEvaluateUnwatchedTVNoneRecentNotFlagged(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "show1",
		MediaType: models.MediaTypeTV,
		Title:     "New Show",
		AddedAt:   now.AddDate(0, 0, -10),
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionUnwatchedTVNone,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (show added 10 days ago, threshold 30)", len(results))
	}
}

func TestEvaluateUnwatchedTVNoneWatchedSkipped(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "show1",
		MediaType: models.MediaTypeTV,
		Title:     "Watched Show",
		AddedAt:   now.AddDate(-2, 0, 0),
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Show was watched 400 days ago — should NOT be flagged by TVNone (it has watch history)
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: srv.ID, UserName: "alice", MediaType: models.MediaTypeTV,
		ItemID: "ep1", GrandparentItemID: "show1", Title: "Episode 1",
		StartedAt: now.AddDate(0, 0, -400), StoppedAt: now.AddDate(0, 0, -400).Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionUnwatchedTVNone,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (show has watch history, TVNone should skip)", len(results))
	}
}

func TestEvaluateTVLowWatchedLongAgoFlagged(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:     srv.ID,
		LibraryID:    "lib1",
		ItemID:       "show1",
		MediaType:    models.MediaTypeTV,
		Title:        "Stale Show",
		EpisodeCount: 20,
		AddedAt:      now.AddDate(-2, 0, 0),
		SyncedAt:     now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Watched 1 episode 60 days ago (5% < 10% threshold, last watched > 30 days)
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: srv.ID, UserName: "alice", MediaType: models.MediaTypeTV,
		ItemID: "ep1", GrandparentItemID: "show1", Title: "Episode 1",
		StartedAt: now.AddDate(0, 0, -60), StoppedAt: now.AddDate(0, 0, -60).Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionUnwatchedTVLow,
		Parameters:    json.RawMessage(`{"days": 30, "max_percent": 10}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !strings.Contains(results[0].Reason, "last watched") {
		t.Errorf("expected 'last watched' in reason, got %q", results[0].Reason)
	}
}

func TestEvaluateTVLowWatchedRecentlyNotFlagged(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:     srv.ID,
		LibraryID:    "lib1",
		ItemID:       "show1",
		MediaType:    models.MediaTypeTV,
		Title:        "Active Show",
		EpisodeCount: 20,
		AddedAt:      now.AddDate(-2, 0, 0),
		SyncedAt:     now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Watched 1 episode 5 days ago (5% < 10% threshold, but last watched < 30 days)
	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID: srv.ID, UserName: "alice", MediaType: models.MediaTypeTV,
		ItemID: "ep1", GrandparentItemID: "show1", Title: "Episode 1",
		StartedAt: now.AddDate(0, 0, -5), StoppedAt: now.AddDate(0, 0, -5).Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionUnwatchedTVLow,
		Parameters:    json.RawMessage(`{"days": 30, "max_percent": 10}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (show watched 5 days ago, threshold 30)", len(results))
	}
}

func TestEvaluateLowResolution(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "movie1", MediaType: models.MediaTypeMovie, Title: "SD Movie", VideoResolution: "480p", AddedAt: now, SyncedAt: now},
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "movie2", MediaType: models.MediaTypeMovie, Title: "HD Movie", VideoResolution: "1080p", AddedAt: now, SyncedAt: now},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionLowResolution,
		Parameters:    json.RawMessage(`{"max_height": 720}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (only 480p)", len(results))
	}
}

func TestEvaluateLargeFiles(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "movie1", MediaType: models.MediaTypeMovie, Title: "Small Movie", FileSize: 1 * 1024 * 1024 * 1024, AddedAt: now, SyncedAt: now},
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "movie2", MediaType: models.MediaTypeMovie, Title: "Large Movie", FileSize: 50 * 1024 * 1024 * 1024, AddedAt: now, SyncedAt: now},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionLargeFiles,
		Parameters:    json.RawMessage(`{"min_size_gb": 10}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (only 50GB movie)", len(results))
	}
}

func TestEvaluateRuleUnknownCriterion(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	rule := &models.MaintenanceRule{
		ServerID:      1,
		LibraryID:     "lib1",
		CriterionType: "unknown_criterion",
		Parameters:    json.RawMessage(`{}`),
	}

	e := NewEvaluator(s)
	_, err := e.EvaluateRule(ctx, rule)
	if err == nil {
		t.Error("expected error for unknown criterion")
	}
}

func TestToBatch(t *testing.T) {
	candidates := []CandidateResult{
		{LibraryItemID: 1, Reason: "Reason 1"},
		{LibraryItemID: 2, Reason: "Reason 2"},
	}

	batch := ToBatch(candidates)
	if len(batch) != 2 {
		t.Fatalf("got %d batch items, want 2", len(batch))
	}
	if batch[0].LibraryItemID != 1 {
		t.Errorf("batch[0].LibraryItemID = %d, want 1", batch[0].LibraryItemID)
	}
	if batch[1].Reason != "Reason 2" {
		t.Errorf("batch[1].Reason = %q, want %q", batch[1].Reason, "Reason 2")
	}
}

func TestToBatchEmpty(t *testing.T) {
	batch := ToBatch(nil)
	if len(batch) != 0 {
		t.Errorf("got %d batch items, want 0", len(batch))
	}
}

func TestEvaluateLowResolutionDefaultParams(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "movie1", MediaType: models.MediaTypeMovie, Title: "SD Movie", VideoResolution: "480p", AddedAt: now, SyncedAt: now},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Use empty parameters - should use default max_height of 720
	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionLowResolution,
		Parameters:    json.RawMessage(`{}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (480p should be flagged with default 720p threshold)", len(results))
	}
}

func TestEvaluateLargeFilesDefaultParams(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "movie1", MediaType: models.MediaTypeMovie, Title: "Large Movie", FileSize: 15 * 1024 * 1024 * 1024, AddedAt: now, SyncedAt: now},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Use empty parameters - should use default min_size_gb of 10
	rule := &models.MaintenanceRule{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		CriterionType: models.CriterionLargeFiles,
		Parameters:    json.RawMessage(`{}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (15GB should be flagged with default 10GB threshold)", len(results))
	}
}
