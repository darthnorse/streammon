package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCriterionTypeValid(t *testing.T) {
	validTypes := []CriterionType{
		CriterionUnwatchedMovie,
		CriterionUnwatchedTVNone,
		CriterionLowResolution,
		CriterionLargeFiles,
	}

	for _, ct := range validTypes {
		if !ct.Valid() {
			t.Errorf("CriterionType(%q).Valid() = false, want true", ct)
		}
	}

	invalidTypes := []CriterionType{"invalid", "", "unknown_type"}
	for _, ct := range invalidTypes {
		if ct.Valid() {
			t.Errorf("CriterionType(%q).Valid() = true, want false", ct)
		}
	}
}

func TestMaintenanceRuleInputValidate(t *testing.T) {
	libs := []RuleLibrary{{ServerID: 1, LibraryID: "lib1"}}

	tests := []struct {
		name    string
		input   MaintenanceRuleInput
		wantErr string
	}{
		{
			name: "valid input",
			input: MaintenanceRuleInput{
				Name:          "Test Rule",
				MediaType:     MediaTypeMovie,
				CriterionType: CriterionUnwatchedMovie,
				Parameters:    json.RawMessage(`{"days": 30}`),
				Enabled:       true,
				Libraries:     libs,
			},
			wantErr: "",
		},
		{
			name: "missing libraries",
			input: MaintenanceRuleInput{
				Name:          "Test Rule",
				MediaType:     MediaTypeMovie,
				CriterionType: CriterionUnwatchedMovie,
			},
			wantErr: "at least one library is required",
		},
		{
			name: "invalid media type",
			input: MaintenanceRuleInput{
				Name:          "Test Rule",
				MediaType:     "invalid",
				CriterionType: CriterionUnwatchedMovie,
				Libraries:     libs,
			},
			wantErr: "media_type must be movie or episode",
		},
		{
			name: "missing name",
			input: MaintenanceRuleInput{
				MediaType:     MediaTypeMovie,
				CriterionType: CriterionUnwatchedMovie,
				Libraries:     libs,
			},
			wantErr: "name is required",
		},
		{
			name: "name too long",
			input: MaintenanceRuleInput{
				Name:          strings.Repeat("a", 256),
				MediaType:     MediaTypeMovie,
				CriterionType: CriterionUnwatchedMovie,
				Libraries:     libs,
			},
			wantErr: "name must be 255 characters or less",
		},
		{
			name: "invalid criterion type",
			input: MaintenanceRuleInput{
				Name:          "Test Rule",
				MediaType:     MediaTypeMovie,
				CriterionType: "invalid_type",
				Libraries:     libs,
			},
			wantErr: "invalid criterion type",
		},
		{
			name: "empty parameters get default",
			input: MaintenanceRuleInput{
				Name:          "Test Rule",
				MediaType:     MediaTypeMovie,
				CriterionType: CriterionUnwatchedMovie,
				Parameters:    nil,
				Libraries:     libs,
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() error = %v, wantErr nil", err)
				}
				// Check that empty parameters get default
				if tt.input.Parameters == nil || len(tt.input.Parameters) == 0 {
					t.Error("Parameters should be set to {} after validation")
				}
			} else {
				if err == nil {
					t.Errorf("Validate() error = nil, wantErr %q", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %v, wantErr containing %q", err, tt.wantErr)
				}
			}
		})
	}
}

func TestMaintenanceRuleUpdateInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   MaintenanceRuleUpdateInput
		wantErr string
	}{
		{
			name: "valid input",
			input: MaintenanceRuleUpdateInput{
				Name:          "Test Rule",
				CriterionType: CriterionUnwatchedMovie,
				Parameters:    json.RawMessage(`{"days": 30}`),
				Enabled:       true,
			},
			wantErr: "",
		},
		{
			name: "missing name",
			input: MaintenanceRuleUpdateInput{
				CriterionType: CriterionUnwatchedMovie,
			},
			wantErr: "name is required",
		},
		{
			name: "invalid criterion type",
			input: MaintenanceRuleUpdateInput{
				Name:          "Test Rule",
				CriterionType: "invalid_type",
			},
			wantErr: "invalid criterion type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() error = %v, wantErr nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() error = nil, wantErr %q", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %v, wantErr containing %q", err, tt.wantErr)
				}
			}
		})
	}
}
