package rules

import (
	"context"
	"encoding/json"
	"testing"

	"streammon/internal/models"
)

// mockGeoResolver is defined in engine_test.go

func TestSimultaneousLocsEvaluator_Type(t *testing.T) {
	e := NewSimultaneousLocsEvaluator(nil)
	if e.Type() != models.RuleTypeSimultaneousLocs {
		t.Errorf("expected %s, got %s", models.RuleTypeSimultaneousLocs, e.Type())
	}
}

func TestSimultaneousLocsEvaluator_SingleLocation(t *testing.T) {
	geo := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"1.1.1.1": {Lat: 40.7128, Lng: -74.0060, City: "New York", Country: "US"},
			"1.1.1.2": {Lat: 40.7128, Lng: -74.0060, City: "New York", Country: "US"},
		},
	}
	e := NewSimultaneousLocsEvaluator(geo)
	ctx := context.Background()

	config := models.SimultaneousLocsConfig{
		MinDistanceKm: 50,
	}
	configJSON, _ := json.Marshal(config)

	rule := &models.Rule{
		ID:     1,
		Name:   "Same Location Check",
		Type:   models.RuleTypeSimultaneousLocs,
		Config: configJSON,
	}

	// Two streams from same location
	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			SessionID: "s1",
			UserName:  "testuser",
			IPAddress: "1.1.1.1",
		},
		AllStreams: []models.ActiveStream{
			{SessionID: "s1", UserName: "testuser", IPAddress: "1.1.1.1"},
			{SessionID: "s2", UserName: "testuser", IPAddress: "1.1.1.2"},
		},
	}

	result, err := e.Evaluate(ctx, rule, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not trigger since both are from same city
	if result != nil && result.Violation != nil {
		t.Errorf("expected no violation for same location, got %+v", result.Violation)
	}
}

func TestSimultaneousLocsEvaluator_DifferentLocations(t *testing.T) {
	geo := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"1.1.1.1": {Lat: 40.7128, Lng: -74.0060, City: "New York", Country: "US"},
			"2.2.2.2": {Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
		},
	}
	e := NewSimultaneousLocsEvaluator(geo)
	ctx := context.Background()

	config := models.SimultaneousLocsConfig{
		MinDistanceKm: 50,
	}
	configJSON, _ := json.Marshal(config)

	rule := &models.Rule{
		ID:     2,
		Name:   "Distance Check",
		Type:   models.RuleTypeSimultaneousLocs,
		Config: configJSON,
	}

	// Two streams from very different locations (NY and LA ~3940km apart)
	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			SessionID: "s1",
			UserName:  "testuser",
			IPAddress: "1.1.1.1",
		},
		AllStreams: []models.ActiveStream{
			{SessionID: "s1", UserName: "testuser", IPAddress: "1.1.1.1"},
			{SessionID: "s2", UserName: "testuser", IPAddress: "2.2.2.2"},
		},
	}

	result, err := e.Evaluate(ctx, rule, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil || result.Violation == nil {
		t.Fatal("expected violation for distant locations")
	}

	v := result.Violation
	if v.Severity != models.SeverityCritical {
		t.Errorf("expected critical severity for >500km distance, got %s", v.Severity)
	}
}

func TestSimultaneousLocsEvaluator_HouseholdExemption(t *testing.T) {
	geo := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"1.1.1.1": {Lat: 40.7128, Lng: -74.0060, City: "New York", Country: "US"},
			"2.2.2.2": {Lat: 34.0522, Lng: -118.2437, City: "Los Angeles", Country: "US"},
		},
	}
	e := NewSimultaneousLocsEvaluator(geo)
	ctx := context.Background()

	config := models.SimultaneousLocsConfig{
		MinDistanceKm:   50,
		ExemptHousehold: true,
	}
	configJSON, _ := json.Marshal(config)

	rule := &models.Rule{
		ID:     3,
		Name:   "Household Exempt",
		Type:   models.RuleTypeSimultaneousLocs,
		Config: configJSON,
	}

	// Both IPs are trusted household locations
	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			SessionID: "s1",
			UserName:  "testuser",
			IPAddress: "1.1.1.1",
		},
		AllStreams: []models.ActiveStream{
			{SessionID: "s1", UserName: "testuser", IPAddress: "1.1.1.1"},
			{SessionID: "s2", UserName: "testuser", IPAddress: "2.2.2.2"},
		},
		Households: []models.HouseholdLocation{
			{IPAddress: "1.1.1.1", Trusted: true},
			{IPAddress: "2.2.2.2", Trusted: true},
		},
	}

	result, err := e.Evaluate(ctx, rule, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not trigger because both are trusted household locations
	if result != nil && result.Violation != nil {
		t.Errorf("expected no violation for household IPs, got %+v", result.Violation)
	}
}

