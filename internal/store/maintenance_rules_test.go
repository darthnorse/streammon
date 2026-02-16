package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"streammon/internal/models"
)

func createTestRuleInput(srvID int64, libs ...models.RuleLibrary) *models.MaintenanceRuleInput {
	return &models.MaintenanceRuleInput{
		Name:          "Test Rule",
		MediaType:     models.MediaTypeMovie,
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 30}`),
		Enabled:       true,
		Libraries:     libs,
	}
}

func TestMaintenanceRuleCRUD(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	libs := []models.RuleLibrary{
		{ServerID: srv.ID, LibraryID: "lib1"},
		{ServerID: srv.ID, LibraryID: "lib2"},
	}

	// Create rule with multiple libraries
	input := createTestRuleInput(srv.ID, libs...)
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
	if rule.MediaType != models.MediaTypeMovie {
		t.Errorf("media_type = %q, want %q", rule.MediaType, models.MediaTypeMovie)
	}
	if len(rule.Libraries) != 2 {
		t.Fatalf("got %d libraries, want 2", len(rule.Libraries))
	}

	// Get rule - verify libraries are loaded
	got, err := s.GetMaintenanceRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetMaintenanceRule: %v", err)
	}
	if got.Name != rule.Name {
		t.Errorf("got name %q, want %q", got.Name, rule.Name)
	}
	if len(got.Libraries) != 2 {
		t.Fatalf("GetMaintenanceRule: got %d libraries, want 2", len(got.Libraries))
	}
	if got.Libraries[0].LibraryID != "lib1" || got.Libraries[1].LibraryID != "lib2" {
		t.Errorf("unexpected library IDs: %v", got.Libraries)
	}

	// Update rule with new libraries
	update := &models.MaintenanceRuleUpdateInput{
		Name:          "Updated Rule",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 60}`),
		Enabled:       false,
		Libraries: []models.RuleLibrary{
			{ServerID: srv.ID, LibraryID: "lib3"},
		},
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
	if len(updated.Libraries) != 1 {
		t.Fatalf("got %d libraries after update, want 1", len(updated.Libraries))
	}
	if updated.Libraries[0].LibraryID != "lib3" {
		t.Errorf("library_id = %q, want %q", updated.Libraries[0].LibraryID, "lib3")
	}

	// Update without changing libraries (empty Libraries slice)
	update2 := &models.MaintenanceRuleUpdateInput{
		Name:          "Updated Again",
		CriterionType: models.CriterionUnwatchedMovie,
		Parameters:    json.RawMessage(`{"days": 90}`),
		Enabled:       true,
	}
	updated2, err := s.UpdateMaintenanceRule(ctx, rule.ID, update2)
	if err != nil {
		t.Fatalf("UpdateMaintenanceRule (no lib change): %v", err)
	}
	if updated2.Name != "Updated Again" {
		t.Errorf("name = %q, want %q", updated2.Name, "Updated Again")
	}
	// Libraries should remain unchanged
	if len(updated2.Libraries) != 1 {
		t.Fatalf("libraries changed unexpectedly: got %d, want 1", len(updated2.Libraries))
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

func TestDeleteMaintenanceRuleCascadesJunction(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	libs := []models.RuleLibrary{
		{ServerID: srv.ID, LibraryID: "lib1"},
		{ServerID: srv.ID, LibraryID: "lib2"},
	}
	rule, err := s.CreateMaintenanceRule(ctx, createTestRuleInput(srv.ID, libs...))
	if err != nil {
		t.Fatal(err)
	}

	// Verify junction rows exist
	count, err := s.CountRulesForLibrary(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 rule for lib1 before delete, got %d", count)
	}

	// Delete rule
	if err := s.DeleteMaintenanceRule(ctx, rule.ID); err != nil {
		t.Fatal(err)
	}

	// Verify junction rows are gone
	count, err = s.CountRulesForLibrary(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 rules for lib1 after delete, got %d", count)
	}
}

func TestListMaintenanceRules(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	// Create 3 rules all targeting lib1
	for i := 0; i < 3; i++ {
		input := createTestRuleInput(srv.ID, models.RuleLibrary{ServerID: srv.ID, LibraryID: "lib1"})
		input.Name = fmt.Sprintf("Rule %d", i)
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
	// Verify each rule has libraries populated
	for _, r := range rules {
		if len(r.Libraries) != 1 {
			t.Errorf("rule %d has %d libraries, want 1", r.ID, len(r.Libraries))
		}
	}
}

func TestListMaintenanceRulesFilteredByJunction(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv1 := &models.Server{Name: "S1", Type: models.ServerTypePlex, URL: "http://s1", APIKey: "k1", Enabled: true}
	if err := s.CreateServer(srv1); err != nil {
		t.Fatal(err)
	}
	srv2 := &models.Server{Name: "S2", Type: models.ServerTypePlex, URL: "http://s2", APIKey: "k2", Enabled: true}
	if err := s.CreateServer(srv2); err != nil {
		t.Fatal(err)
	}

	// Rule 1: targets srv1/lib1 and srv1/lib2
	r1Input := createTestRuleInput(srv1.ID,
		models.RuleLibrary{ServerID: srv1.ID, LibraryID: "lib1"},
		models.RuleLibrary{ServerID: srv1.ID, LibraryID: "lib2"},
	)
	r1Input.Name = "Rule1"
	if _, err := s.CreateMaintenanceRule(ctx, r1Input); err != nil {
		t.Fatal(err)
	}

	// Rule 2: targets srv2/lib1 only
	r2Input := createTestRuleInput(srv2.ID,
		models.RuleLibrary{ServerID: srv2.ID, LibraryID: "lib1"},
	)
	r2Input.Name = "Rule2"
	if _, err := s.CreateMaintenanceRule(ctx, r2Input); err != nil {
		t.Fatal(err)
	}

	// Rule 3: targets both srv1/lib1 and srv2/lib1 (cross-server)
	r3Input := createTestRuleInput(srv1.ID,
		models.RuleLibrary{ServerID: srv1.ID, LibraryID: "lib1"},
		models.RuleLibrary{ServerID: srv2.ID, LibraryID: "lib1"},
	)
	r3Input.Name = "Rule3"
	if _, err := s.CreateMaintenanceRule(ctx, r3Input); err != nil {
		t.Fatal(err)
	}

	// Filter by srv1 only -> should get Rule1 + Rule3
	rules, err := s.ListMaintenanceRules(ctx, srv1.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Errorf("srv1 filter: got %d rules, want 2", len(rules))
	}

	// Filter by srv1/lib2 -> should get Rule1 only
	rules, err = s.ListMaintenanceRules(ctx, srv1.ID, "lib2")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Errorf("srv1/lib2 filter: got %d rules, want 1", len(rules))
	}

	// Filter by srv2/lib1 -> should get Rule2 + Rule3
	rules, err = s.ListMaintenanceRules(ctx, srv2.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Errorf("srv2/lib1 filter: got %d rules, want 2", len(rules))
	}

	// No filter -> all 3 rules
	rules, err = s.ListMaintenanceRules(ctx, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 {
		t.Errorf("no filter: got %d rules, want 3", len(rules))
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

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	input := createTestRuleInput(srv.ID, models.RuleLibrary{ServerID: srv.ID, LibraryID: "lib1"})
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
	// Verify libraries are populated
	if len(rules[0].Libraries) != 1 {
		t.Errorf("libraries count = %d, want 1", len(rules[0].Libraries))
	}
}

func TestListMaintenanceRulesWithCountsExclusions(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	rule, err := s.CreateMaintenanceRule(ctx, createTestRuleInput(srv.ID,
		models.RuleLibrary{ServerID: srv.ID, LibraryID: "lib1"},
	))
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
			name: "missing name",
			input: &models.MaintenanceRuleInput{
				MediaType:     models.MediaTypeMovie,
				CriterionType: models.CriterionUnwatchedMovie,
				Libraries:     []models.RuleLibrary{{ServerID: 1, LibraryID: "lib1"}},
			},
		},
		{
			name: "invalid criterion type",
			input: &models.MaintenanceRuleInput{
				Name:          "Test",
				MediaType:     models.MediaTypeMovie,
				CriterionType: "invalid_type",
				Libraries:     []models.RuleLibrary{{ServerID: 1, LibraryID: "lib1"}},
			},
		},
		{
			name: "missing libraries",
			input: &models.MaintenanceRuleInput{
				Name:          "Test",
				MediaType:     models.MediaTypeMovie,
				CriterionType: models.CriterionUnwatchedMovie,
			},
		},
		{
			name: "invalid media_type",
			input: &models.MaintenanceRuleInput{
				Name:          "Test",
				MediaType:     "invalid",
				CriterionType: models.CriterionUnwatchedMovie,
				Libraries:     []models.RuleLibrary{{ServerID: 1, LibraryID: "lib1"}},
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

func TestListAllMaintenanceRules(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	// Create 2 enabled rules and 1 disabled rule
	for i := 0; i < 2; i++ {
		input := createTestRuleInput(srv.ID, models.RuleLibrary{ServerID: srv.ID, LibraryID: "lib1"})
		input.Name = fmt.Sprintf("Enabled Rule %d", i)
		input.Enabled = true
		if _, err := s.CreateMaintenanceRule(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	disabledInput := createTestRuleInput(srv.ID, models.RuleLibrary{ServerID: srv.ID, LibraryID: "lib1"})
	disabledInput.Name = "Disabled Rule"
	disabledInput.Enabled = false
	if _, err := s.CreateMaintenanceRule(ctx, disabledInput); err != nil {
		t.Fatal(err)
	}

	// ListAllMaintenanceRules should only return enabled rules
	rules, err := s.ListAllMaintenanceRules(ctx)
	if err != nil {
		t.Fatalf("ListAllMaintenanceRules: %v", err)
	}
	if len(rules) != 2 {
		t.Errorf("got %d rules, want 2 (enabled only)", len(rules))
	}
	for _, r := range rules {
		if !r.Enabled {
			t.Errorf("rule %d should be enabled", r.ID)
		}
		if len(r.Libraries) != 1 {
			t.Errorf("rule %d has %d libraries, want 1", r.ID, len(r.Libraries))
		}
	}
}

func TestCountRulesForLibrary(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	// Create 2 rules targeting lib1, 1 rule targeting lib2
	for i := 0; i < 2; i++ {
		input := createTestRuleInput(srv.ID, models.RuleLibrary{ServerID: srv.ID, LibraryID: "lib1"})
		input.Name = fmt.Sprintf("Lib1 Rule %d", i)
		if _, err := s.CreateMaintenanceRule(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	lib2Input := createTestRuleInput(srv.ID, models.RuleLibrary{ServerID: srv.ID, LibraryID: "lib2"})
	lib2Input.Name = "Lib2 Rule"
	if _, err := s.CreateMaintenanceRule(ctx, lib2Input); err != nil {
		t.Fatal(err)
	}

	count, err := s.CountRulesForLibrary(ctx, srv.ID, "lib1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count for lib1 = %d, want 2", count)
	}

	count, err = s.CountRulesForLibrary(ctx, srv.ID, "lib2")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("count for lib2 = %d, want 1", count)
	}

	count, err = s.CountRulesForLibrary(ctx, srv.ID, "lib_nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count for non-existent lib = %d, want 0", count)
	}
}

func TestGetMaintenanceRuleReturnsEmptyLibrariesSlice(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()

	srv := &models.Server{Name: "Test", Type: models.ServerTypePlex, URL: "http://test", APIKey: "key", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	// Insert a rule directly (bypassing junction) to simulate an edge case
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO maintenance_rules (name, media_type, criterion_type, parameters, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"Orphan Rule", models.MediaTypeMovie, models.CriterionUnwatchedMovie, "{}", 1, now, now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()

	rule, err := s.GetMaintenanceRule(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	// Should return empty slice, not nil
	if rule.Libraries == nil {
		t.Error("Libraries should be empty slice, not nil")
	}
	if len(rule.Libraries) != 0 {
		t.Errorf("expected 0 libraries, got %d", len(rule.Libraries))
	}
}
