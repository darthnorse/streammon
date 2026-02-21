package maintenance

import (
	"testing"

	"streammon/internal/models"
)

func TestGetCriterionTypes(t *testing.T) {
	types := GetCriterionTypes()

	// Should have 5 criterion types
	if len(types) != 5 {
		t.Errorf("GetCriterionTypes() returned %d types, want 5", len(types))
	}

	// Check each type exists
	expectedTypes := map[models.CriterionType]bool{
		models.CriterionUnwatchedMovie:    false,
		models.CriterionUnwatchedTVNone:   false,
		models.CriterionLowResolution:     false,
		models.CriterionLargeFiles:        false,
		models.CriterionKeepLatestSeasons: false,
	}

	for _, ct := range types {
		if _, ok := expectedTypes[ct.Type]; !ok {
			t.Errorf("Unexpected criterion type: %s", ct.Type)
		}
		expectedTypes[ct.Type] = true

		// Each type should have a name and description
		if ct.Name == "" {
			t.Errorf("Criterion type %s has empty name", ct.Type)
		}
		if ct.Description == "" {
			t.Errorf("Criterion type %s has empty description", ct.Type)
		}

		// Each type should have at least one media type
		if len(ct.MediaTypes) == 0 {
			t.Errorf("Criterion type %s has no media types", ct.Type)
		}

		// Each type should have at least one parameter
		if len(ct.Parameters) == 0 {
			t.Errorf("Criterion type %s has no parameters", ct.Type)
		}

		// Verify parameters have required fields
		for _, param := range ct.Parameters {
			if param.Name == "" {
				t.Errorf("Criterion type %s has parameter with empty name", ct.Type)
			}
			if param.Type == "" {
				t.Errorf("Criterion type %s parameter %s has empty type", ct.Type, param.Name)
			}
			if param.Label == "" {
				t.Errorf("Criterion type %s parameter %s has empty label", ct.Type, param.Name)
			}
			if param.Default == nil && param.Type != "genre_multi_select" {
				t.Errorf("Criterion type %s parameter %s has nil default", ct.Type, param.Name)
			}
		}
	}

	// Verify all expected types were found
	for ct, found := range expectedTypes {
		if !found {
			t.Errorf("Expected criterion type %s not found", ct)
		}
	}
}

func TestDefaultsUsedInRegistry(t *testing.T) {
	types := GetCriterionTypes()

	for _, ct := range types {
		for _, param := range ct.Parameters {
			switch param.Name {
			case "days":
				if param.Default != DefaultDays {
					t.Errorf("Criterion %s parameter 'days' default = %v, want %d", ct.Type, param.Default, DefaultDays)
				}
			case "max_height":
				if param.Default != DefaultMaxHeight {
					t.Errorf("Criterion %s parameter 'max_height' default = %v, want %d", ct.Type, param.Default, DefaultMaxHeight)
				}
			case "min_size_gb":
				if param.Default != int(DefaultMinSizeGB) {
					t.Errorf("Criterion %s parameter 'min_size_gb' default = %v, want %d", ct.Type, param.Default, int(DefaultMinSizeGB))
				}
			}
		}
	}
}
