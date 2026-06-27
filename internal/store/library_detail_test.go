package store

import (
	"context"
	"testing"
	"time"

	"streammon/internal/models"
)

// seedLibraryItem inserts one library_items row and returns its PK.
func seedLibraryItem(t *testing.T, s *Store, it models.LibraryItemCache) int64 {
	t.Helper()
	if _, err := s.UpsertLibraryItems(context.Background(), []models.LibraryItemCache{it}); err != nil {
		t.Fatalf("seed item: %v", err)
	}
	var id int64
	err := s.db.QueryRow(`SELECT id FROM library_items WHERE server_id=? AND item_id=?`,
		it.ServerID, it.ItemID).Scan(&id)
	if err != nil {
		t.Fatalf("lookup id: %v", err)
	}
	return id
}

func TestListLibraryItemDetails_MovieAggregates(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := s.CreateServer(&models.Server{
		Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true,
	}); err != nil {
		t.Fatalf("seed server: %v", err)
	}

	id := seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "m1",
		MediaType: models.MediaTypeMovie, Title: "Dune", AddedAt: now, FileSize: 5000})

	for i, u := range []string{"alice", "alice", "bob"} {
		if err := s.InsertHistory(&models.WatchHistoryEntry{ServerID: 1, ItemID: "m1",
			UserName: u, MediaType: models.MediaTypeMovie, Title: "Dune",
			StartedAt: now.Add(time.Duration(i) * time.Hour),
			StoppedAt: now.Add(time.Duration(i)*time.Hour + time.Minute),
			WatchedMs: 1800000}); err != nil {
			t.Fatalf("insert history: %v", err)
		}
	}

	res, err := s.ListLibraryItemDetails(ctx, LibraryItemQuery{ServerID: 1, LibraryID: "1", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("ListLibraryItemDetails: %v", err)
	}
	if res.Total != 1 || len(res.Items) != 1 {
		t.Fatalf("total=%d items=%d, want 1/1", res.Total, len(res.Items))
	}
	got := res.Items[0]
	if got.ID != id || got.Plays != 3 || got.UniqueViewers != 2 {
		t.Errorf("got ID=%d plays=%d viewers=%d, want %d/3/2", got.ID, got.Plays, got.UniqueViewers, id)
	}
	if got.LastPlayedAt == nil {
		t.Error("LastPlayedAt should be set")
	}
	// bob has the most recent stopped_at (i=2), so he is the last viewer.
	if got.LastViewer != "bob" {
		t.Errorf("LastViewer=%q, want bob (most recent play's user)", got.LastViewer)
	}
}

