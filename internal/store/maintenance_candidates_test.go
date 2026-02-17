package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"streammon/internal/models"
)

func seedMaintenanceTestData(t *testing.T, s *Store) (serverID int64, ruleID int64, itemID int64) {
	t.Helper()
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	items := []models.LibraryItemCache{{
		ServerID:   srv.ID,
		LibraryID:  "lib1",
		ItemID:     "item1",
		MediaType:  models.MediaTypeMovie,
		Title:      "Test Movie",
		Year:       2024,
		AddedAt:    time.Now().UTC().AddDate(0, 0, -100),
		FileSize:   1024 * 1024 * 1024,
		SyncedAt:   time.Now().UTC(),
	}}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	libItems, err := s.ListLibraryItems(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if len(libItems) == 0 {
		t.Fatal("no library items")
	}

	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name:          "Test Rule",
		MediaType:     models.MediaTypeMovie,
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
		Libraries:     []models.RuleLibrary{{ServerID: srv.ID, LibraryID: "lib1"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	return srv.ID, rule.ID, libItems[0].ID
}

func TestBatchUpsertCandidates(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	candidates := []models.BatchCandidate{
		{LibraryItemID: itemID, Reason: "Test reason"},
	}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatalf("BatchUpsertCandidates: %v", err)
	}

	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "", "", 0, "")
	if err != nil {
		t.Fatalf("ListCandidatesForRule: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
	if result.Items[0].Reason != "Test reason" {
		t.Errorf("reason = %q, want %q", result.Items[0].Reason, "Test reason")
	}
}

func TestBatchUpsertCandidatesReplacesExisting(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	candidates := []models.BatchCandidate{
		{LibraryItemID: itemID, Reason: "Initial reason"},
	}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	if err := s.BatchUpsertCandidates(ctx, ruleID, nil); err != nil {
		t.Fatal(err)
	}

	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("total = %d, want 0 (candidates should be cleared)", result.Total)
	}
}

func seedCandidatesFromItems(t *testing.T, s *Store, ctx context.Context, serverID, ruleID int64) {
	t.Helper()
	libItems, err := s.ListLibraryItems(ctx, serverID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	var candidates []models.BatchCandidate
	for _, item := range libItems {
		candidates = append(candidates, models.BatchCandidate{
			LibraryItemID: item.ID,
			Reason:        "Test",
		})
	}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}
}

func TestListCandidatesForRulePagination(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, ruleID, _ := seedMaintenanceTestData(t, s)

	now := time.Now().UTC()
	for i := 2; i <= 5; i++ {
		items := []models.LibraryItemCache{{
			ServerID:  serverID,
			LibraryID: "lib1",
			ItemID:    fmt.Sprintf("item%d", i),
			MediaType: models.MediaTypeMovie,
			Title:     fmt.Sprintf("Movie %d", i),
			Year:      2024,
			AddedAt:   now.AddDate(0, 0, -100+i),
			SyncedAt:  now,
		}}
		if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
			t.Fatal(err)
		}
	}

	seedCandidatesFromItems(t, s, ctx, serverID, ruleID)

	page1, _ := s.ListCandidatesForRule(ctx, ruleID, 1, 2, "", "", "", 0, "")
	if len(page1.Items) != 2 {
		t.Errorf("page 1 items = %d, want 2", len(page1.Items))
	}
	if page1.Total != 5 {
		t.Errorf("total = %d, want 5", page1.Total)
	}

	page2, _ := s.ListCandidatesForRule(ctx, ruleID, 2, 2, "", "", "", 0, "")
	if len(page2.Items) != 2 {
		t.Errorf("page 2 items = %d, want 2", len(page2.Items))
	}

	page3, _ := s.ListCandidatesForRule(ctx, ruleID, 3, 2, "", "", "", 0, "")
	if len(page3.Items) != 1 {
		t.Errorf("page 3 items = %d, want 1", len(page3.Items))
	}
}

func TestCountCandidatesForRule(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	count, err := s.CountCandidatesForRule(ctx, ruleID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	count, err = s.CountCandidatesForRule(ctx, ruleID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestDeleteCandidatesForRule(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	count, _ := s.CountCandidatesForRule(ctx, ruleID)
	if count != 1 {
		t.Fatalf("expected 1 candidate before delete, got %d", count)
	}

	if err := s.DeleteCandidatesForRule(ctx, ruleID); err != nil {
		t.Fatalf("DeleteCandidatesForRule: %v", err)
	}

	count, _ = s.CountCandidatesForRule(ctx, ruleID)
	if count != 0 {
		t.Errorf("count = %d, want 0 after delete", count)
	}
}

func TestUpsertMaintenanceCandidate(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	if err := s.UpsertMaintenanceCandidate(ctx, ruleID, itemID, "First reason"); err != nil {
		t.Fatalf("UpsertMaintenanceCandidate: %v", err)
	}

	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
	if result.Items[0].Reason != "First reason" {
		t.Errorf("reason = %q, want %q", result.Items[0].Reason, "First reason")
	}

	if err := s.UpsertMaintenanceCandidate(ctx, ruleID, itemID, "Updated reason"); err != nil {
		t.Fatalf("UpsertMaintenanceCandidate update: %v", err)
	}

	result, _ = s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "", "", 0, "")
	if result.Total != 1 {
		t.Errorf("total = %d, want 1 (should update, not insert)", result.Total)
	}
	if result.Items[0].Reason != "Updated reason" {
		t.Errorf("reason = %q, want %q", result.Items[0].Reason, "Updated reason")
	}
}

func TestListCandidatesForRuleItemPopulated(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].Item == nil {
		t.Fatal("expected Item to be populated")
	}
	if result.Items[0].Item.Title != "Test Movie" {
		t.Errorf("item title = %q, want %q", result.Items[0].Item.Title, "Test Movie")
	}
	if result.Items[0].Item.FileSize != 1024*1024*1024 {
		t.Errorf("item file size = %d, want %d", result.Items[0].Item.FileSize, 1024*1024*1024)
	}
}

func TestGetMaintenanceCandidate(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	result, _ := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "", "", 0, "")
	candidateID := result.Items[0].ID

	got, err := s.GetMaintenanceCandidate(ctx, candidateID)
	if err != nil {
		t.Fatalf("GetMaintenanceCandidate: %v", err)
	}
	if got.Item == nil {
		t.Fatal("expected Item to be populated")
	}
	if got.Item.Title != "Test Movie" {
		t.Errorf("title = %q, want %q", got.Item.Title, "Test Movie")
	}
}

func TestGetMaintenanceCandidateNotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, err := s.GetMaintenanceCandidate(ctx, 99999)
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteMaintenanceCandidate(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	result, _ := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "", "", 0, "")
	candidateID := result.Items[0].ID

	if err := s.DeleteMaintenanceCandidate(ctx, candidateID); err != nil {
		t.Fatalf("DeleteMaintenanceCandidate: %v", err)
	}

	_, err := s.GetMaintenanceCandidate(ctx, candidateID)
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteMaintenanceCandidateNotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	err := s.DeleteMaintenanceCandidate(ctx, 99999)
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRecordDeleteAction(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, _, _ := seedMaintenanceTestData(t, s)

	err := s.RecordDeleteAction(ctx, serverID, "item123", "Test Movie", "movie", 1024*1024*1024, "admin@test.com", true, "")
	if err != nil {
		t.Fatalf("RecordDeleteAction: %v", err)
	}

	var count int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM maintenance_delete_log WHERE server_id = ?`, serverID).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 delete log entry, got %d", count)
	}
}

func TestRecordDeleteActionWithError(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, _, _ := seedMaintenanceTestData(t, s)

	err := s.RecordDeleteAction(ctx, serverID, "item123", "Test Movie", "movie", 1024*1024*1024, "admin@test.com", false, "connection refused")
	if err != nil {
		t.Fatalf("RecordDeleteAction: %v", err)
	}

	var errMsg string
	err = s.db.QueryRowContext(ctx, `SELECT error_message FROM maintenance_delete_log WHERE server_id = ?`, serverID).Scan(&errMsg)
	if err != nil {
		t.Fatal(err)
	}
	if errMsg != "connection refused" {
		t.Errorf("error_message = %q, want %q", errMsg, "connection refused")
	}
}

func TestListAllCandidatesForRule(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	all, err := s.ListAllCandidatesForRule(ctx, ruleID)
	if err != nil {
		t.Fatalf("ListAllCandidatesForRule: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("got %d candidates, want 1", len(all))
	}
	if all[0].Item == nil {
		t.Error("expected Item to be populated")
	}
}

func TestListAllCandidatesForRuleEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	all, err := s.ListAllCandidatesForRule(ctx, 99999)
	if err != nil {
		t.Fatalf("ListAllCandidatesForRule: %v", err)
	}
	if all == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(all) != 0 {
		t.Errorf("got %d candidates, want 0", len(all))
	}
}

func TestListCandidatesForRuleSortByTitle(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, ruleID, _ := seedMaintenanceTestData(t, s)

	now := time.Now().UTC()
	titles := []string{"Alpha", "Charlie", "Bravo"}
	for i, title := range titles {
		items := []models.LibraryItemCache{{
			ServerID:  serverID,
			LibraryID: "lib1",
			ItemID:    fmt.Sprintf("sort_item_%d", i),
			MediaType: models.MediaTypeMovie,
			Title:     title,
			Year:      2024,
			AddedAt:   now.AddDate(0, 0, -50+i),
			SyncedAt:  now,
		}}
		if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
			t.Fatal(err)
		}
	}

	seedCandidatesFromItems(t, s, ctx, serverID, ruleID)

	// Sort by title ascending
	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "title", "asc", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) < 3 {
		t.Fatalf("expected at least 3 items, got %d", len(result.Items))
	}
	if result.Items[0].Item.Title != "Alpha" {
		t.Errorf("first item = %q, want %q", result.Items[0].Item.Title, "Alpha")
	}
	if result.Items[1].Item.Title != "Bravo" {
		t.Errorf("second item = %q, want %q", result.Items[1].Item.Title, "Bravo")
	}

	// Sort by title descending
	result, err = s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "title", "desc", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	last := result.Items[len(result.Items)-1]
	if last.Item.Title != "Alpha" {
		t.Errorf("last item = %q, want %q", last.Item.Title, "Alpha")
	}
}

func TestListCandidatesForRuleSortBySize(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, ruleID, _ := seedMaintenanceTestData(t, s)

	now := time.Now().UTC()
	sizes := []int64{3 * 1024 * 1024 * 1024, 1 * 1024 * 1024 * 1024, 5 * 1024 * 1024 * 1024}
	for i, sz := range sizes {
		items := []models.LibraryItemCache{{
			ServerID:  serverID,
			LibraryID: "lib1",
			ItemID:    fmt.Sprintf("size_item_%d", i),
			MediaType: models.MediaTypeMovie,
			Title:     fmt.Sprintf("Movie Size %d", i),
			Year:      2024,
			AddedAt:   now.AddDate(0, 0, -50),
			FileSize:  sz,
			SyncedAt:  now,
		}}
		if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
			t.Fatal(err)
		}
	}

	seedCandidatesFromItems(t, s, ctx, serverID, ruleID)

	// Sort by size descending — largest first
	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "size", "desc", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) < 2 {
		t.Fatalf("expected at least 2 items, got %d", len(result.Items))
	}
	if result.Items[0].Item.FileSize < result.Items[1].Item.FileSize {
		t.Errorf("descending: first (%d) should be >= second (%d)", result.Items[0].Item.FileSize, result.Items[1].Item.FileSize)
	}

	// Sort by size ascending — smallest first
	result, err = s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "size", "asc", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Items[0].Item.FileSize > result.Items[1].Item.FileSize {
		t.Errorf("ascending: first (%d) should be <= second (%d)", result.Items[0].Item.FileSize, result.Items[1].Item.FileSize)
	}
}

func TestListCandidatesForRuleSortInvalidColumn(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	// Invalid sort column should fall back to default (added_at DESC), not error
	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "bogus; DROP TABLE", "asc", 0, "")
	if err != nil {
		t.Fatalf("invalid sort column should not error: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
}

func TestListCandidatesForRuleSearch(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	// Search by title should find the item
	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "Test Movie", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("search by title: total = %d, want 1", result.Total)
	}

	// Search by year should find the item
	result, err = s.ListCandidatesForRule(ctx, ruleID, 1, 10, "2024", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("search by year: total = %d, want 1", result.Total)
	}

	// Search for non-existent term should return empty
	result, err = s.ListCandidatesForRule(ctx, ruleID, 1, 10, "nonexistent", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("search non-existent: total = %d, want 0", result.Total)
	}
}

func TestListCandidatesForRuleOtherCopies(t *testing.T) {
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

	// Movie exists on both servers with same TMDB ID
	itemsA := []models.LibraryItemCache{{
		ServerID: srvA.ID, LibraryID: "lib1", ItemID: "movieA",
		MediaType: models.MediaTypeMovie, Title: "Shared Movie", Year: 2020,
		TMDBID: "tmdb999", AddedAt: now.AddDate(0, 0, -100), SyncedAt: now,
	}}
	itemsB := []models.LibraryItemCache{{
		ServerID: srvB.ID, LibraryID: "lib2", ItemID: "movieB",
		MediaType: models.MediaTypeMovie, Title: "Shared Movie", Year: 2020,
		TMDBID: "tmdb999", AddedAt: now.AddDate(0, 0, -100), SyncedAt: now,
	}}
	// Unique movie only on server A
	itemsUnique := []models.LibraryItemCache{{
		ServerID: srvA.ID, LibraryID: "lib1", ItemID: "unique1",
		MediaType: models.MediaTypeMovie, Title: "Unique Movie", Year: 2021,
		TMDBID: "tmdb111", AddedAt: now.AddDate(0, 0, -100), SyncedAt: now,
	}}

	if _, err := s.UpsertLibraryItems(ctx, itemsA); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertLibraryItems(ctx, itemsB); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertLibraryItems(ctx, itemsUnique); err != nil {
		t.Fatal(err)
	}

	libItemsA, _ := s.ListLibraryItems(ctx, srvA.ID, "lib1")

	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name:          "Test Rule",
		MediaType:     models.MediaTypeMovie,
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
		Libraries:     []models.RuleLibrary{{ServerID: srvA.ID, LibraryID: "lib1"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var candidates []models.BatchCandidate
	for _, item := range libItemsA {
		candidates = append(candidates, models.BatchCandidate{LibraryItemID: item.ID, Reason: "Test"})
	}
	if err := s.BatchUpsertCandidates(ctx, rule.ID, candidates); err != nil {
		t.Fatal(err)
	}

	result, err := s.ListCandidatesForRule(ctx, rule.ID, 1, 10, "", "title", "asc", 0, "")
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range result.Items {
		if c.Item.Title == "Shared Movie" {
			if len(c.OtherCopies) != 1 {
				t.Errorf("Shared Movie: other_copies = %d, want 1", len(c.OtherCopies))
			} else if c.OtherCopies[0].ServerID != srvB.ID || c.OtherCopies[0].LibraryID != "lib2" {
				t.Errorf("Shared Movie: other copy = {%d, %s}, want {%d, lib2}", c.OtherCopies[0].ServerID, c.OtherCopies[0].LibraryID, srvB.ID)
			}
		} else if c.Item.Title == "Unique Movie" {
			if len(c.OtherCopies) != 0 {
				t.Errorf("Unique Movie: other_copies = %d, want 0", len(c.OtherCopies))
			}
		}
	}
}

func TestOtherCopiesIMDBMatch(t *testing.T) {
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
	// Items share IMDB but have different TMDB IDs
	if _, err := s.UpsertLibraryItems(ctx, []models.LibraryItemCache{{
		ServerID: srvA.ID, LibraryID: "lib1", ItemID: "m1",
		MediaType: models.MediaTypeMovie, Title: "IMDB Match", Year: 2020,
		TMDBID: "tmdb_a", IMDBID: "tt1234", AddedAt: now.AddDate(0, 0, -100), SyncedAt: now,
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertLibraryItems(ctx, []models.LibraryItemCache{{
		ServerID: srvB.ID, LibraryID: "lib2", ItemID: "m2",
		MediaType: models.MediaTypeMovie, Title: "IMDB Match", Year: 2020,
		TMDBID: "tmdb_b", IMDBID: "tt1234", AddedAt: now.AddDate(0, 0, -100), SyncedAt: now,
	}}); err != nil {
		t.Fatal(err)
	}

	libItems, _ := s.ListLibraryItems(ctx, srvA.ID, "lib1")
	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name: "Rule", MediaType: models.MediaTypeMovie, CriterionType: models.CriterionUnwatchedMovie,
		Parameters: json.RawMessage(`{}`), Enabled: true,
		Libraries: []models.RuleLibrary{{ServerID: srvA.ID, LibraryID: "lib1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BatchUpsertCandidates(ctx, rule.ID, []models.BatchCandidate{{LibraryItemID: libItems[0].ID, Reason: "Test"}}); err != nil {
		t.Fatal(err)
	}

	result, err := s.ListCandidatesForRule(ctx, rule.ID, 1, 10, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Items))
	}
	if len(result.Items[0].OtherCopies) != 1 {
		t.Errorf("IMDB match: other_copies = %d, want 1", len(result.Items[0].OtherCopies))
	}
}

func TestOtherCopiesNoExternalIDs(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srvA := &models.Server{Name: "Server A", Type: models.ServerTypePlex, URL: "http://a", APIKey: "keyA", Enabled: true}
	if err := s.CreateServer(srvA); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	if _, err := s.UpsertLibraryItems(ctx, []models.LibraryItemCache{{
		ServerID: srvA.ID, LibraryID: "lib1", ItemID: "no_ids",
		MediaType: models.MediaTypeMovie, Title: "No External IDs", Year: 2020,
		AddedAt: now.AddDate(0, 0, -100), SyncedAt: now,
	}}); err != nil {
		t.Fatal(err)
	}

	libItems, _ := s.ListLibraryItems(ctx, srvA.ID, "lib1")
	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name: "Rule", MediaType: models.MediaTypeMovie, CriterionType: models.CriterionUnwatchedMovie,
		Parameters: json.RawMessage(`{}`), Enabled: true,
		Libraries: []models.RuleLibrary{{ServerID: srvA.ID, LibraryID: "lib1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BatchUpsertCandidates(ctx, rule.ID, []models.BatchCandidate{{LibraryItemID: libItems[0].ID, Reason: "Test"}}); err != nil {
		t.Fatal(err)
	}

	result, err := s.ListCandidatesForRule(ctx, rule.ID, 1, 10, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Items))
	}
	if len(result.Items[0].OtherCopies) != 0 {
		t.Errorf("no external IDs: other_copies = %d, want 0", len(result.Items[0].OtherCopies))
	}
}

func TestOtherCopiesThreeServersDeduped(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srvA := &models.Server{Name: "A", Type: models.ServerTypePlex, URL: "http://a", APIKey: "a", Enabled: true}
	srvB := &models.Server{Name: "B", Type: models.ServerTypePlex, URL: "http://b", APIKey: "b", Enabled: true}
	srvC := &models.Server{Name: "C", Type: models.ServerTypePlex, URL: "http://c", APIKey: "c", Enabled: true}
	for _, srv := range []*models.Server{srvA, srvB, srvC} {
		if err := s.CreateServer(srv); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Now().UTC()
	tmdb := "tmdb_shared"
	for _, srv := range []*models.Server{srvA, srvB, srvC} {
		if _, err := s.UpsertLibraryItems(ctx, []models.LibraryItemCache{{
			ServerID: srv.ID, LibraryID: "lib1", ItemID: fmt.Sprintf("m_%d", srv.ID),
			MediaType: models.MediaTypeMovie, Title: "Three Way", Year: 2020,
			TMDBID: tmdb, AddedAt: now.AddDate(0, 0, -100), SyncedAt: now,
		}}); err != nil {
			t.Fatal(err)
		}
	}

	libItems, _ := s.ListLibraryItems(ctx, srvA.ID, "lib1")
	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name: "Rule", MediaType: models.MediaTypeMovie, CriterionType: models.CriterionUnwatchedMovie,
		Parameters: json.RawMessage(`{}`), Enabled: true,
		Libraries: []models.RuleLibrary{{ServerID: srvA.ID, LibraryID: "lib1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BatchUpsertCandidates(ctx, rule.ID, []models.BatchCandidate{{LibraryItemID: libItems[0].ID, Reason: "Test"}}); err != nil {
		t.Fatal(err)
	}

	result, err := s.ListCandidatesForRule(ctx, rule.ID, 1, 10, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Items))
	}
	if len(result.Items[0].OtherCopies) != 2 {
		t.Errorf("3-server: other_copies = %d, want 2", len(result.Items[0].OtherCopies))
	}

	// Verify no duplicate (server_id, library_id) entries
	seen := make(map[string]bool)
	for _, cp := range result.Items[0].OtherCopies {
		key := fmt.Sprintf("%d:%s", cp.ServerID, cp.LibraryID)
		if seen[key] {
			t.Errorf("duplicate other_copy: %s", key)
		}
		seen[key] = true
	}
}

func TestOtherCopiesSameServerExcluded(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Server", Type: models.ServerTypePlex, URL: "http://a", APIKey: "a", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	// Two items on the same server/library with the same TMDB ID (different editions)
	if _, err := s.UpsertLibraryItems(ctx, []models.LibraryItemCache{
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "edition1",
			MediaType: models.MediaTypeMovie, Title: "Movie Edition 1", Year: 2020,
			TMDBID: "tmdb_same", AddedAt: now.AddDate(0, 0, -100), SyncedAt: now},
		{ServerID: srv.ID, LibraryID: "lib1", ItemID: "edition2",
			MediaType: models.MediaTypeMovie, Title: "Movie Edition 2", Year: 2020,
			TMDBID: "tmdb_same", AddedAt: now.AddDate(0, 0, -99), SyncedAt: now},
	}); err != nil {
		t.Fatal(err)
	}

	libItems, _ := s.ListLibraryItems(ctx, srv.ID, "lib1")
	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name: "Rule", MediaType: models.MediaTypeMovie, CriterionType: models.CriterionUnwatchedMovie,
		Parameters: json.RawMessage(`{}`), Enabled: true,
		Libraries: []models.RuleLibrary{{ServerID: srv.ID, LibraryID: "lib1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var candidates []models.BatchCandidate
	for _, item := range libItems {
		candidates = append(candidates, models.BatchCandidate{LibraryItemID: item.ID, Reason: "Test"})
	}
	if err := s.BatchUpsertCandidates(ctx, rule.ID, candidates); err != nil {
		t.Fatal(err)
	}

	result, err := s.ListCandidatesForRule(ctx, rule.ID, 1, 10, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}

	// Neither item should show the other as an "other copy" since they're on the same server/library
	for _, c := range result.Items {
		if len(c.OtherCopies) != 0 {
			t.Errorf("%s: other_copies = %d, want 0 (same server/library should be excluded)", c.Item.Title, len(c.OtherCopies))
		}
	}
}

func TestListCandidatesForRuleSearchEscapesWildcards(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	// Search with SQL wildcard characters should be escaped and not match everything
	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "%", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("search with %%: total = %d, want 0 (should not match everything)", result.Total)
	}

	result, err = s.ListCandidatesForRule(ctx, ruleID, 1, 10, "_", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("search with _: total = %d, want 0 (should not match single char)", result.Total)
	}
}
