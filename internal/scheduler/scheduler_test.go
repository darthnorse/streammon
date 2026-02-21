package scheduler

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/poller"
	"streammon/internal/store"
)

// fakeMediaServer implements media.MediaServer for testing
type fakeMediaServer struct {
	id        int64
	name      string
	libraries []models.Library
	items     map[string][]models.LibraryItemCache

	mu        sync.Mutex
	syncOrder *[]string
}

func (f *fakeMediaServer) Name() string                    { return f.name }
func (f *fakeMediaServer) Type() models.ServerType         { return models.ServerTypePlex }
func (f *fakeMediaServer) ServerID() int64                 { return f.id }
func (f *fakeMediaServer) TestConnection(context.Context) error { return nil }
func (f *fakeMediaServer) GetSessions(context.Context) ([]models.ActiveStream, error) {
	return nil, nil
}
func (f *fakeMediaServer) GetRecentlyAdded(context.Context, int) ([]models.LibraryItem, error) {
	return nil, nil
}
func (f *fakeMediaServer) GetItemDetails(context.Context, string) (*models.ItemDetails, error) {
	return nil, nil
}
func (f *fakeMediaServer) GetUsers(context.Context) ([]models.MediaUser, error) {
	return nil, nil
}
func (f *fakeMediaServer) DeleteItem(context.Context, string) error { return nil }
func (f *fakeMediaServer) GetSeasons(context.Context, string) ([]models.Season, error) {
	return nil, nil
}

func (f *fakeMediaServer) GetLibraries(ctx context.Context) ([]models.Library, error) {
	return f.libraries, nil
}

func (f *fakeMediaServer) GetLibraryItems(ctx context.Context, libraryID string) ([]models.LibraryItemCache, error) {
	if f.syncOrder != nil {
		f.mu.Lock()
		*f.syncOrder = append(*f.syncOrder, libraryID)
		f.mu.Unlock()
	}
	items := f.items[libraryID]
	return items, nil
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

func seedServer(t *testing.T, s *store.Store, name string) *models.Server {
	t.Helper()
	srv := &models.Server{
		Name:    name,
		Type:    models.ServerTypePlex,
		URL:     "http://" + name,
		APIKey:  "key",
		Enabled: true,
	}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestDurationUntil3AM(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Duration
	}{
		{
			name: "before 3 AM today",
			now:  time.Date(2024, 1, 15, 2, 0, 0, 0, time.Local),
			want: 1 * time.Hour,
		},
		{
			name: "at 3 AM exactly",
			now:  time.Date(2024, 1, 15, 3, 0, 0, 0, time.Local),
			want: 24 * time.Hour,
		},
		{
			name: "after 3 AM",
			now:  time.Date(2024, 1, 15, 15, 30, 0, 0, time.Local),
			want: 11*time.Hour + 30*time.Minute,
		},
		{
			name: "just before midnight",
			now:  time.Date(2024, 1, 15, 23, 59, 0, 0, time.Local),
			want: 3*time.Hour + 1*time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := durationUntil3AM(tt.now)
			if got != tt.want {
				t.Errorf("durationUntil3AM(%v) = %v, want %v",
					tt.now.Format("15:04"), got, tt.want)
			}
		})
	}
}

