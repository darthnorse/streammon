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

	// Create server
	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	// Create library item
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

	// Get item ID
	libItems, err := s.ListLibraryItems(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if len(libItems) == 0 {
		t.Fatal("no library items")
	}

	// Create rule
	rule, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		Name:          "Test Rule",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
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

	// Insert candidates
	candidates := []models.BatchCandidate{
		{LibraryItemID: itemID, Reason: "Test reason"},
	}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatalf("BatchUpsertCandidates: %v", err)
	}

	// Verify inserted
	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10)
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

	// Insert initial candidates
	candidates := []models.BatchCandidate{
		{LibraryItemID: itemID, Reason: "Initial reason"},
	}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	// Replace with new candidates (empty list)
	if err := s.BatchUpsertCandidates(ctx, ruleID, nil); err != nil {
		t.Fatal(err)
	}

	// Verify cleared
	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("total = %d, want 0 (candidates should be cleared)", result.Total)
	}
}

func TestListCandidatesForRulePagination(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, ruleID, _ := seedMaintenanceTestData(t, s)

	// Add more library items
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

	// Get all item IDs
	libItems, _ := s.ListLibraryItems(ctx, serverID, "lib1")

	// Insert candidates for all items
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

	// Test pagination
	page1, _ := s.ListCandidatesForRule(ctx, ruleID, 1, 2)
	if len(page1.Items) != 2 {
		t.Errorf("page 1 items = %d, want 2", len(page1.Items))
	}
	if page1.Total != 5 {
		t.Errorf("total = %d, want 5", page1.Total)
	}

	page2, _ := s.ListCandidatesForRule(ctx, ruleID, 2, 2)
	if len(page2.Items) != 2 {
		t.Errorf("page 2 items = %d, want 2", len(page2.Items))
	}

	page3, _ := s.ListCandidatesForRule(ctx, ruleID, 3, 2)
	if len(page3.Items) != 1 {
		t.Errorf("page 3 items = %d, want 1", len(page3.Items))
	}
}

func TestCountCandidatesForRule(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	// Empty initially
	count, err := s.CountCandidatesForRule(ctx, ruleID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	// Add candidate
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

	// Add candidate
	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	// Verify exists
	count, _ := s.CountCandidatesForRule(ctx, ruleID)
	if count != 1 {
		t.Fatalf("expected 1 candidate before delete, got %d", count)
	}

	// Delete candidates
	if err := s.DeleteCandidatesForRule(ctx, ruleID); err != nil {
		t.Fatalf("DeleteCandidatesForRule: %v", err)
	}

	// Verify deleted
	count, _ = s.CountCandidatesForRule(ctx, ruleID)
	if count != 0 {
		t.Errorf("count = %d, want 0 after delete", count)
	}
}

func TestUpsertMaintenanceCandidate(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	// Upsert single candidate
	if err := s.UpsertMaintenanceCandidate(ctx, ruleID, itemID, "First reason"); err != nil {
		t.Fatalf("UpsertMaintenanceCandidate: %v", err)
	}

	// Verify inserted
	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
	if result.Items[0].Reason != "First reason" {
		t.Errorf("reason = %q, want %q", result.Items[0].Reason, "First reason")
	}

	// Upsert again with different reason (should update)
	if err := s.UpsertMaintenanceCandidate(ctx, ruleID, itemID, "Updated reason"); err != nil {
		t.Fatalf("UpsertMaintenanceCandidate update: %v", err)
	}

	// Verify updated (still 1 candidate)
	result, _ = s.ListCandidatesForRule(ctx, ruleID, 1, 10)
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

	// Add candidate
	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	// Verify Item is populated
	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10)
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

	// Insert candidate
	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	// Get all candidates to find ID
	result, _ := s.ListCandidatesForRule(ctx, ruleID, 1, 10)
	candidateID := result.Items[0].ID

	// Get single candidate
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

	// Insert candidate
	candidates := []models.BatchCandidate{{LibraryItemID: itemID, Reason: "Test"}}
	if err := s.BatchUpsertCandidates(ctx, ruleID, candidates); err != nil {
		t.Fatal(err)
	}

	result, _ := s.ListCandidatesForRule(ctx, ruleID, 1, 10)
	candidateID := result.Items[0].ID

	// Delete
	if err := s.DeleteMaintenanceCandidate(ctx, candidateID); err != nil {
		t.Fatalf("DeleteMaintenanceCandidate: %v", err)
	}

	// Verify deleted
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

	// Record a deletion
	err := s.RecordDeleteAction(ctx, serverID, "item123", "Test Movie", "movie", 1024*1024*1024, "admin@test.com", true, "")
	if err != nil {
		t.Fatalf("RecordDeleteAction: %v", err)
	}

	// Verify it was recorded (we can query the table directly)
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

	// Record a failed deletion
	err := s.RecordDeleteAction(ctx, serverID, "item123", "Test Movie", "movie", 1024*1024*1024, "admin@test.com", false, "connection refused")
	if err != nil {
		t.Fatalf("RecordDeleteAction: %v", err)
	}

	// Verify error was recorded
	var errMsg string
	err = s.db.QueryRowContext(ctx, `SELECT error_message FROM maintenance_delete_log WHERE server_id = ?`, serverID).Scan(&errMsg)
	if err != nil {
		t.Fatal(err)
	}
	if errMsg != "connection refused" {
		t.Errorf("error_message = %q, want %q", errMsg, "connection refused")
	}
}
