package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"streammon/internal/media"
	"streammon/internal/mediautil"
	"streammon/internal/models"
	"streammon/internal/store"
	"streammon/internal/tmdb"
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

func seedTwoTestServers(t *testing.T, s *store.Store) (*models.Server, *models.Server) {
	t.Helper()
	srvA := &models.Server{Name: "Server A", Type: models.ServerTypePlex, URL: "http://a", APIKey: "keyA", Enabled: true}
	if err := s.CreateServer(srvA); err != nil {
		t.Fatal(err)
	}
	srvB := &models.Server{Name: "Server B", Type: models.ServerTypePlex, URL: "http://b", APIKey: "keyB", Enabled: true}
	if err := s.CreateServer(srvB); err != nil {
		t.Fatal(err)
	}
	return srvA, srvB
}

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

	e := NewEvaluator(s, nil, nil)
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

	e := NewEvaluator(s, nil, nil)
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

	e := NewEvaluator(s, nil, nil)
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

	e := NewEvaluator(s, nil, nil)
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

	e := NewEvaluator(s, nil, nil)
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

	e := NewEvaluator(s, nil, nil)
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

	e := NewEvaluator(s, nil, nil)
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

	e := NewEvaluator(s, nil, nil)
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

	e := NewEvaluator(s, nil, nil)
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

	e := NewEvaluator(s, nil, nil)
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

	e := NewEvaluator(s, nil, nil)
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

	e := NewEvaluator(s, nil, nil)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (15GB should be flagged with default 10GB threshold)", len(results))
	}
}

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

	e := NewEvaluator(s, nil, nil)
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
	srvA, srvB := seedTwoTestServers(t, s)

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

	e := NewEvaluator(s, nil, nil)
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
	srvA, srvB := seedTwoTestServers(t, s)

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

	e := NewEvaluator(s, nil, nil)
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

func TestEvaluateUnwatchedExcludesItemWithStreamMonHistory(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	// TV show added 200 days ago, media server reports LastWatchedAt=nil
	// (API user never watched it, but another household member did today)
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "show1",
		MediaType: models.MediaTypeTV,
		Title:     "Real Housewives",
		Year:      2020,
		AddedAt:   now.AddDate(0, 0, -200),
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Simulate a watch session from another user captured by StreamMon's poller
	recentWatch := now.Add(-2 * time.Hour)
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID:          srv.ID,
		ItemID:            "ep-42",
		GrandparentItemID: "show1",
		UserName:          "familymember",
		MediaType:         models.MediaTypeTV,
		Title:             "S05E01",
		GrandparentTitle:  "Real Housewives",
		StartedAt:         recentWatch.Add(-time.Hour),
		StoppedAt:         recentWatch,
	})

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionUnwatchedTVNone,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s, nil, nil)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (StreamMon history from today should prevent flagging); reason=%q",
			len(results), results[0].Reason)
	}
}

func TestEvaluateUnwatchedMovieExcludesItemWithStreamMonHistory(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	// Movie added 200 days ago, media server reports LastWatchedAt=nil
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "movie1",
		MediaType: models.MediaTypeMovie,
		Title:     "Family Movie",
		Year:      2022,
		AddedAt:   now.AddDate(0, 0, -200),
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Another user watched it recently — direct item_id match for movies
	recentWatch := now.Add(-3 * time.Hour)
	s.InsertHistory(&models.WatchHistoryEntry{
		ServerID:  srv.ID,
		ItemID:    "movie1",
		UserName:  "kid",
		MediaType: models.MediaTypeMovie,
		Title:     "Family Movie",
		StartedAt: recentWatch.Add(-2 * time.Hour),
		StoppedAt: recentWatch,
	})

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s, nil, nil)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (StreamMon history should prevent flagging); reason=%q",
			len(results), results[0].Reason)
	}
}

