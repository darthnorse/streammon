package rules

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"streammon/internal/models"
)

func TestConcurrentStreamsEvaluator_Type(t *testing.T) {
	e := NewConcurrentStreamsEvaluator()
	if got := e.Type(); got != models.RuleTypeConcurrentStreams {
		t.Errorf("Type() = %v, want %v", got, models.RuleTypeConcurrentStreams)
	}
}

func TestConcurrentStreamsEvaluator_Evaluate(t *testing.T) {
	e := NewConcurrentStreamsEvaluator()
	ctx := context.Background()

	makeRule := func(maxStreams int) *models.Rule {
		config := models.ConcurrentStreamsConfig{MaxStreams: maxStreams}
		configJSON, _ := json.Marshal(config)
		return &models.Rule{
			ID:     1,
			Name:   "Max 2 Streams",
			Type:   models.RuleTypeConcurrentStreams,
			Config: configJSON,
		}
	}

	makeStreams := func(userName string, count int) []models.ActiveStream {
		streams := make([]models.ActiveStream, count)
		for i := 0; i < count; i++ {
			streams[i] = models.ActiveStream{
				SessionID: string(rune('a' + i)),
				UserName:  userName,
				IPAddress: "192.168.1." + string(rune('1'+i)),
				Player:    "Device" + string(rune('1'+i)),
				Platform:  "Platform" + string(rune('1'+i)),
				StartedAt: time.Now().UTC(),
			}
		}
		return streams
	}

	tests := []struct {
		name        string
		rule        *models.Rule
		input       *EvaluationInput
		wantViolation bool
		wantSeverity  models.Severity
	}{
		{
			name: "nil stream",
			rule: makeRule(2),
			input: &EvaluationInput{
				Stream:     nil,
				AllStreams: []models.ActiveStream{},
			},
			wantViolation: false,
		},
		{
			name: "under limit",
			rule: makeRule(2),
			input: &EvaluationInput{
				Stream:     &models.ActiveStream{UserName: "testuser"},
				AllStreams: makeStreams("testuser", 2),
			},
			wantViolation: false,
		},
		{
			name: "at limit",
			rule: makeRule(2),
			input: &EvaluationInput{
				Stream:     &models.ActiveStream{UserName: "testuser"},
				AllStreams: makeStreams("testuser", 2),
			},
			wantViolation: false,
		},
		{
			name: "over limit by 1 - info",
			rule: makeRule(2),
			input: &EvaluationInput{
				Stream:     &models.ActiveStream{UserName: "testuser"},
				AllStreams: makeStreams("testuser", 3),
			},
			wantViolation: true,
			wantSeverity:  models.SeverityInfo,
		},
		{
			name: "over limit by 2 - warning",
			rule: makeRule(2),
			input: &EvaluationInput{
				Stream:     &models.ActiveStream{UserName: "testuser"},
				AllStreams: makeStreams("testuser", 4),
			},
			wantViolation: true,
			wantSeverity:  models.SeverityWarning,
		},
		{
			name: "over limit by 3+ - critical",
			rule: makeRule(2),
			input: &EvaluationInput{
				Stream:     &models.ActiveStream{UserName: "testuser"},
				AllStreams: makeStreams("testuser", 5),
			},
			wantViolation: true,
			wantSeverity:  models.SeverityCritical,
		},
		{
			name: "other users streams don't count",
			rule: makeRule(2),
			input: &EvaluationInput{
				Stream: &models.ActiveStream{UserName: "testuser"},
				AllStreams: append(
					makeStreams("testuser", 2),
					makeStreams("otheruser", 3)...,
				),
			},
			wantViolation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := e.Evaluate(ctx, tt.rule, tt.input)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}

			gotViolation := result != nil && result.Violation != nil
			if gotViolation != tt.wantViolation {
				t.Errorf("Evaluate() violation = %v, want %v", gotViolation, tt.wantViolation)
			}

			if gotViolation && tt.wantSeverity != "" {
				if result.Violation.Severity != tt.wantSeverity {
					t.Errorf("Evaluate() severity = %v, want %v", result.Violation.Severity, tt.wantSeverity)
				}
			}
		})
	}
}