func TestListLibraryItemDetails_NeverPlayed(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	if err := s.CreateServer(&models.Server{
		Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true,
	}); err != nil {
		t.Fatalf("seed server: %v", err)
	}

	seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "m9",
		MediaType: models.MediaTypeMovie, Title: "Unwatched", AddedAt: time.Now().UTC(), FileSize: 1})

	res, err := s.ListLibraryItemDetails(ctx, LibraryItemQuery{ServerID: 1, LibraryID: "1", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("ListLibraryItemDetails: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].Plays != 0 || res.Items[0].LastPlayedAt != nil {
		t.Errorf("never-played row wrong: %+v", res.Items)
	}
}

func seedThree(t *testing.T, s *Store) {
	t.Helper()
	now := time.Now().UTC()
	seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "a",
		MediaType: models.MediaTypeMovie, Title: "Alpha", AddedAt: now.Add(-3 * time.Hour), FileSize: 10})
	seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "b",
		MediaType: models.MediaTypeMovie, Title: "Beta", AddedAt: now.Add(-2 * time.Hour), FileSize: 20})
	seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "c",
		MediaType: models.MediaTypeMovie, Title: "Gamma", AddedAt: now.Add(-1 * time.Hour), FileSize: 30})
	// Only "Beta" gets a play.
	if err := s.InsertHistory(&models.WatchHistoryEntry{ServerID: 1, ItemID: "b", UserName: "x",
		MediaType: models.MediaTypeMovie, Title: "Beta", StartedAt: now, StoppedAt: now}); err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func TestListLibraryItemDetails_Search(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	if err := s.CreateServer(&models.Server{
		Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true,
	}); err != nil {
		t.Fatalf("seed server: %v", err)
	}
	seedThree(t, s)
	res, err := s.ListLibraryItemDetails(context.Background(), LibraryItemQuery{
		ServerID: 1, LibraryID: "1", Page: 1, PerPage: 20, Search: "amma"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Total != 1 || res.Items[0].Title != "Gamma" {
		t.Errorf("search amma => %+v, want only Gamma", res.Items)
	}
}

func TestListLibraryItemDetails_FilterUnplayed(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	if err := s.CreateServer(&models.Server{
		Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true,
	}); err != nil {
		t.Fatalf("seed server: %v", err)
	}
	seedThree(t, s)
	res, err := s.ListLibraryItemDetails(context.Background(), LibraryItemQuery{
		ServerID: 1, LibraryID: "1", Page: 1, PerPage: 20, Filter: "unplayed"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Total != 2 {
		t.Errorf("unplayed total=%d, want 2", res.Total)
	}
	for _, it := range res.Items {
		if it.Plays != 0 {
			t.Errorf("unplayed returned a played item: %+v", it)
		}
	}
}

func TestListLibraryItemDetails_SortTitleAsc(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	if err := s.CreateServer(&models.Server{
		Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true,
	}); err != nil {
		t.Fatalf("seed server: %v", err)
	}
	seedThree(t, s)
	res, err := s.ListLibraryItemDetails(context.Background(), LibraryItemQuery{
		ServerID: 1, LibraryID: "1", Page: 1, PerPage: 20, SortColumn: "li.title", SortOrder: "asc"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Items[0].Title != "Alpha" || res.Items[2].Title != "Gamma" {
		t.Errorf("sort title asc wrong order: %v", []string{res.Items[0].Title, res.Items[2].Title})
	}
}

func TestGetLibrarySummary(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Seed server to satisfy FK constraint (gets ID 1 in a fresh DB).
	if err := s.CreateServer(&models.Server{
		Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true,
	}); err != nil {
		t.Fatalf("seed server: %v", err)
	}

	// Two movies in library "1" on server 1: one watched, one never played.
	seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "m1",
		MediaType: models.MediaTypeMovie, Title: "Watched Movie", AddedAt: now, FileSize: 1000})
	seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "m2",
		MediaType: models.MediaTypeMovie, Title: "Never Movie", AddedAt: now, FileSize: 2000})

	if err := s.InsertHistory(&models.WatchHistoryEntry{ServerID: 1, ItemID: "m1",
		UserName: "alice", MediaType: models.MediaTypeMovie, Title: "Watched Movie",
		StartedAt: now, StoppedAt: now, WatchedMs: 3600000}); err != nil {
		t.Fatalf("insert history: %v", err)
	}

	got, err := s.GetLibrarySummary(ctx, 1, "1")
	if err != nil {
		t.Fatalf("GetLibrarySummary: %v", err)
	}
	if got.TotalTitles != 2 || got.WatchedTitles != 1 || got.NeverPlayed != 1 {
		t.Errorf("counts = %+v, want total=2 watched=1 never=1", got)
	}
	if got.TotalSize != 3000 || got.ReclaimableSize != 2000 {
		t.Errorf("sizes = total %d reclaimable %d, want 3000 / 2000", got.TotalSize, got.ReclaimableSize)
	}
}

func TestSeriesSizeHints(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := s.CreateServer(&models.Server{
		Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true,
	}); err != nil {
		t.Fatalf("seed server: %v", err)
	}

	// A show with episodes + size (a valid hint), a show with no size yet, and a movie.
	seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "show1",
		MediaType: models.MediaTypeTV, Title: "Show1", AddedAt: now, EpisodeCount: 10, FileSize: 5000})
	seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "show2",
		MediaType: models.MediaTypeTV, Title: "Show2", AddedAt: now, EpisodeCount: 3, FileSize: 0})
	seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "movie1",
		MediaType: models.MediaTypeMovie, Title: "Movie1", AddedAt: now, EpisodeCount: 0, FileSize: 7000})

	hints, err := s.SeriesSizeHints(ctx, 1, "1")
	if err != nil {
		t.Fatalf("SeriesSizeHints: %v", err)
	}
	if len(hints) != 1 {
		t.Fatalf("got %d hints, want 1 (only show1 has episodes + size)", len(hints))
	}
	if h := hints["show1"]; h.FileSize != 5000 || h.EpisodeCount != 10 {
		t.Errorf("show1 hint = %+v, want {FileSize:5000 EpisodeCount:10}", h)
	}
}

func TestGetLibrarySummary_ExcludesProtectedFromReclaimable(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := s.CreateServer(&models.Server{
		Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true,
	}); err != nil {
		t.Fatalf("seed server: %v", err)
	}

	// Two never-played titles; one is protected (kept) via maintenance_exclusions.
	seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "free",
		MediaType: models.MediaTypeMovie, Title: "Free", AddedAt: now, FileSize: 100})
	protectedID := seedLibraryItem(t, s, models.LibraryItemCache{ServerID: 1, LibraryID: "1", ItemID: "keep",
		MediaType: models.MediaTypeMovie, Title: "Keep", AddedAt: now, FileSize: 500})
	if _, err := s.CreateExclusions(ctx, []int64{protectedID}, "tester"); err != nil {
		t.Fatalf("protect item: %v", err)
	}

	got, err := s.GetLibrarySummary(ctx, 1, "1")
	if err != nil {
		t.Fatalf("GetLibrarySummary: %v", err)
	}
	if got.NeverPlayed != 2 || got.TotalSize != 600 {
		t.Errorf("never=%d total_size=%d, want 2 / 600", got.NeverPlayed, got.TotalSize)
	}
	// Reclaimable counts only the unprotected never-played title (100), not the protected one (500).
	if got.ReclaimableSize != 100 {
		t.Errorf("reclaimable=%d, want 100 (protected 500 excluded)", got.ReclaimableSize)
	}
}
