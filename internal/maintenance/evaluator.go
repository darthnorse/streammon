package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

const (
	DefaultDays      = 365
	DefaultMaxHeight = 720
	DefaultMinSizeGB = 10.0
)

type Evaluator struct {
	store *store.Store
}

func NewEvaluator(s *store.Store) *Evaluator {
	return &Evaluator{store: s}
}

// getItemRefTime returns the last-watched time for an item, or AddedAt if never watched.
func getItemRefTime(item models.LibraryItemCache) (time.Time, bool) {
	if item.LastWatchedAt != nil {
		return *item.LastWatchedAt, true
	}
	return item.AddedAt, false
}

func (e *Evaluator) EvaluateRule(ctx context.Context, rule *models.MaintenanceRule) ([]models.BatchCandidate, error) {
	switch rule.CriterionType {
	case models.CriterionUnwatchedMovie:
		return e.evaluateUnwatched(ctx, rule, models.MediaTypeMovie,
			"Not watched in %d days", "Never watched (added %d days ago)")
	case models.CriterionUnwatchedTVNone:
		return e.evaluateUnwatched(ctx, rule, models.MediaTypeTV,
			"Last watched %d days ago", "Never watched (%d days inactive)")
	case models.CriterionLowResolution:
		return e.evaluateLowResolution(ctx, rule)
	case models.CriterionLargeFiles:
		return e.evaluateLargeFiles(ctx, rule)
	default:
		return nil, fmt.Errorf("unknown criterion type: %s", rule.CriterionType)
	}
}

type unwatchedParams struct {
	Days int `json:"days"`
}

func (e *Evaluator) evaluateUnwatched(ctx context.Context, rule *models.MaintenanceRule, mediaType models.MediaType, watchedFmt, neverFmt string) ([]models.BatchCandidate, error) {
	var params unwatchedParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if params.Days <= 0 {
		params.Days = DefaultDays
	}

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -params.Days)
	items, err := e.store.ListItemsForLibraries(ctx, rule.Libraries)
	if err != nil {
		return nil, err
	}

	// Collect item IDs for cross-server watch lookup
	itemIDs := make([]int64, len(items))
	for i, item := range items {
		itemIDs[i] = item.ID
	}

	watchTimes, err := e.store.GetCrossServerWatchTimes(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("cross-server watch times: %w", err)
	}

	// Merge cross-server watch times into items
	for i := range items {
		if t, ok := watchTimes[items[i].ID]; ok && t != nil {
			if items[i].LastWatchedAt == nil || t.After(*items[i].LastWatchedAt) {
				items[i].LastWatchedAt = t
			}
		}
	}

	var results []models.BatchCandidate
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if item.MediaType != mediaType {
			continue
		}

		refTime, wasWatched := getItemRefTime(item)
		if refTime.After(cutoff) {
			continue
		}

		days := int(now.Sub(refTime).Hours() / 24)
		if wasWatched {
			results = append(results, models.BatchCandidate{
				LibraryItemID: item.ID,
				Reason:        fmt.Sprintf(watchedFmt, days),
			})
		} else {
			results = append(results, models.BatchCandidate{
				LibraryItemID: item.ID,
				Reason:        fmt.Sprintf(neverFmt, days),
			})
		}
	}
	return results, nil
}

func (e *Evaluator) evaluateLowResolution(ctx context.Context, rule *models.MaintenanceRule) ([]models.BatchCandidate, error) {
	var params models.LowResolutionParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if params.MaxHeight <= 0 {
		params.MaxHeight = DefaultMaxHeight
	}

	items, err := e.store.ListItemsForLibraries(ctx, rule.Libraries)
	if err != nil {
		return nil, err
	}

	var results []models.BatchCandidate
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if item.MediaType != rule.MediaType {
			continue
		}
		if item.VideoResolution == "" {
			continue
		}

		height := parseResolutionHeight(item.VideoResolution)
		if height > 0 && height <= params.MaxHeight {
			results = append(results, models.BatchCandidate{
				LibraryItemID: item.ID,
				Reason:        fmt.Sprintf("Resolution %dp (at or below %dp)", height, params.MaxHeight),
			})
		}
	}
	return results, nil
}

func (e *Evaluator) evaluateLargeFiles(ctx context.Context, rule *models.MaintenanceRule) ([]models.BatchCandidate, error) {
	var params models.LargeFilesParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if params.MinSizeGB <= 0 {
		params.MinSizeGB = DefaultMinSizeGB
	}

	minSizeBytes := int64(params.MinSizeGB * 1024 * 1024 * 1024)

	items, err := e.store.ListItemsForLibraries(ctx, rule.Libraries)
	if err != nil {
		return nil, err
	}

	var results []models.BatchCandidate
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if item.MediaType != rule.MediaType {
			continue
		}
		if item.FileSize <= 0 {
			continue
		}

		if item.FileSize >= minSizeBytes {
			sizeGB := float64(item.FileSize) / (1024 * 1024 * 1024)
			results = append(results, models.BatchCandidate{
				LibraryItemID: item.ID,
				Reason:        fmt.Sprintf("File size %.1f GB exceeds %.1f GB", sizeGB, params.MinSizeGB),
			})
		}
	}
	return results, nil
}

// resolutionRegex matches resolution strings like "576p", "1080p", "720"
var resolutionRegex = regexp.MustCompile(`^(\d+)p?$`)

// parseResolutionHeight extracts height from resolution strings like "1080p", "720p", "4K", "480", "576p"
func parseResolutionHeight(res string) int {
	lower := strings.ToLower(res)

	switch lower {
	case "4k", "uhd":
		return 2160
	case "8k":
		return 4320
	case "fhd":
		return 1080
	case "hd":
		return 720
	case "sd":
		return 480
	}

	if matches := resolutionRegex.FindStringSubmatch(lower); len(matches) == 2 {
		if height, err := strconv.Atoi(matches[1]); err == nil {
			return height
		}
	}

	return 0
}