func TestExternalIDKeys(t *testing.T) {
	tests := []struct {
		name string
		item models.LibraryItemCache
		want []string
	}{
		{"tmdb only", models.LibraryItemCache{TMDBID: "123"}, []string{"tmdb:123"}},
		{"imdb only", models.LibraryItemCache{IMDBID: "tt999"}, []string{"imdb:tt999"}},
		{"tvdb only", models.LibraryItemCache{TVDBID: "456"}, []string{"tvdb:456"}},
		{"tmdb and imdb", models.LibraryItemCache{TMDBID: "1", IMDBID: "tt2"}, []string{"tmdb:1", "imdb:tt2"}},
		{"all three", models.LibraryItemCache{TMDBID: "1", IMDBID: "tt2", TVDBID: "3"}, []string{"tmdb:1", "imdb:tt2", "tvdb:3"}},
		{"no external IDs", models.LibraryItemCache{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := externalIDKeys(&tt.item)
			if len(got) != len(tt.want) {
				t.Fatalf("externalIDKeys() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("externalIDKeys()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDeduplicateCandidates(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result := deduplicateCandidates(nil, nil)
		if len(result) != 0 {
			t.Errorf("got %d, want 0", len(result))
		}
	})

	t.Run("all unique external IDs", func(t *testing.T) {
		items := []models.LibraryItemCache{
			{ID: 1, TMDBID: "100"},
			{ID: 2, TMDBID: "200"},
		}
		candidates := []models.BatchCandidate{
			{LibraryItemID: 1, Reason: "r1"},
			{LibraryItemID: 2, Reason: "r2"},
		}
		result := deduplicateCandidates(candidates, items)
		if len(result) != 2 {
			t.Errorf("got %d, want 2", len(result))
		}
	})

	t.Run("duplicate TMDB ID keeps first", func(t *testing.T) {
		items := []models.LibraryItemCache{
			{ID: 1, TMDBID: "100", ServerID: 1},
			{ID: 2, TMDBID: "100", ServerID: 2},
		}
		candidates := []models.BatchCandidate{
			{LibraryItemID: 1, Reason: "server1"},
			{LibraryItemID: 2, Reason: "server2"},
		}
		result := deduplicateCandidates(candidates, items)
		if len(result) != 1 {
			t.Fatalf("got %d, want 1", len(result))
		}
		if result[0].LibraryItemID != 1 {
			t.Errorf("kept item %d, want 1 (first)", result[0].LibraryItemID)
		}
	})

	t.Run("items without external IDs not deduped", func(t *testing.T) {
		items := []models.LibraryItemCache{
			{ID: 1},
			{ID: 2},
		}
		candidates := []models.BatchCandidate{
			{LibraryItemID: 1, Reason: "r1"},
			{LibraryItemID: 2, Reason: "r2"},
		}
		result := deduplicateCandidates(candidates, items)
		if len(result) != 2 {
			t.Errorf("got %d, want 2 (no external IDs = no dedup)", len(result))
		}
	})

	t.Run("mixed scenario", func(t *testing.T) {
		items := []models.LibraryItemCache{
			{ID: 1, TMDBID: "100"},         // dup group A
			{ID: 2, TMDBID: "100"},         // dup group A (removed)
			{ID: 3, IMDBID: "tt200"},       // unique
			{ID: 4},                         // no external ID
			{ID: 5, TVDBID: "300"},         // dup group B
			{ID: 6, TVDBID: "300"},         // dup group B (removed)
		}
		candidates := []models.BatchCandidate{
			{LibraryItemID: 1}, {LibraryItemID: 2}, {LibraryItemID: 3},
			{LibraryItemID: 4}, {LibraryItemID: 5}, {LibraryItemID: 6},
		}
		result := deduplicateCandidates(candidates, items)
		// Expected: item 1 (tmdb:100), item 3 (imdb:tt200), item 4 (no ID), item 5 (tvdb:300) = 4
		if len(result) != 4 {
			t.Errorf("got %d, want 4", len(result))
		}
	})

	t.Run("mixed external ID types deduped via shared IMDB", func(t *testing.T) {
		// Item A has TMDB + IMDB, item B only has IMDB. They share IMDB so should dedup.
		items := []models.LibraryItemCache{
			{ID: 1, TMDBID: "100", IMDBID: "tt999"},
			{ID: 2, IMDBID: "tt999"},
		}
		candidates := []models.BatchCandidate{
			{LibraryItemID: 1, Reason: "r1"},
			{LibraryItemID: 2, Reason: "r2"},
		}
		result := deduplicateCandidates(candidates, items)
		if len(result) != 1 {
			t.Fatalf("got %d, want 1 (shared IMDB should dedup)", len(result))
		}
		if result[0].LibraryItemID != 1 {
			t.Errorf("kept item %d, want 1 (first)", result[0].LibraryItemID)
		}
	})

	t.Run("candidate with missing item not deduped", func(t *testing.T) {
		candidates := []models.BatchCandidate{
			{LibraryItemID: 999, Reason: "orphan"},
		}
		result := deduplicateCandidates(candidates, nil)
		if len(result) != 1 {
			t.Errorf("got %d, want 1", len(result))
		}
	})
}

func TestEvaluateRuleDeduplicatesCrossServer(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srvA, srvB := seedTwoTestServers(t, s)

	now := time.Now().UTC()
	// Same movie on both servers, same TMDB ID, both old enough to be flagged
	itemsA := []models.LibraryItemCache{{
		ServerID: srvA.ID, LibraryID: "lib1", ItemID: "movieA",
		MediaType: models.MediaTypeMovie, Title: "Inception", Year: 2010,
		TMDBID: "27205", AddedAt: now.AddDate(0, 0, -100), SyncedAt: now,
	}}
	itemsB := []models.LibraryItemCache{{
		ServerID: srvB.ID, LibraryID: "lib2", ItemID: "movieB",
		MediaType: models.MediaTypeMovie, Title: "Inception", Year: 2010,
		TMDBID: "27205", AddedAt: now.AddDate(0, 0, -100), SyncedAt: now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, itemsA); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertLibraryItems(ctx, itemsB); err != nil {
		t.Fatal(err)
	}

	rule := &models.MaintenanceRule{
		Libraries: []models.RuleLibrary{
			{ServerID: srvA.ID, LibraryID: "lib1"},
			{ServerID: srvB.ID, LibraryID: "lib2"},
		},
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 30}`),
	}

	e := NewEvaluator(s, nil, nil)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("got %d results, want 1 (same TMDB ID should dedup)", len(results))
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

	e := NewEvaluator(s, nil, nil)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	// 480p from lib1 and 576p from lib2 should be flagged; 1080p should not
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (480p + 576p across two libraries)", len(results))
	}
}

type mockMediaServer struct {
	seasons map[string][]models.Season
}

func (m *mockMediaServer) Name() string                        { return "mock" }
func (m *mockMediaServer) Type() models.ServerType             { return models.ServerTypePlex }
func (m *mockMediaServer) ServerID() int64                     { return 0 }
func (m *mockMediaServer) GetSessions(ctx context.Context) ([]models.ActiveStream, error) {
	return nil, nil
}
func (m *mockMediaServer) TestConnection(ctx context.Context) error { return nil }
func (m *mockMediaServer) GetRecentlyAdded(ctx context.Context, limit int) ([]models.LibraryItem, error) {
	return nil, nil
}
func (m *mockMediaServer) GetItemDetails(ctx context.Context, itemID string) (*models.ItemDetails, error) {
	return nil, nil
}
func (m *mockMediaServer) GetLibraries(ctx context.Context) ([]models.Library, error) {
	return nil, nil
}
func (m *mockMediaServer) GetUsers(ctx context.Context) ([]models.MediaUser, error) {
	return nil, nil
}
func (m *mockMediaServer) GetLibraryItems(ctx context.Context, libraryID string) ([]models.LibraryItemCache, error) {
	return nil, nil
}
func (m *mockMediaServer) DeleteItem(ctx context.Context, itemID string) error { return nil }
func (m *mockMediaServer) GetSeasons(ctx context.Context, showID string) ([]models.Season, error) {
	return m.seasons[showID], nil
}

type mockServerResolver struct {
	servers map[int64]media.MediaServer
}

func (m *mockServerResolver) GetServer(id int64) (media.MediaServer, bool) {
	ms, ok := m.servers[id]
	return ms, ok
}

func TestEvaluateKeepLatestSeasons(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "show1",
		MediaType: models.MediaTypeTV,
		Title:     "Long Running Show",
		AddedAt:   now,
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	ms := &mockMediaServer{
		seasons: map[string][]models.Season{
			"show1": {
				{ID: "s0", Number: 0, Title: "Specials"},
				{ID: "s1", Number: 1, Title: "Season 1"},
				{ID: "s2", Number: 2, Title: "Season 2"},
				{ID: "s3", Number: 3, Title: "Season 3"},
				{ID: "s4", Number: 4, Title: "Season 4"},
			},
		},
	}
	resolver := &mockServerResolver{
		servers: map[int64]media.MediaServer{srv.ID: ms},
	}

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionKeepLatestSeasons,
		Parameters:    json.RawMessage(`{"keep_seasons": 2}`),
	}

	e := NewEvaluator(s, nil, resolver)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !strings.Contains(results[0].Reason, "4 seasons") {
		t.Errorf("expected '4 seasons' in reason, got %q", results[0].Reason)
	}
	if !strings.Contains(results[0].Reason, "keeping latest 2") {
		t.Errorf("expected 'keeping latest 2' in reason, got %q", results[0].Reason)
	}
}

func TestEvaluateKeepLatestSeasonsNotEnoughSeasons(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "show1",
		MediaType: models.MediaTypeTV,
		Title:     "Short Show",
		AddedAt:   now,
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	ms := &mockMediaServer{
		seasons: map[string][]models.Season{
			"show1": {
				{ID: "s1", Number: 1, Title: "Season 1"},
				{ID: "s2", Number: 2, Title: "Season 2"},
			},
		},
	}
	resolver := &mockServerResolver{
		servers: map[int64]media.MediaServer{srv.ID: ms},
	}

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionKeepLatestSeasons,
		Parameters:    json.RawMessage(`{"keep_seasons": 3}`),
	}

	e := NewEvaluator(s, nil, resolver)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (only 2 seasons, threshold 3)", len(results))
	}
}

func TestEvaluateKeepLatestSeasonsIgnoresSpecials(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "show1",
		MediaType: models.MediaTypeTV,
		Title:     "Show With Specials",
		AddedAt:   now,
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	ms := &mockMediaServer{
		seasons: map[string][]models.Season{
			"show1": {
				{ID: "s0", Number: 0, Title: "Specials"},
				{ID: "s1", Number: 1, Title: "Season 1"},
				{ID: "s2", Number: 2, Title: "Season 2"},
				{ID: "s3", Number: 3, Title: "Season 3"},
			},
		},
	}
	resolver := &mockServerResolver{
		servers: map[int64]media.MediaServer{srv.ID: ms},
	}

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionKeepLatestSeasons,
		Parameters:    json.RawMessage(`{"keep_seasons": 3}`),
	}

	e := NewEvaluator(s, nil, resolver)
	results, err := e.EvaluateRule(ctx, rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (3 regular seasons + specials, threshold 3)", len(results))
	}
}

// setupKeepLatestSeasonsGenreTest creates a common test fixture for genre filter tests.
func setupKeepLatestSeasonsGenreTest(t *testing.T, tmdbID string) (*store.Store, *models.Server, *mockServerResolver) {
	t.Helper()
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{{
		ServerID:  srv.ID,
		LibraryID: "lib1",
		ItemID:    "show1",
		MediaType: models.MediaTypeTV,
		Title:     "Genre Test Show",
		TMDBID:    tmdbID,
		AddedAt:   now,
		SyncedAt:  now,
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	ms := &mockMediaServer{
		seasons: map[string][]models.Season{
			"show1": {
				{ID: "s1", Number: 1, Title: "Season 1"},
				{ID: "s2", Number: 2, Title: "Season 2"},
				{ID: "s3", Number: 3, Title: "Season 3"},
				{ID: "s4", Number: 4, Title: "Season 4"},
			},
		},
	}
	resolver := &mockServerResolver{
		servers: map[int64]media.MediaServer{srv.ID: ms},
	}
	return s, srv, resolver
}

func TestEvaluateKeepLatestSeasonsGenreMatch(t *testing.T) {
	s, srv, resolver := setupKeepLatestSeasonsGenreTest(t, "100")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":100,"genres":[{"id":10767,"name":"Talk"},{"id":18,"name":"Drama"}]}`)
	}))
	defer ts.Close()

	tmdbClient := tmdb.NewWithBaseURL("", nil, ts.URL)

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionKeepLatestSeasons,
		Parameters:    json.RawMessage(`{"keep_seasons": 2, "genre_ids": [10767]}`),
	}

	e := NewEvaluator(s, tmdbClient, resolver)
	results, err := e.EvaluateRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (genre matches)", len(results))
	}
}