func TestSyncAllTwoPhases(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := seedServer(t, s, "test-server")
	now := time.Now().UTC()

	items := []models.LibraryItemCache{
		{
			ServerID:  srv.ID,
			LibraryID: "lib1",
			ItemID:    "item1",
			MediaType: models.MediaTypeMovie,
			Title:     "Old Movie",
			Year:      2020,
			AddedAt:   now.AddDate(0, 0, -200),
			SyncedAt:  now,
		},
	}

	fake := &fakeMediaServer{
		id:   srv.ID,
		name: srv.Name,
		libraries: []models.Library{
			{ID: "lib1", Name: "Movies", Type: models.LibraryTypeMovie},
		},
		items: map[string][]models.LibraryItemCache{
			"lib1": items,
		},
	}

	p := poller.New(s, 5*time.Second)
	p.AddServer(srv.ID, fake)

	sch := New(s, p, nil, WithSyncTimeout(1*time.Minute))

	// Create a maintenance rule targeting lib1
	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name:          "Unwatched Movies",
		CriterionType: models.CriterionUnwatchedMovie,
		MediaType:     models.MediaTypeMovie,
		Parameters:    json.RawMessage(`{"days": 90}`),
		Enabled:       true,
		Libraries:     []models.RuleLibrary{{ServerID: srv.ID, LibraryID: "lib1"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sch.SyncAll(ctx); err != nil {
		t.Fatalf("SyncAll() error: %v", err)
	}

	// Verify items were synced
	count, err := s.CountLibraryItems(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 item, got %d", count)
	}

	// Verify candidates were produced by rule evaluation
	result, err := s.ListCandidatesForRule(ctx, rule.ID, 1, 100, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 candidate, got %d", result.Total)
	}
}

func TestSyncAllLibrariesBeforeRuleEvaluation(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := seedServer(t, s, "test-server")
	now := time.Now().UTC()

	var syncOrder []string

	items1 := []models.LibraryItemCache{
		{
			ServerID:  srv.ID,
			LibraryID: "movies",
			ItemID:    "m1",
			MediaType: models.MediaTypeMovie,
			Title:     "Movie One",
			Year:      2020,
			AddedAt:   now.AddDate(0, 0, -200),
			SyncedAt:  now,
		},
	}

	items2 := []models.LibraryItemCache{
		{
			ServerID:  srv.ID,
			LibraryID: "movies2",
			ItemID:    "m2",
			MediaType: models.MediaTypeMovie,
			Title:     "Movie Two",
			Year:      2019,
			AddedAt:   now.AddDate(0, 0, -300),
			SyncedAt:  now,
		},
	}

	fake := &fakeMediaServer{
		id:   srv.ID,
		name: srv.Name,
		libraries: []models.Library{
			{ID: "movies", Name: "Movies", Type: models.LibraryTypeMovie},
			{ID: "movies2", Name: "Movies 2", Type: models.LibraryTypeMovie},
		},
		items: map[string][]models.LibraryItemCache{
			"movies":  items1,
			"movies2": items2,
		},
		syncOrder: &syncOrder,
	}

	p := poller.New(s, 5*time.Second)
	p.AddServer(srv.ID, fake)

	sch := New(s, p, nil, WithSyncTimeout(1*time.Minute))

	// Create a rule that spans BOTH libraries
	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name:          "Multi-lib Unwatched",
		CriterionType: models.CriterionUnwatchedMovie,
		MediaType:     models.MediaTypeMovie,
		Parameters:    json.RawMessage(`{"days": 90}`),
		Enabled:       true,
		Libraries: []models.RuleLibrary{
			{ServerID: srv.ID, LibraryID: "movies"},
			{ServerID: srv.ID, LibraryID: "movies2"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sch.SyncAll(ctx); err != nil {
		t.Fatalf("SyncAll() error: %v", err)
	}

	// Verify both libraries were synced (phase 1 completes before phase 2)
	if len(syncOrder) != 2 {
		t.Fatalf("expected 2 library syncs, got %d: %v", len(syncOrder), syncOrder)
	}

	// Verify items from both libraries exist
	count1, _ := s.CountLibraryItems(ctx, srv.ID, "movies")
	count2, _ := s.CountLibraryItems(ctx, srv.ID, "movies2")
	if count1 != 1 || count2 != 1 {
		t.Errorf("expected 1 item in each library, got %d and %d", count1, count2)
	}

	// Verify rule evaluation found candidates from both libraries
	result, err := s.ListCandidatesForRule(ctx, rule.ID, 1, 100, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 {
		t.Errorf("expected 2 candidates from both libraries, got %d", result.Total)
	}
}

func TestSyncAllNoRulesDoesNotFail(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := seedServer(t, s, "test-server")
	now := time.Now().UTC()

	fake := &fakeMediaServer{
		id:   srv.ID,
		name: srv.Name,
		libraries: []models.Library{
			{ID: "lib1", Name: "Movies", Type: models.LibraryTypeMovie},
		},
		items: map[string][]models.LibraryItemCache{
			"lib1": {
				{
					ServerID:  srv.ID,
					LibraryID: "lib1",
					ItemID:    "item1",
					MediaType: models.MediaTypeMovie,
					Title:     "Movie",
					Year:      2020,
					AddedAt:   now,
					SyncedAt:  now,
				},
			},
		},
	}

	p := poller.New(s, 5*time.Second)
	p.AddServer(srv.ID, fake)

	sch := New(s, p, nil, WithSyncTimeout(1*time.Minute))

	// SyncAll with no rules should succeed without error
	if err := sch.SyncAll(ctx); err != nil {
		t.Fatalf("SyncAll() error: %v", err)
	}

	// Items should still be synced
	count, _ := s.CountLibraryItems(ctx, srv.ID, "lib1")
	if count != 1 {
		t.Errorf("expected 1 item, got %d", count)
	}
}

func TestSyncAllDisabledRulesSkipped(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := seedServer(t, s, "test-server")
	now := time.Now().UTC()

	fake := &fakeMediaServer{
		id:   srv.ID,
		name: srv.Name,
		libraries: []models.Library{
			{ID: "lib1", Name: "Movies", Type: models.LibraryTypeMovie},
		},
		items: map[string][]models.LibraryItemCache{
			"lib1": {
				{
					ServerID:  srv.ID,
					LibraryID: "lib1",
					ItemID:    "item1",
					MediaType: models.MediaTypeMovie,
					Title:     "Old Movie",
					Year:      2020,
					AddedAt:   now.AddDate(0, 0, -200),
					SyncedAt:  now,
				},
			},
		},
	}

	p := poller.New(s, 5*time.Second)
	p.AddServer(srv.ID, fake)

	sch := New(s, p, nil, WithSyncTimeout(1*time.Minute))

	// Create a disabled rule
	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name:          "Disabled Rule",
		CriterionType: models.CriterionUnwatchedMovie,
		MediaType:     models.MediaTypeMovie,
		Parameters:    json.RawMessage(`{"days": 90}`),
		Enabled:       false,
		Libraries:     []models.RuleLibrary{{ServerID: srv.ID, LibraryID: "lib1"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sch.SyncAll(ctx); err != nil {
		t.Fatalf("SyncAll() error: %v", err)
	}

	// Disabled rules should not produce candidates (ListAllMaintenanceRules only returns enabled)
	result, err := s.ListCandidatesForRule(ctx, rule.ID, 1, 100, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 candidates for disabled rule, got %d", result.Total)
	}
}

func TestSyncAllDisabledServerSkipped(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{
		Name:    "disabled",
		Type:    models.ServerTypePlex,
		URL:     "http://disabled",
		APIKey:  "key",
		Enabled: false,
	}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	// Don't register server in poller - it should be skipped before that matters
	p := poller.New(s, 5*time.Second)

	sch := New(s, p, nil, WithSyncTimeout(1*time.Minute))

	if err := sch.SyncAll(ctx); err != nil {
		t.Fatalf("SyncAll() error: %v", err)
	}
}

func TestSyncAllMultipleServers(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv1 := seedServer(t, s, "server1")
	srv2 := seedServer(t, s, "server2")
	now := time.Now().UTC()

	fake1 := &fakeMediaServer{
		id:   srv1.ID,
		name: srv1.Name,
		libraries: []models.Library{
			{ID: "lib1", Name: "Movies", Type: models.LibraryTypeMovie},
		},
		items: map[string][]models.LibraryItemCache{
			"lib1": {
				{
					ServerID:  srv1.ID,
					LibraryID: "lib1",
					ItemID:    "s1-m1",
					MediaType: models.MediaTypeMovie,
					Title:     "Server1 Movie",
					Year:      2020,
					AddedAt:   now.AddDate(0, 0, -200),
					SyncedAt:  now,
				},
			},
		},
	}

	fake2 := &fakeMediaServer{
		id:   srv2.ID,
		name: srv2.Name,
		libraries: []models.Library{
			{ID: "lib2", Name: "Movies", Type: models.LibraryTypeMovie},
		},
		items: map[string][]models.LibraryItemCache{
			"lib2": {
				{
					ServerID:  srv2.ID,
					LibraryID: "lib2",
					ItemID:    "s2-m1",
					MediaType: models.MediaTypeMovie,
					Title:     "Server2 Movie",
					Year:      2019,
					AddedAt:   now.AddDate(0, 0, -300),
					SyncedAt:  now,
				},
			},
		},
	}

	p := poller.New(s, 5*time.Second)
	p.AddServer(srv1.ID, fake1)
	p.AddServer(srv2.ID, fake2)

	sch := New(s, p, nil, WithSyncTimeout(1*time.Minute))

	// Create a rule spanning both servers
	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name:          "Cross-server Unwatched",
		CriterionType: models.CriterionUnwatchedMovie,
		MediaType:     models.MediaTypeMovie,
		Parameters:    json.RawMessage(`{"days": 90}`),
		Enabled:       true,
		Libraries: []models.RuleLibrary{
			{ServerID: srv1.ID, LibraryID: "lib1"},
			{ServerID: srv2.ID, LibraryID: "lib2"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sch.SyncAll(ctx); err != nil {
		t.Fatalf("SyncAll() error: %v", err)
	}

	// Verify candidates from both servers
	result, err := s.ListCandidatesForRule(ctx, rule.ID, 1, 100, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 {
		t.Errorf("expected 2 candidates from both servers, got %d", result.Total)
	}
}

func TestEvaluateAllRulesIndependently(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := seedServer(t, s, "test-server")
	now := time.Now().UTC()

	// Pre-seed library items directly (skip sync phase)
	items := []models.LibraryItemCache{
		{
			ServerID:        srv.ID,
			LibraryID:       "lib1",
			ItemID:          "m1",
			MediaType:       models.MediaTypeMovie,
			Title:           "Low Res Movie",
			Year:            2020,
			AddedAt:         now.AddDate(0, 0, -200),
			VideoResolution: "480",
			SyncedAt:        now,
		},
		{
			ServerID:  srv.ID,
			LibraryID: "lib1",
			ItemID:    "m2",
			MediaType: models.MediaTypeMovie,
			Title:     "Old Unwatched Movie",
			Year:      2019,
			AddedAt:   now.AddDate(0, 0, -200),
			SyncedAt:  now,
		},
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Create two different rules for the same library
	rule1, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name:          "Unwatched",
		CriterionType: models.CriterionUnwatchedMovie,
		MediaType:     models.MediaTypeMovie,
		Parameters:    json.RawMessage(`{"days": 90}`),
		Enabled:       true,
		Libraries:     []models.RuleLibrary{{ServerID: srv.ID, LibraryID: "lib1"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	rule2, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name:          "Low Res",
		CriterionType: models.CriterionLowResolution,
		MediaType:     models.MediaTypeMovie,
		Parameters:    json.RawMessage(`{"max_resolution": "720"}`),
		Enabled:       true,
		Libraries:     []models.RuleLibrary{{ServerID: srv.ID, LibraryID: "lib1"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	p := poller.New(s, 5*time.Second)
	sch := New(s, p, nil)

	totalCandidates, totalErrors := sch.evaluateAllRules(ctx)
	if totalErrors != 0 {
		t.Errorf("expected 0 errors, got %d", totalErrors)
	}
	if totalCandidates == 0 {
		t.Error("expected candidates from evaluation, got 0")
	}

	// Verify each rule has candidates
	r1, _ := s.ListCandidatesForRule(ctx, rule1.ID, 1, 100, "", "", "", 0, "")
	r2, _ := s.ListCandidatesForRule(ctx, rule2.ID, 1, 100, "", "", "", 0, "")

	if r1.Total != 2 {
		t.Errorf("rule1: expected 2 candidates, got %d", r1.Total)
	}
	if r2.Total != 1 {
		t.Errorf("rule2: expected 1 candidate (low res), got %d", r2.Total)
	}
}
