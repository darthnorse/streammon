package maintenance

import (
	"streammon/internal/models"
)

var (
	minDays   = 1
	maxDays   = 3650
	minHeight = 240
	maxHeight = 4320
	minSizeGB  = 1
	maxSizeGB  = 1000
	minSeasons = 1
	maxSeasons = 100
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
			Name:        "Unwatched TV Shows",
			Description: "TV shows with no watch activity reported by the media server in specified days",
			MediaTypes:  []models.MediaType{models.MediaTypeTV},
			Parameters: []models.ParamSpec{
				{Name: "days", Type: "int", Label: "Days since last watched", Default: DefaultDays, Min: &minDays, Max: &maxDays},
			},
		},
		{
			Type:        models.CriterionLowResolution,
			Name:        "Low Resolution",
			Description: "Items with resolution at or below threshold",
			MediaTypes:  []models.MediaType{models.MediaTypeMovie},
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
		{
			Type:        models.CriterionKeepLatestSeasons,
			Name:        "Keep Latest Seasons",
			Description: "Keep only the latest N seasons of TV shows, optionally filtered by genre",
			MediaTypes:  []models.MediaType{models.MediaTypeTV},
			Parameters: []models.ParamSpec{
				{Name: "keep_seasons", Type: "int", Label: "Seasons to keep", Default: DefaultKeepSeasons, Min: &minSeasons, Max: &maxSeasons},
				{Name: "genre_ids", Type: "genre_multi_select", Label: "Filter by genres (empty = all)", Default: nil},
			},
		},
	}
}