func TestEvaluateKeepLatestSeasonsGenreNoMatch(t *testing.T) {
	s, srv, resolver := setupKeepLatestSeasonsGenreTest(t, "100")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":100,"genres":[{"id":18,"name":"Drama"}]}`)
	}))
	defer ts.Close()

	tmdbClient := tmdb.NewWithBaseURL("", nil, ts.URL)

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionKeepLatestSeasons,
		Parameters:    json.RawMessage(`{"keep_seasons": 2, "genre_ids": [10767]}`),
	}

	e := NewEvaluator(s, tmdbClient, resolver)
	results, err := e.EvaluateRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (genre does not match)", len(results))
	}
}

func TestEvaluateKeepLatestSeasonsGenreTMDBFailure(t *testing.T) {
	s, srv, resolver := setupKeepLatestSeasonsGenreTest(t, "100")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	tmdbClient := tmdb.NewWithBaseURL("", nil, ts.URL)

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionKeepLatestSeasons,
		Parameters:    json.RawMessage(`{"keep_seasons": 2, "genre_ids": [10767]}`),
	}

	e := NewEvaluator(s, tmdbClient, resolver)
	results, err := e.EvaluateRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (TMDB failure should skip item, not include it)", len(results))
	}
}

func TestEvaluateKeepLatestSeasonsGenreNoTMDBID(t *testing.T) {
	s, srv, resolver := setupKeepLatestSeasonsGenreTest(t, "") // empty TMDB ID

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionKeepLatestSeasons,
		Parameters:    json.RawMessage(`{"keep_seasons": 2, "genre_ids": [10767]}`),
	}

	e := NewEvaluator(s, nil, resolver)
	results, err := e.EvaluateRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 (no TMDB ID should skip when genre filter active)", len(results))
	}
}

