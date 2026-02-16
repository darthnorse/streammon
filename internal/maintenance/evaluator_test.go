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
	if DefaultDays != 365 {
		t.Errorf("DefaultDays = %d, want 365", DefaultDays)
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

func libs(serverID int64, libraryID string) []models.RuleLibrary {
	return []models.RuleLibrary{{ServerID: serverID, LibraryID: libraryID}}
}

// Integration tests

func TestEvaluateUnwatchedMovie(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

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
		Libraries:     libs(srv.ID, "lib1"),
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
		Libraries:     libs(srv.ID, "lib1"),
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
	watchedAt := now.AddDate(0, 0, -60)
	items := []models.LibraryItemCache{{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		ItemID:        "movie1",
		MediaType:     models.MediaTypeMovie,
		Title:         "Old Movie",
		Year:          2020,
		AddedAt:       now.AddDate(-2, 0, 0),
		LastWatchedAt: &watchedAt,
		SyncedAt:      now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
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
	watchedAt := now.AddDate(0, 0, -5)
	items := []models.LibraryItemCache{{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		ItemID:        "movie1",
		MediaType:     models.MediaTypeMovie,
		Title:         "Old Movie",
		Year:          2020,
		AddedAt:       now.AddDate(-2, 0, 0),
		LastWatchedAt: &watchedAt,
		SyncedAt:      now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
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
		Libraries:     libs(srv.ID, "lib1"),
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
	if !strings.Contains(results[0].Reason, "Never watched") {
		t.Errorf("expected 'Never watched' in reason, got %q", results[0].Reason)
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
		Libraries:     libs(srv.ID, "lib1"),
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

func TestEvaluateUnwatchedTVNoneWatchedLongAgoFlagged(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	watchedAt := now.AddDate(0, 0, -400)
	items := []models.LibraryItemCache{{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		ItemID:        "show1",
		MediaType:     models.MediaTypeTV,
		Title:         "Watched Show",
		AddedAt:       now.AddDate(-2, 0, 0),
		LastWatchedAt: &watchedAt,
		SyncedAt:      now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionUnwatchedTVNone,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (last activity 400 days ago, threshold 30)", len(results))
	}
	if !strings.Contains(results[0].Reason, "Last watched") {
		t.Errorf("expected 'Last watched' in reason, got %q", results[0].Reason)
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
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionLowResolution,
		MediaType:     models.MediaTypeMovie,
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
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionLargeFiles,
		MediaType:     models.MediaTypeMovie,
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
		Libraries:     libs(1, "lib1"),
		CriterionType: "unknown_criterion",
		Parameters:    json.RawMessage(`{}`),
	}

	e := NewEvaluator(s)
	_, err := e.EvaluateRule(ctx, rule)
	if err == nil {
		t.Error("expected error for unknown criterion")
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

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionLowResolution,
		MediaType:     models.MediaTypeMovie,
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

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionLargeFiles,
		MediaType:     models.MediaTypeMovie,
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

// New multi-library and cross-server tests

func TestEvaluateMultiLibraryRule(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "movie1", MediaType: models.MediaTypeMovie, Title: "Movie in Lib1", Year: 2020, AddedAt: now.AddDate(0, 0, -100), SyncedAt: now},
		{ServerID: srv.ID, LibraryID: "lib2", ItemID: "movie2", MediaType: models.MediaTypeMovie, Title: "Movie in Lib2", Year: 2021, AddedAt: now.AddDate(0, 0, -90), SyncedAt: now},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		Libraries: []models.RuleLibrary{
			{ServerID: srv.ID, LibraryID: "lib1"},
			{ServerID: srv.ID, LibraryID: "lib2"},
		},
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (items from both libraries)", len(results))
	}
}

func TestEvaluateCrossServerWatch(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	// Create two servers
	srvA := &models.Server{Name: "Server A", Type: models.ServerTypePlex, URL: "http://a", APIKey: "keyA", Enabled: true}
	if err := s.CreateServer(srvA); err != nil {
		t.Fatal(err)
	}
	srvB := &models.Server{Name: "Server B", Type: models.ServerTypePlex, URL: "http://b", APIKey: "keyB", Enabled: true}
	if err := s.CreateServer(srvB); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	watchedRecently := now.AddDate(0, 0, -5)

	// Same movie (same TMDB ID) on both servers.
	// Server A: watched 5 days ago. Server B: never watched locally.
	itemsA := []models.LibraryItemCache{{
		ServerID:      srvA.ID,
		LibraryID:     "lib1",
		ItemID:        "movieA",
		MediaType:     models.MediaTypeMovie,
		Title:         "Shared Movie",
		Year:          2020,
		TMDBID:        "tmdb123",
		AddedAt:       now.AddDate(-1, 0, 0),
		LastWatchedAt: &watchedRecently,
		SyncedAt:      now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, itemsA); err != nil {
		t.Fatal(err)
	}

	itemsB := []models.LibraryItemCache{{
		ServerID:  srvB.ID,
		LibraryID: "lib2",
		ItemID:    "movieB",
		MediaType: models.MediaTypeMovie,
		Title:     "Shared Movie",
		Year:      2020,
		TMDBID:    "tmdb123",
		AddedAt:   now.AddDate(-1, 0, 0),
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, itemsB); err != nil {
		t.Fatal(err)
	}

	// Rule targets Server B's library only
	rule := &models.MaintenanceRule{
		Libraries:     libs(srvB.ID, "lib2"),
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	// Item on Server B should NOT be flagged because the cross-server watch from
	// Server A (5 days ago) is within the 30-day threshold
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (cross-server watch should prevent flagging)", len(results))
	}
}

func TestEvaluateCrossServerWatchNoExternalIDs(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srvA := &models.Server{Name: "Server A", Type: models.ServerTypePlex, URL: "http://a", APIKey: "keyA", Enabled: true}
	if err := s.CreateServer(srvA); err != nil {
		t.Fatal(err)
	}
	srvB := &models.Server{Name: "Server B", Type: models.ServerTypePlex, URL: "http://b", APIKey: "keyB", Enabled: true}
	if err := s.CreateServer(srvB); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	watchedRecently := now.AddDate(0, 0, -5)

	// Same title on both servers but NO external IDs -- no cross-server matching.
	// Server A: watched recently
	itemsA := []models.LibraryItemCache{{
		ServerID:      srvA.ID,
		LibraryID:     "lib1",
		ItemID:        "movieA",
		MediaType:     models.MediaTypeMovie,
		Title:         "No External IDs Movie",
		Year:          2020,
		AddedAt:       now.AddDate(-1, 0, 0),
		LastWatchedAt: &watchedRecently,
		SyncedAt:      now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, itemsA); err != nil {
		t.Fatal(err)
	}

	// Server B: never watched, no external IDs, added 100 days ago
	itemsB := []models.LibraryItemCache{{
		ServerID:  srvB.ID,
		LibraryID: "lib2",
		ItemID:    "movieB",
		MediaType: models.MediaTypeMovie,
		Title:     "No External IDs Movie",
		Year:      2020,
		AddedAt:   now.AddDate(0, 0, -100),
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, itemsB); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		Libraries:     libs(srvB.ID, "lib2"),
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	// Without external IDs, cross-server matching cannot work.
	// Item on Server B should be flagged (never watched, added 100 days ago).
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (no external IDs means no cross-server match)", len(results))
	}
	if !strings.Contains(results[0].Reason, "Never watched") {
		t.Errorf("expected 'Never watched' in reason, got %q", results[0].Reason)
	}
}

func TestEvaluateLowResolutionMultiLibrary(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "movie1", MediaType: models.MediaTypeMovie, Title: "SD Movie Lib1", VideoResolution: "480p", AddedAt: now, SyncedAt: now},
		{ServerID: srv.ID, LibraryID: "lib2", ItemID: "movie2", MediaType: models.MediaTypeMovie, Title: "SD Movie Lib2", VideoResolution: "576p", AddedAt: now, SyncedAt: now},
		{ServerID: srv.ID, LibraryID: "lib2", ItemID: "movie3", MediaType: models.MediaTypeMovie, Title: "HD Movie Lib2", VideoResolution: "1080p", AddedAt: now, SyncedAt: now},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		Libraries: []models.RuleLibrary{
			{ServerID: srv.ID, LibraryID: "lib1"},
			{ServerID: srv.ID, LibraryID: "lib2"},
		},
		CriterionType: models.CriterionLowResolution,
		MediaType:     models.MediaTypeMovie,
		Parameters:    json.RawMessage(`{"max_height": 720}`),
	}

	e := NewEvaluator(s)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	// 480p from lib1 and 576p from lib2 should be flagged; 1080p should not
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (480p + 576p across two libraries)", len(results))
	}
}
