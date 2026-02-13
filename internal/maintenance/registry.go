package maintenance

import (
	"streammon/internal/models"
)

var (
	minDays    = 1
	maxDays    = 3650
	minPercent = 1
	maxPercent = 100
	minHeight  = 240
	maxHeight  = 4320
	minSizeGB  = 1
	maxSizeGB  = 1000
)

func GetCriterionTypes() []models.CriterionTypeInfo {
	return []models.CriterionTypeInfo{
		{
			Type:        models.CriterionUnwatchedMovie,
			Name:        "Unwatched Movies",
			Description: "Movies never watched or not watched in specified days",
			MediaTypes:  []models.MediaType{models.MediaTypeMovie},
			Parameters: []models.ParamSpec{
				{Name: "days", Type: "int", Label: "Days since last watched", Default: DefaultDays, Min: &minDays, Max: &maxDays},
			},
		},
		{
			Type:        models.CriterionUnwatchedTVNone,
			Name:        "Unwatched TV Shows (Zero Episodes)",
			Description: "TV shows where no episodes have ever been watched",
			MediaTypes:  []models.MediaType{models.MediaTypeTV},
			Parameters: []models.ParamSpec{
				{Name: "days", Type: "int", Label: "Days since last watched", Default: DefaultDays, Min: &minDays, Max: &maxDays},
			},
		},
		{
			Type:        models.CriterionUnwatchedTVLow,
			Name:        "Unwatched TV Shows (Low Watch %)",
			Description: "TV shows with watch percentage below threshold and no recent activity",
			MediaTypes:  []models.MediaType{models.MediaTypeTV},
			Parameters: []models.ParamSpec{
				{Name: "days", Type: "int", Label: "Days since last watched", Default: DefaultDays, Min: &minDays, Max: &maxDays},
				{Name: "max_percent", Type: "int", Label: "Max watch percentage", Default: DefaultMaxPercent, Min: &minPercent, Max: &maxPercent},
			},
		},
		{
			Type:        models.CriterionLowResolution,
			Name:        "Low Resolution",
			Description: "Items with resolution at or below threshold",
			MediaTypes:  []models.MediaType{models.MediaTypeMovie, models.MediaTypeTV},
			Parameters: []models.ParamSpec{
				{Name: "max_height", Type: "int", Label: "Max resolution height", Default: DefaultMaxHeight, Min: &minHeight, Max: &maxHeight},
			},
		},
		{
			Type:        models.CriterionLargeFiles,
			Name:        "Large Files",
			Description: "Items with file size above threshold",
			MediaTypes:  []models.MediaType{models.MediaTypeMovie, models.MediaTypeTV},
			Parameters: []models.ParamSpec{
				{Name: "min_size_gb", Type: "int", Label: "Minimum size (GB)", Default: int(DefaultMinSizeGB), Min: &minSizeGB, Max: &maxSizeGB},
			},
		},
	}
}