func TestConcurrentStreamsEvaluator_HouseholdExemption(t *testing.T) {
	e := NewConcurrentStreamsEvaluator()
	ctx := context.Background()

	config := models.ConcurrentStreamsConfig{
		MaxStreams:      2,
		ExemptHousehold: true,
	}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		ID:     1,
		Name:   "Max 2 Streams (Household Exempt)",
		Type:   models.RuleTypeConcurrentStreams,
		Config: configJSON,
	}

	now := time.Now().UTC()
	householdIP := "192.168.1.100"

	streams := []models.ActiveStream{
		{SessionID: "a", UserName: "testuser", IPAddress: householdIP, StartedAt: now},
		{SessionID: "b", UserName: "testuser", IPAddress: householdIP, StartedAt: now},
		{SessionID: "c", UserName: "testuser", IPAddress: householdIP, StartedAt: now},
	}

	households := []models.HouseholdLocation{
		{UserName: "testuser", IPAddress: householdIP, Trusted: true},
	}

	input := &EvaluationInput{
		Stream:     &streams[0],
		AllStreams: streams,
		Households: households,
	}

	result, err := e.Evaluate(ctx, rule, input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result != nil && result.Violation != nil {
		t.Error("expected no violation when all streams from household")
	}

	streams[2].IPAddress = "10.0.0.1"
	input.AllStreams = streams

	result, err = e.Evaluate(ctx, rule, input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result == nil || result.Violation == nil {
		t.Error("expected violation when stream from outside household")
	}
}

func TestConcurrentStreamsEvaluator_ViolationDetails(t *testing.T) {
	e := NewConcurrentStreamsEvaluator()
	ctx := context.Background()

	config := models.ConcurrentStreamsConfig{MaxStreams: 1}
	configJSON, _ := json.Marshal(config)
	rule := &models.Rule{
		ID:     1,
		Name:   "Max 1 Stream",
		Type:   models.RuleTypeConcurrentStreams,
		Config: configJSON,
	}

	now := time.Now().UTC()
	streams := []models.ActiveStream{
		{SessionID: "a", UserName: "testuser", IPAddress: "192.168.1.1", Player: "Chrome", Platform: "Windows", StartedAt: now},
		{SessionID: "b", UserName: "testuser", IPAddress: "10.0.0.1", Player: "TV App", Platform: "Samsung TV", StartedAt: now},
	}

	input := &EvaluationInput{
		Stream:     &streams[0],
		AllStreams: streams,
	}

	result, err := e.Evaluate(ctx, rule, input)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result == nil || result.Violation == nil {
		t.Fatal("expected violation")
	}

	v := result.Violation

	if v.UserName != "testuser" {
		t.Errorf("UserName = %q, want %q", v.UserName, "testuser")
	}

	if v.RuleID != rule.ID {
		t.Errorf("RuleID = %d, want %d", v.RuleID, rule.ID)
	}

	streamCount, ok := v.Details["stream_count"].(int)
	if !ok || streamCount != 2 {
		t.Errorf("stream_count = %v, want 2", v.Details["stream_count"])
	}

	locations, ok := v.Details["locations"].([]string)
	if !ok || len(locations) != 2 {
		t.Errorf("locations = %v, want 2 items", v.Details["locations"])
	}

	devices, ok := v.Details["devices"].([]string)
	if !ok || len(devices) != 2 {
		t.Errorf("devices = %v, want 2 items", v.Details["devices"])
	}

	if v.ConfidenceScore < 50 || v.ConfidenceScore > 100 {
		t.Errorf("ConfidenceScore = %f, want between 50 and 100", v.ConfidenceScore)
	}

	if len(result.Signals) == 0 {
		t.Error("expected signals to be populated")
	}
}

func TestConcurrentStreamsEvaluator_InvalidConfig(t *testing.T) {
	e := NewConcurrentStreamsEvaluator()
	ctx := context.Background()

	rule := &models.Rule{
		ID:     1,
		Name:   "Bad Config",
		Type:   models.RuleTypeConcurrentStreams,
		Config: json.RawMessage(`{"invalid": "json`),
	}

	input := &EvaluationInput{
		Stream:     &models.ActiveStream{UserName: "test"},
		AllStreams: []models.ActiveStream{{UserName: "test"}},
	}

	_, err := e.Evaluate(ctx, rule, input)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}