func TestEvaluateKeepLatestSeasonsProgress(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	srv := seedTestServer(t, s)

	now := time.Now().UTC()
	items := []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "show1", MediaType: models.MediaTypeTV, Title: "Show A", AddedAt: now, SyncedAt: now},
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "show2", MediaType: models.MediaTypeTV, Title: "Show B", AddedAt: now, SyncedAt: now},
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "movie1", MediaType: models.MediaTypeMovie, Title: "A Movie", AddedAt: now, SyncedAt: now},
	}
	if _, err := s.UpsertLibraryItems(context.Background(), items); err != nil {
		t.Fatal(err)
	}

	ms := &mockMediaServer{
		seasons: map[string][]models.Season{
			"show1": {{ID: "s1", Number: 1}, {ID: "s2", Number: 2}},
			"show2": {{ID: "s1", Number: 1}, {ID: "s2", Number: 2}},
		},
	}
	resolver := &mockServerResolver{servers: map[int64]media.MediaServer{srv.ID: ms}}

	progressCtx, progressCh := mediautil.ContextWithProgress(context.Background())

	rule := &models.MaintenanceRule{
		Libraries:     libs(srv.ID, "lib1"),
		CriterionType: models.CriterionKeepLatestSeasons,
		Parameters:    json.RawMessage(`{"keep_seasons": 3}`),
	}

	e := NewEvaluator(s, nil, resolver)

	var msgs []mediautil.SyncProgress
	done := make(chan struct{})
	go func() {
		for p := range progressCh {
			msgs = append(msgs, p)
		}
		close(done)
	}()

	_, err := e.EvaluateRule(progressCtx, rule)
	mediautil.CloseProgress(progressCtx)
	<-done

	if err != nil {
		t.Fatalf("EvaluateRule: %v", err)
	}

	// Should have 2 progress messages (one per TV item; movie is skipped)
	if len(msgs) != 2 {
		t.Fatalf("got %d progress messages, want 2", len(msgs))
	}

	for i, msg := range msgs {
		if msg.Phase != mediautil.PhaseEvaluating {
			t.Errorf("msg[%d].Phase = %q, want %q", i, msg.Phase, mediautil.PhaseEvaluating)
		}
		if msg.Total != 2 {
			t.Errorf("msg[%d].Total = %d, want 2", i, msg.Total)
		}
		if msg.Current != i+1 {
			t.Errorf("msg[%d].Current = %d, want %d", i, msg.Current, i+1)
		}
		if msg.Library != "lib1" {
			t.Errorf("msg[%d].Library = %q, want %q", i, msg.Library, "lib1")
		}
	}
}
