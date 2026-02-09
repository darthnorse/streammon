package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"streammon/internal/models"
)

func TestMaintenanceRuleCRUD(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	// Seed a server first (required for foreign key)
	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	// Create rule
	input := &models.MaintenanceRuleInput{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		Name:          "Test Rule",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 30}`),
		Enabled:       true,
	}
	rule, err := s.CreateMaintenanceRule(ctx, input)
	if err != nil {
		t.Fatalf("CreateMaintenanceRule: %v", err)
	}
	if rule.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if rule.Name != "Test Rule" {
		t.Errorf("name = %q, want %q", rule.Name, "Test Rule")
	}

	// Get rule
	got, err := s.GetMaintenanceRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetMaintenanceRule: %v", err)
	}
	if got.Name != rule.Name {
		t.Errorf("got name %q, want %q", got.Name, rule.Name)
	}

	// Update rule
	update := &models.MaintenanceRuleUpdateInput{
		Name:          "Updated Rule",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 60}`),
		Enabled:       false,
	}
	updated, err := s.UpdateMaintenanceRule(ctx, rule.ID, update)
	if err != nil {
		t.Fatalf("UpdateMaintenanceRule: %v", err)
	}
	if updated.Name != "Updated Rule" {
		t.Errorf("updated name = %q, want %q", updated.Name, "Updated Rule")
	}
	if updated.Enabled {
		t.Error("expected enabled = false")
	}

	// Delete rule
	if err := s.DeleteMaintenanceRule(ctx, rule.ID); err != nil {
		t.Fatalf("DeleteMaintenanceRule: %v", err)
	}

	// Verify deleted
	_, err = s.GetMaintenanceRule(ctx, rule.ID)
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestListMaintenanceRules(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	// Seed server
	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	// Create rules
	for i := 0; i < 3; i++ {
		input := &models.MaintenanceRuleInput{
			ServerID:      srv.ID,
			LibraryID:     "lib1",
			Name:          "Rule",
			CriterionType: models.CriterionUnwatchedMovie,
			Parameters:    json.RawMessage(`{}`),
			Enabled:       true,
		}
		if _, err := s.CreateMaintenanceRule(ctx, input); err != nil {
			t.Fatal(err)
		}
	}

	rules, err := s.ListMaintenanceRules(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatalf("ListMaintenanceRules: %v", err)
	}
	if len(rules) != 3 {
		t.Errorf("got %d rules, want 3", len(rules))
	}
}

func TestUpdateMaintenanceRuleNotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	update := &models.MaintenanceRuleUpdateInput{
		Name:          "X",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
	}
	_, err := s.UpdateMaintenanceRule(ctx, 999, update)
	if err == nil {
		t.Error("expected error for not found")
	}
}

func TestDeleteMaintenanceRuleNotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	err := s.DeleteMaintenanceRule(ctx, 999)
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListMaintenanceRulesWithCounts(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	// Seed server
	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	// Create rule
	input := &models.MaintenanceRuleInput{
		ServerID:      srv.ID,
		LibraryID:     "lib1",
		Name:          "Test Rule",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{}`),
		Enabled:       true,
	}
	rule, err := s.CreateMaintenanceRule(ctx, input)
	if err != nil {
		t.Fatal(err)
	}

	// List with counts (should be 0 candidates, 0 exclusions)
	rules, err := s.ListMaintenanceRulesWithCounts(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatalf("ListMaintenanceRulesWithCounts: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(rules))
	}
	if rules[0].ID != rule.ID {
		t.Errorf("rule ID = %d, want %d", rules[0].ID, rule.ID)
	}
	if rules[0].CandidateCount != 0 {
		t.Errorf("candidate count = %d, want 0", rules[0].CandidateCount)
	}
	if rules[0].ExclusionCount != 0 {
		t.Errorf("exclusion count = %d, want 0", rules[0].ExclusionCount)
	}
}

func TestListMaintenanceRulesWithCountsExclusions(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	// Seed server
	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
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

	// Create 3 library items
	now := time.Now().UTC()
	items := make([]models.LibraryItemCache, 3)
	for i := range items {
		items[i] = models.LibraryItemCache{
			ServerID:  srv.ID,
			LibraryID: "lib1",
			ItemID:    fmt.Sprintf("item%d", i+1),
			MediaType: models.MediaTypeMovie,
			Title:     fmt.Sprintf("Movie %d", i+1),
			Year:      2024,
			AddedAt:   now.AddDate(0, 0, -30),
			SyncedAt:  now,
		}
	}
	if _, err := s.UpsertLibraryItems(ctx, items); err != nil {
		t.Fatal(err)
	}

	// Look up the inserted library item IDs
	libItems, err := s.ListLibraryItems(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if len(libItems) != 3 {
		t.Fatalf("got %d library items, want 3", len(libItems))
	}

	// Create 3 candidates
	for _, item := range libItems {
		if err := s.UpsertMaintenanceCandidate(ctx, rule.ID, item.ID, "test reason"); err != nil {
			t.Fatal(err)
		}
	}

	// Exclude 1 item
	if _, err := s.CreateExclusions(ctx, rule.ID, []int64{libItems[0].ID}, "test"); err != nil {
		t.Fatal(err)
	}

	// Verify: 2 candidates (3 minus 1 excluded), 1 exclusion
	rules, err := s.ListMaintenanceRulesWithCounts(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatalf("ListMaintenanceRulesWithCounts: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(rules))
	}
	if rules[0].CandidateCount != 2 {
		t.Errorf("candidate count = %d, want 2", rules[0].CandidateCount)
	}
	if rules[0].ExclusionCount != 1 {
		t.Errorf("exclusion count = %d, want 1", rules[0].ExclusionCount)
	}

	// Exclude another item
	if _, err := s.CreateExclusions(ctx, rule.ID, []int64{libItems[1].ID}, "test"); err != nil {
		t.Fatal(err)
	}

	// Verify: 1 candidate, 2 exclusions
	rules, err = s.ListMaintenanceRulesWithCounts(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatalf("ListMaintenanceRulesWithCounts after second exclusion: %v", err)
	}
	if rules[0].CandidateCount != 1 {
		t.Errorf("candidate count = %d, want 1", rules[0].CandidateCount)
	}
	if rules[0].ExclusionCount != 2 {
		t.Errorf("exclusion count = %d, want 2", rules[0].ExclusionCount)
	}
}

func TestCreateMaintenanceRuleValidation(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	tests := []struct {
		name  string
		input *models.MaintenanceRuleInput
	}{
		{
			name: "missing server_id",
			input: &models.MaintenanceRuleInput{
				LibraryID:     "lib1",
				Name:          "Test",
				CriterionType: models.CriterionUnwatchedMovie,
			},
		},
		{
			name: "missing library_id",
			input: &models.MaintenanceRuleInput{
				ServerID:      1,
				Name:          "Test",
				CriterionType: models.CriterionUnwatchedMovie,
			},
		},
		{
			name: "missing name",
			input: &models.MaintenanceRuleInput{
				ServerID:      1,
				LibraryID:     "lib1",
				CriterionType: models.CriterionUnwatchedMovie,
			},
		},
		{
			name: "invalid criterion type",
			input: &models.MaintenanceRuleInput{
				ServerID:      1,
				LibraryID:     "lib1",
				Name:          "Test",
				CriterionType: "invalid_type",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.CreateMaintenanceRule(ctx, tt.input)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}
