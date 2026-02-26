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

	_, _, itemID := seedMaintenanceTestData(t, s)

	_, err := s.CreateExclusions(ctx, []int64{itemID}, "admin@test.com")
	if err != nil {
		t.Fatalf("CreateExclusions: %v", err)
	}

	count, err := s.CountExclusions(ctx)
	if err != nil {
		t.Fatalf("CountExclusions: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestCreateExclusionsDuplicate(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, _, itemID := seedMaintenanceTestData(t, s)

	if _, err := s.CreateExclusions(ctx, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	created, err := s.CreateExclusions(ctx, []int64{itemID}, "admin@test.com")
	if err != nil {
		t.Fatalf("duplicate exclusion should not error: %v", err)
	}
	if created != 0 {
		t.Errorf("duplicate exclusion should return 0 created, got %d", created)
	}

	count, _ := s.CountExclusions(ctx)
	if count != 1 {
		t.Errorf("count = %d, want 1 (no duplicate)", count)
	}
}

func TestDeleteExclusion(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, _, itemID := seedMaintenanceTestData(t, s)

	if _, err := s.CreateExclusions(ctx, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteExclusion(ctx, itemID); err != nil {
		t.Fatalf("DeleteExclusion: %v", err)
	}

	count, _ := s.CountExclusions(ctx)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestDeleteExclusionNotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	// Should return ErrNotFound if exclusion doesn't exist
	err := s.DeleteExclusion(ctx, 99999)
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

	serverID, _, itemID := seedMaintenanceTestData(t, s)

	// Create additional items
	items := createAdditionalItems(t, s, ctx, serverID, 2)
	allItemIDs := append([]int64{itemID}, items...)

	if _, err := s.CreateExclusions(ctx, allItemIDs, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	count, _ := s.CountExclusions(ctx)
	if count != 3 {
		t.Fatalf("setup: expected 3 exclusions, got %d", count)
	}

	// Delete two of them
	if _, err := s.DeleteExclusions(ctx, []int64{itemID, items[0]}); err != nil {
		t.Fatalf("DeleteExclusions: %v", err)
	}

	count, _ = s.CountExclusions(ctx)
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestCountExclusions(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, _, itemID := seedMaintenanceTestData(t, s)

	// Zero initially
	count, err := s.CountExclusions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	// Add one
	if _, err := s.CreateExclusions(ctx, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}
	count, err = s.CountExclusions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// Add more
	items := createAdditionalItems(t, s, ctx, serverID, 2)
	if _, err := s.CreateExclusions(ctx, items, "admin@test.com"); err != nil {
		t.Fatal(err)
	}
	count, err = s.CountExclusions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	// Remove one, verify count decreases
	if err := s.DeleteExclusion(ctx, itemID); err != nil {
		t.Fatal(err)
	}
	count, err = s.CountExclusions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestListExclusions(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, _, itemID := seedMaintenanceTestData(t, s)

	if _, err := s.CreateExclusions(ctx, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	result, err := s.ListExclusions(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("ListExclusions: %v", err)
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

func TestListExclusionsPagination(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, _, itemID := seedMaintenanceTestData(t, s)

	items := createAdditionalItems(t, s, ctx, serverID, 4)
	allItemIDs := append([]int64{itemID}, items...)

	if _, err := s.CreateExclusions(ctx, allItemIDs, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	page1, _ := s.ListExclusions(ctx, 1, 2, "")
	if len(page1.Items) != 2 {
		t.Errorf("page 1 items = %d, want 2", len(page1.Items))
	}
	if page1.Total != 5 {
		t.Errorf("total = %d, want 5", page1.Total)
	}

	page3, _ := s.ListExclusions(ctx, 3, 2, "")
	if len(page3.Items) != 1 {
		t.Errorf("page 3 items = %d, want 1", len(page3.Items))
	}
}

func TestIsItemExcluded(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, _, itemID := seedMaintenanceTestData(t, s)

	excluded, err := s.IsItemExcluded(ctx, itemID)
	if err != nil {
		t.Fatal(err)
	}
	if excluded {
		t.Error("item should not be excluded yet")
	}

	if _, err := s.CreateExclusions(ctx, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	excluded, err = s.IsItemExcluded(ctx, itemID)
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

	for _, id := range allItemIDs {
		if err := s.UpsertMaintenanceCandidate(ctx, ruleID, id, "Test reason"); err != nil {
			t.Fatal(err)
		}
	}

	// Verify we have 3 candidates
	result, _ := s.ListCandidatesForRule(ctx, ruleID, models.CandidateListOptions{Page: 1, PerPage: 10})
	if result.Total != 3 {
		t.Fatalf("setup: expected 3 candidates, got %d", result.Total)
	}

	// Exclude one item globally
	if _, err := s.CreateExclusions(ctx, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	// Now list should return only 2
	result, err := s.ListCandidatesForRule(ctx, ruleID, models.CandidateListOptions{Page: 1, PerPage: 10})
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

func TestListExclusionsSearch(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, _, itemID := seedMaintenanceTestData(t, s)

	if _, err := s.CreateExclusions(ctx, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	// Search by title should find the item
	result, err := s.ListExclusions(ctx, 1, 10, "Test Movie")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("search by title: total = %d, want 1", result.Total)
	}

	// Search by year should find the item
	result, err = s.ListExclusions(ctx, 1, 10, "2024")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Errorf("search by year: total = %d, want 1", result.Total)
	}

	// Search for non-existent term should return empty
	result, err = s.ListExclusions(ctx, 1, 10, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("search non-existent: total = %d, want 0", result.Total)
	}
}

func TestListExclusionsSearchEscapesWildcards(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	_, _, itemID := seedMaintenanceTestData(t, s)

	if _, err := s.CreateExclusions(ctx, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	// Search with SQL wildcard characters should be escaped and not match everything
	result, err := s.ListExclusions(ctx, 1, 10, "%")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("search with %%: total = %d, want 0 (should not match everything)", result.Total)
	}

	result, err = s.ListExclusions(ctx, 1, 10, "_")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 {
		t.Errorf("search with _: total = %d, want 0 (should not match single char)", result.Total)
	}
}

func TestGlobalExclusionFiltersCandidatesAcrossRules(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, ruleID, itemID := seedMaintenanceTestData(t, s)

	// Create a second rule
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

	// Add the item as a candidate for both rules
	if err := s.UpsertMaintenanceCandidate(ctx, ruleID, itemID, "reason 1"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMaintenanceCandidate(ctx, rule2.ID, itemID, "reason 2"); err != nil {
		t.Fatal(err)
	}

	// Verify both rules have 1 candidate
	r1, _ := s.ListCandidatesForRule(ctx, ruleID, models.CandidateListOptions{Page: 1, PerPage: 10})
	if r1.Total != 1 {
		t.Fatalf("rule 1: expected 1 candidate, got %d", r1.Total)
	}
	r2, _ := s.ListCandidatesForRule(ctx, rule2.ID, models.CandidateListOptions{Page: 1, PerPage: 10})
	if r2.Total != 1 {
		t.Fatalf("rule 2: expected 1 candidate, got %d", r2.Total)
	}

	// Exclude the item globally
	if _, err := s.CreateExclusions(ctx, []int64{itemID}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	// Both rules should now have 0 candidates
	r1, _ = s.ListCandidatesForRule(ctx, ruleID, models.CandidateListOptions{Page: 1, PerPage: 10})
	if r1.Total != 0 {
		t.Errorf("rule 1: expected 0 candidates after global exclusion, got %d", r1.Total)
	}
	r2, _ = s.ListCandidatesForRule(ctx, rule2.ID, models.CandidateListOptions{Page: 1, PerPage: 10})
	if r2.Total != 0 {
		t.Errorf("rule 2: expected 0 candidates after global exclusion, got %d", r2.Total)
	}
}

func TestListExcludedCandidatesForRule(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	serverID, ruleID, itemID := seedMaintenanceTestData(t, s)

	// Create a second rule
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

	// Create another item and make it a candidate for rule 2 only
	extraItems := createAdditionalItems(t, s, ctx, serverID, 1)
	if err := s.UpsertMaintenanceCandidate(ctx, rule2.ID, extraItems[0], "reason extra"); err != nil {
		t.Fatal(err)
	}

	// Make itemID a candidate for both rules
	if err := s.UpsertMaintenanceCandidate(ctx, ruleID, itemID, "reason 1"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMaintenanceCandidate(ctx, rule2.ID, itemID, "reason 2"); err != nil {
		t.Fatal(err)
	}

	// Exclude both items globally
	if _, err := s.CreateExclusions(ctx, []int64{itemID, extraItems[0]}, "admin@test.com"); err != nil {
		t.Fatal(err)
	}

	// Rule 1: should only see itemID as excluded candidate (only it is a candidate for rule 1)
	result, err := s.ListExcludedCandidatesForRule(ctx, ruleID, 1, 10, "")
	if err != nil {
		t.Fatalf("ListExcludedCandidatesForRule: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("rule 1: expected 1 excluded candidate, got %d", result.Total)
	}

	// Rule 2: should see both items as excluded candidates
	result, err = s.ListExcludedCandidatesForRule(ctx, rule2.ID, 1, 10, "")
	if err != nil {
		t.Fatalf("ListExcludedCandidatesForRule: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("rule 2: expected 2 excluded candidates, got %d", result.Total)
	}
}

