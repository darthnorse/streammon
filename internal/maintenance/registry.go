package maintenance

import (
	"streammon/internal/models"
)

// GetCriterionTypes returns all available criterion types
func GetCriterionTypes() []models.CriterionTypeInfo {
	minDays := 1
	maxDays := 3650
	minPercent := 1
	maxPercent := 100
	minHeight := 240
	maxHeight := 4320

	return []models.CriterionTypeInfo{
		{
			Type:        models.CriterionUnwatchedMovie,
			Name:        "Unwatched Movies",
			Description: "Movies not watched by anyone, older than specified days",
			MediaTypes:  []models.MediaType{models.MediaTypeMovie},
			Parameters: []models.ParamSpec{
				{Name: "days", Type: "int", Label: "Days since added", Default: 365, Min: &minDays, Max: &maxDays},
			},
		},
		{
			Type:        models.CriterionUnwatchedTVNone,
			Name:        "Unwatched TV Shows (Zero Episodes)",
			Description: "TV shows where no episodes have been watched",
			MediaTypes:  []models.MediaType{models.MediaTypeTV},
			Parameters: []models.ParamSpec{
				{Name: "days", Type: "int", Label: "Days since added", Default: 365, Min: &minDays, Max: &maxDays},
			},
		},
		{
			Type:        models.CriterionUnwatchedTVLow,
			Name:        "Unwatched TV Shows (Low Watch %)",
			Description: "TV shows with watch percentage below threshold",
			MediaTypes:  []models.MediaType{models.MediaTypeTV},
			Parameters: []models.ParamSpec{
				{Name: "days", Type: "int", Label: "Days since added", Default: 365, Min: &minDays, Max: &maxDays},
				{Name: "max_percent", Type: "int", Label: "Max watch percentage", Default: 10, Min: &minPercent, Max: &maxPercent},
			},
		},
		{
			Type:        models.CriterionLowResolution,
			Name:        "Low Resolution",
			Description: "Items with resolution below threshold",
			MediaTypes:  []models.MediaType{models.MediaTypeMovie, models.MediaTypeTV},
			Parameters: []models.ParamSpec{
				{Name: "max_height", Type: "int", Label: "Max resolution height", Default: 720, Min: &minHeight, Max: &maxHeight},
			},
		},
	}
}
