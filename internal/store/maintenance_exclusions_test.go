package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"streammon/internal/models"
)

func TestCreateExclusions(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	_, err := s.CreateExclusions(ctx, ruleID, []int64{itemID}, "admin@test.com")
	if err != nil {
		t.Fatalf("CreateExclusions: %v", err)
	}

	count, err := s.CountExclusionsForRule(ctx, ruleID)
	if err != nil {
		t.Fatalf("CountExclusionsForRule: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestCreateExclusionsDuplicate(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	if _, err := s.CreateExclusions(ctx, ruleID, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	// Should not error on duplicate, just return 0 new exclusions
	created, err := s.CreateExclusions(ctx, ruleID, []int64{itemID}, "admin@test.com")
	if err != nil {
		t.Fatalf("duplicate exclusion should not error: %v", err)
	}
	if created != 0 {
		t.Errorf("duplicate exclusion should return 0 created, got %d", created)
	}

	count, _ := s.CountExclusionsForRule(ctx, ruleID)
	if count != 1 {
		t.Errorf("count = %d, want 1 (no duplicate)", count)
	}
}

func TestDeleteExclusion(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	if _, err := s.CreateExclusions(ctx, ruleID, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteExclusion(ctx, ruleID, itemID); err != nil {
		t.Fatalf("DeleteExclusion: %v", err)
	}

	count, _ := s.CountExclusionsForRule(ctx, ruleID)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestDeleteExclusionNotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	// Should return ErrNotFound if exclusion doesn't exist
	err := s.DeleteExclusion(ctx, 99999, 99999)
	if err == nil {
		t.Error("DeleteExclusion for non-existent should return ErrNotFound")
	}
	if !errors.Is(err, models.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestDeleteExclusions(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, ruleID, itemID := seedMaintenanceTestData(t, s)

	// Create additional items
	items := createAdditionalItems(t, s, ctx, serverID, 2)
	allItemIDs := append([]int64{itemID}, items...)

	if _, err := s.CreateExclusions(ctx, ruleID, allItemIDs, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	count, _ := s.CountExclusionsForRule(ctx, ruleID)
	if count != 3 {
		t.Fatalf("setup: expected 3 exclusions, got %d", count)
	}

	// Delete two of them
	if _, err := s.DeleteExclusions(ctx, ruleID, []int64{itemID, items[0]}); err != nil {
		t.Fatalf("DeleteExclusions: %v", err)
	}

	count, _ = s.CountExclusionsForRule(ctx, ruleID)
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestListExclusionsForRule(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	if _, err := s.CreateExclusions(ctx, ruleID, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	result, err := s.ListExclusionsForRule(ctx, ruleID, 1, 10, "")
	if err != nil {
		t.Fatalf("ListExclusionsForRule: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("total = %d, want 1", result.Total)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	if result.Items[0].Item == nil {
		t.Fatal("expected Item to be populated")
	}
	if result.Items[0].Item.Title != "Test Movie" {
		t.Errorf("title = %q, want %q", result.Items[0].Item.Title, "Test Movie")
	}
	if result.Items[0].ExcludedBy != "admin@test.com" {
		t.Errorf("excluded_by = %q, want %q", result.Items[0].ExcludedBy, "admin@test.com")
	}
}

func TestListExclusionsForRulePagination(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, ruleID, itemID := seedMaintenanceTestData(t, s)

	items := createAdditionalItems(t, s, ctx, serverID, 4)
	allItemIDs := append([]int64{itemID}, items...)

	if _, err := s.CreateExclusions(ctx, ruleID, allItemIDs, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	page1, _ := s.ListExclusionsForRule(ctx, ruleID, 1, 2, "")
	if len(page1.Items) != 2 {
		t.Errorf("page 1 items = %d, want 2", len(page1.Items))
	}
	if page1.Total != 5 {
		t.Errorf("total = %d, want 5", page1.Total)
	}

	page3, _ := s.ListExclusionsForRule(ctx, ruleID, 3, 2, "")
	if len(page3.Items) != 1 {
		t.Errorf("page 3 items = %d, want 1", len(page3.Items))
	}
}

func TestIsItemExcluded(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	excluded, err := s.IsItemExcluded(ctx, ruleID, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if excluded {
		t.Error("item should not be excluded yet")
	}

	if _, err := s.CreateExclusions(ctx, ruleID, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	excluded, err = s.IsItemExcluded(ctx, ruleID, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if !excluded {
		t.Error("item should be excluded")
	}
}

func TestCandidatesExcludeExcludedItems(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, ruleID, itemID := seedMaintenanceTestData(t, s)

	items := createAdditionalItems(t, s, ctx, serverID, 2)
	allItemIDs := append([]int64{itemID}, items...)

	// Create candidates for all items
	var candidates []struct {
		LibraryItemID int64
		Reason        string
	}
	for _, id := range allItemIDs {
		candidates = append(candidates, struct {
			LibraryItemID int64
			Reason        string
		}{id, "Test reason"})
	}

	// Use BatchUpsertCandidates
	batchCandidates := make([]struct{ LibraryItemID int64; Reason string }, len(allItemIDs))
	for i, id := range allItemIDs {
		batchCandidates[i] = struct{ LibraryItemID int64; Reason string }{id, "Test reason"}
	}

	// Actually we need to import the models.BatchCandidate
	// Let me just use the store method directly
	for _, id := range allItemIDs {
		if err := s.UpsertMaintenanceCandidate(ctx, ruleID, id, "Test reason"); err != nil {
			t.Fatal(err)
		}
	}

	// Verify we have 3 candidates
	result, _ := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "", "", 0, "")
	if result.Total != 3 {
		t.Fatalf("setup: expected 3 candidates, got %d", result.Total)
	}

	// Exclude one item
	if _, err := s.CreateExclusions(ctx, ruleID, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	// Now list should return only 2
	result, err := s.ListCandidatesForRule(ctx, ruleID, 1, 10, "", "", "", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2 (excluded item filtered out)", result.Total)
	}
}

// Helper to create additional library items for testing
func createAdditionalItems(t *testing.T, s *Store, ctx context.Context, serverID int64, count int) []int64 {
	t.Helper()
	now := time.Now().UTC()
	ids := []int64{}

	for i := 0; i < count; i++ {
		itemID := fmt.Sprintf("extra_item_%d", i)
		libItems := []models.LibraryItemCache{{
			ServerID:  serverID,
			LibraryID: "lib1",
			ItemID:    itemID,
			MediaType: models.MediaTypeMovie,
			Title:     fmt.Sprintf("Extra Movie %d", i),
			Year:      2024,
			AddedAt:   now.AddDate(0, 0, -100),
			FileSize:  1024 * 1024 * 1024,
			SyncedAt:  now,
		}}

		if _, err := s.UpsertLibraryItems(ctx, libItems); err != nil {
			t.Fatal(err)
		}

		// Get the ID of the inserted item
		allItems, err := s.ListLibraryItems(ctx, serverID, "lib1")
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range allItems {
			if item.ItemID == itemID {
				ids = append(ids, item.ID)
				break
			}
		}
	}

	return ids
}

func TestListExclusionsForRuleSearch(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	if _, err := s.CreateExclusions(ctx, ruleID, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	// Search by title should find the item
	result, err := s.ListExclusionsForRule(ctx, ruleID, 1, 10, "Test Movie")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("search by title: total = %d, want 1", result.Total)
	}

	// Search by year should find the item
	result, err = s.ListExclusionsForRule(ctx, ruleID, 1, 10, "2024")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("search by year: total = %d, want 1", result.Total)
	}

	// Search for non-existent term should return empty
	result, err = s.ListExclusionsForRule(ctx, ruleID, 1, 10, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("search non-existent: total = %d, want 0", result.Total)
	}
}

func TestIsItemExcludedFromAnyRule(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, ruleID, itemID := seedMaintenanceTestData(t, s)

	// Not excluded from any rule yet
	excluded, err := s.IsItemExcludedFromAnyRule(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if excluded {
		t.Error("item should not be excluded from any rule yet")
	}

	// Exclude from rule 1
	if _, err := s.CreateExclusions(ctx, ruleID, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	excluded, err = s.IsItemExcludedFromAnyRule(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if !excluded {
		t.Error("item should be excluded (rule 1)")
	}

	// Create a second rule and exclude from it too
	rule2, err := s.CreateMaintenanceRule(ctx, &models.MaintenanceRuleInput{
		Name:          "Rule 2",
		CriterionType: "large_files",
		MediaType:     models.MediaTypeMovie,
		Parameters:    json.RawMessage(`{"min_size_gb": 50}`),
		Enabled:       true,
		Libraries:     []models.RuleLibrary{{ServerID: serverID, LibraryID: "lib1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateExclusions(ctx, rule2.ID, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	// Remove exclusion from rule 1 only — should still be excluded via rule 2
	if err := s.DeleteExclusion(ctx, ruleID, itemID); err != nil {
		t.Fatal(err)
	}

	excluded, err = s.IsItemExcludedFromAnyRule(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if !excluded {
		t.Error("item should still be excluded (rule 2)")
	}

	// Remove exclusion from rule 2 — now truly not excluded
	if err := s.DeleteExclusion(ctx, rule2.ID, itemID); err != nil {
		t.Fatal(err)
	}

	excluded, err = s.IsItemExcludedFromAnyRule(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if excluded {
		t.Error("item should no longer be excluded from any rule")
	}
}

func TestListExclusionsForRuleSearchEscapesWildcards(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, ruleID, itemID := seedMaintenanceTestData(t, s)

	if _, err := s.CreateExclusions(ctx, ruleID, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	// Search with SQL wildcard characters should be escaped and not match everything
	result, err := s.ListExclusionsForRule(ctx, ruleID, 1, 10, "%")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("search with %%: total = %d, want 0 (should not match everything)", result.Total)
	}

	result, err = s.ListExclusionsForRule(ctx, ruleID, 1, 10, "_")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("search with _: total = %d, want 0 (should not match single char)", result.Total)
	}
}
