package rules

import (
	"context"
	"encoding/json"
	"testing"

	"streammon/internal/models"
)

func TestGeoRestrictionEvaluator_Type(t *testing.T) {
	e := NewGeoRestrictionEvaluator()
	if e.Type() != models.RuleTypeGeoRestriction {
		t.Errorf("expected %s, got %s", models.RuleTypeGeoRestriction, e.Type())
	}
}

func TestGeoRestrictionEvaluator_AllowedCountries(t *testing.T) {
	e := NewGeoRestrictionEvaluator()
	ctx := context.Background()

	config := models.GeoRestrictionConfig{
		AllowedCountries: []string{"US", "CA", "GB"},
	}
	configJSON, _ := json.Marshal(config)

	rule := &models.Rule{
		ID:     1,
		Name:   "US/CA/GB Only",
		Type:   models.RuleTypeGeoRestriction,
		Config: configJSON,
	}

	tests := []struct {
		name      string
		country   string
		wantViol  bool
	}{
		{"allowed country US", "US", false},
		{"allowed country CA", "CA", false},
		{"allowed country GB", "GB", false},
		{"blocked country DE", "DE", true},
		{"blocked country RU", "RU", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &EvaluationInput{
				Stream: &models.ActiveStream{
					UserName:  "testuser",
					IPAddress: "1.2.3.4",
				},
				GeoData: &models.GeoResult{
					Country: tt.country,
					City:    "TestCity",
				},
			}

			result, err := e.Evaluate(ctx, rule, input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantViol && result == nil {
				t.Error("expected violation, got nil")
			}
			if !tt.wantViol && result != nil && result.Violation != nil {
				t.Errorf("expected no violation, got %+v", result.Violation)
			}
		})
	}
}

func TestGeoRestrictionEvaluator_BlockedCountries(t *testing.T) {
	e := NewGeoRestrictionEvaluator()
	ctx := context.Background()

	config := models.GeoRestrictionConfig{
		BlockedCountries: []string{"RU", "CN"},
	}
	configJSON, _ := json.Marshal(config)

	rule := &models.Rule{
		ID:     2,
		Name:   "Block RU/CN",
		Type:   models.RuleTypeGeoRestriction,
		Config: configJSON,
	}

	tests := []struct {
		name      string
		country   string
		wantViol  bool
	}{
		{"allowed country US", "US", false},
		{"allowed country DE", "DE", false},
		{"blocked country RU", "RU", true},
		{"blocked country CN", "CN", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &EvaluationInput{
				Stream: &models.ActiveStream{
					UserName:  "testuser",
					IPAddress: "1.2.3.4",
				},
				GeoData: &models.GeoResult{
					Country: tt.country,
					City:    "TestCity",
				},
			}

			result, err := e.Evaluate(ctx, rule, input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantViol && result == nil {
				t.Error("expected violation, got nil")
			}
			if !tt.wantViol && result != nil && result.Violation != nil {
				t.Errorf("expected no violation, got %+v", result.Violation)
			}
		})
	}
}

func TestGeoRestrictionEvaluator_NoGeoData(t *testing.T) {
	e := NewGeoRestrictionEvaluator()
	ctx := context.Background()

	config := models.GeoRestrictionConfig{
		AllowedCountries: []string{"US"},
	}
	configJSON, _ := json.Marshal(config)

	rule := &models.Rule{
		ID:     3,
		Name:   "US Only",
		Type:   models.RuleTypeGeoRestriction,
		Config: configJSON,
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "testuser",
			IPAddress: "1.2.3.4",
		},
		GeoData: nil, // No geo data available
	}

	result, err := e.Evaluate(ctx, rule, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not produce a violation when geo data is unavailable
	if result != nil && result.Violation != nil {
		t.Errorf("expected no violation when geo data unavailable, got %+v", result.Violation)
	}
}

func TestGeoRestrictionEvaluator_ViolationDetails(t *testing.T) {
	e := NewGeoRestrictionEvaluator()
	ctx := context.Background()

	config := models.GeoRestrictionConfig{
		AllowedCountries: []string{"US"},
	}
	configJSON, _ := json.Marshal(config)

	rule := &models.Rule{
		ID:     4,
		Name:   "US Only",
		Type:   models.RuleTypeGeoRestriction,
		Config: configJSON,
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "testuser",
			IPAddress: "1.2.3.4",
		},
		GeoData: &models.GeoResult{
			Country: "DE",
			City:    "Berlin",
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
	if v.Severity != models.SeverityCritical {
		t.Errorf("expected critical severity, got %s", v.Severity)
	}
	if v.ConfidenceScore != 100 {
		t.Errorf("expected 100%% confidence, got %.1f", v.ConfidenceScore)
	}
}

func TestGeoRestrictionEvaluator_InvalidConfig(t *testing.T) {
	e := NewGeoRestrictionEvaluator()
	ctx := context.Background()

	rule := &models.Rule{
		ID:     5,
		Name:   "Invalid",
		Type:   models.RuleTypeGeoRestriction,
		Config: []byte(`{invalid json`),
	}

	input := &EvaluationInput{
		Stream: &models.ActiveStream{
			UserName:  "testuser",
			IPAddress: "1.2.3.4",
		},
		GeoData: &models.GeoResult{
			Country: "US",
		},
	}

	_, err := e.Evaluate(ctx, rule, input)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}