func TestSimultaneousLocsEvaluator_SingleStream(t *testing.T) {
	e := NewSimultaneousLocsEvaluator(nil)
	ctx := context.Background()

	config := models.SimultaneousLocsConfig{
		MinDistanceKm: 50,
	}
	configJSON, _ := json.Marshal(config)

	rule := &models.Rule{
		ID:     4,
		Name:   "Single Stream",
		Type:   models.RuleTypeSimultaneousLocs,
		Config: configJSON,
	}

	// Only one stream - can't have simultaneous locations
	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			SessionID: "s1",
			UserName:  "testuser",
			IPAddress: "1.1.1.1",
		},
		AllStreams: []models.ActiveStream{
			{SessionID: "s1", UserName: "testuser", IPAddress: "1.1.1.1"},
		},
	}

	result, err := e.Evaluate(ctx, rule, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != nil && result.Violation != nil {
		t.Errorf("expected no violation for single stream, got %+v", result.Violation)
	}
}

func TestSimultaneousLocsEvaluator_InvalidConfig(t *testing.T) {
	e := NewSimultaneousLocsEvaluator(nil)
	ctx := context.Background()

	rule := &models.Rule{
		ID:     5,
		Name:   "Invalid",
		Type:   models.RuleTypeSimultaneousLocs,
		Config: []byte(`{invalid json`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			SessionID: "s1",
			UserName:  "testuser",
			IPAddress: "1.1.1.1",
		},
		AllStreams: []models.ActiveStream{
			{SessionID: "s1", UserName: "testuser", IPAddress: "1.1.1.1"},
		},
	}

	_, err := e.Evaluate(ctx, rule, input)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestSimultaneousLocsEvaluator_ViolationDetails(t *testing.T) {
	geo := &mockGeoResolver{
		results: map[string]*models.GeoResult{
			"1.1.1.1": {Lat: 40.7128, Lng: -74.0060, City: "New York", Country: "US"},
			"2.2.2.2": {Lat: 51.5074, Lng: -0.1278, City: "London", Country: "GB"},
		},
	}
	e := NewSimultaneousLocsEvaluator(geo)
	ctx := context.Background()

	config := models.SimultaneousLocsConfig{
		MinDistanceKm: 50,
	}
	configJSON, _ := json.Marshal(config)

	rule := &models.Rule{
		ID:     6,
		Name:   "Transatlantic Check",
		Type:   models.RuleTypeSimultaneousLocs,
		Config: configJSON,
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			SessionID: "s1",
			UserName:  "testuser",
			IPAddress: "1.1.1.1",
		},
		AllStreams: []models.ActiveStream{
			{SessionID: "s1", UserName: "testuser", IPAddress: "1.1.1.1"},
			{SessionID: "s2", UserName: "testuser", IPAddress: "2.2.2.2"},
		},
	}

	result, err := e.Evaluate(ctx, rule, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil || result.Violation == nil {
		t.Fatal("expected violation")
	}

	v := result.Violation
	if v.RuleID != rule.ID {
		t.Errorf("expected rule ID %d, got %d", rule.ID, v.RuleID)
	}
	if v.UserName != "testuser" {
		t.Errorf("expected user testuser, got %s", v.UserName)
	}

	// Check details contain location info
	if v.Details == nil {
		t.Error("expected violation details")
	}
}

func TestHaversineDistance(t *testing.T) {
	// Test known distance: New York to London ~5570km
	nyLat, nyLng := 40.7128, -74.0060
	lonLat, lonLng := 51.5074, -0.1278

	dist := haversineDistance(nyLat, nyLng, lonLat, lonLng)

	// Allow 5% tolerance
	expected := 5570.0
	tolerance := expected * 0.05
	if dist < expected-tolerance || dist > expected+tolerance {
		t.Errorf("expected ~%.0fkm, got %.0fkm", expected, dist)
	}
}
