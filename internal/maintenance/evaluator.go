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

// Default parameter values (centralized)
const (
	DefaultDays       = 365
	DefaultMaxPercent = 10
	DefaultMaxHeight  = 720
	DefaultMinSizeGB  = 10.0
)

// CandidateResult represents a single evaluation result
type CandidateResult struct {
	LibraryItemID int64
	Reason        string
}

// ToBatch converts a slice of CandidateResults to BatchCandidates for store operations
func ToBatch(candidates []CandidateResult) []models.BatchCandidate {
	batch := make([]models.BatchCandidate, len(candidates))
	for i, c := range candidates {
		batch[i] = models.BatchCandidate{
			LibraryItemID: c.LibraryItemID,
			Reason:        c.Reason,
		}
	}
	return batch
}

// Evaluator evaluates a rule against library items
type Evaluator struct {
	store *store.Store
}

// NewEvaluator creates a new evaluator
func NewEvaluator(s *store.Store) *Evaluator {
	return &Evaluator{store: s}
}

// EvaluateRule evaluates a rule and returns matching candidates
func (e *Evaluator) EvaluateRule(ctx context.Context, rule *models.MaintenanceRule) ([]CandidateResult, error) {
	switch rule.CriterionType {
	case models.CriterionUnwatchedMovie:
		return e.evaluateUnwatchedMovie(ctx, rule)
	case models.CriterionUnwatchedTVNone:
		return e.evaluateUnwatchedTVNone(ctx, rule)
	case models.CriterionUnwatchedTVLow:
		return e.evaluateUnwatchedTVLow(ctx, rule)
	case models.CriterionLowResolution:
		return e.evaluateLowResolution(ctx, rule)
	case models.CriterionLargeFiles:
		return e.evaluateLargeFiles(ctx, rule)
	default:
		return nil, fmt.Errorf("unknown criterion type: %s", rule.CriterionType)
	}
}

func (e *Evaluator) evaluateUnwatchedMovie(ctx context.Context, rule *models.MaintenanceRule) ([]CandidateResult, error) {
	var params models.UnwatchedMovieParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if params.Days <= 0 {
		params.Days = DefaultDays
	}

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -params.Days)
	items, err := e.store.ListLibraryItems(ctx, rule.ServerID, rule.LibraryID)
	if err != nil {
		return nil, err
	}

	var results []CandidateResult
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if item.MediaType != models.MediaTypeMovie {
			continue
		}
		if item.AddedAt.After(cutoff) {
			continue
		}

		watched, err := e.store.IsItemWatched(ctx, rule.ServerID, item.ItemID)
		if err != nil {
			return nil, err
		}
		if !watched {
			days := int(now.Sub(item.AddedAt).Hours() / 24)
			results = append(results, CandidateResult{
				LibraryItemID: item.ID,
				Reason:        fmt.Sprintf("Unwatched for %d days", days),
			})
		}
	}
	return results, nil
}

func (e *Evaluator) evaluateUnwatchedTVNone(ctx context.Context, rule *models.MaintenanceRule) ([]CandidateResult, error) {
	var params models.UnwatchedTVNoneParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if params.Days <= 0 {
		params.Days = DefaultDays
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -params.Days)
	items, err := e.store.ListLibraryItems(ctx, rule.ServerID, rule.LibraryID)
	if err != nil {
		return nil, err
	}

	var results []CandidateResult
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// EpisodeCount may be 0 for some media servers (e.g., Emby), so we only check media type
		if item.MediaType != models.MediaTypeTV {
			continue
		}
		if item.AddedAt.After(cutoff) {
			continue
		}

		watched, err := e.store.IsItemWatched(ctx, rule.ServerID, item.ItemID)
		if err != nil {
			return nil, err
		}
		if !watched {
			results = append(results, CandidateResult{
				LibraryItemID: item.ID,
				Reason:        "No episodes watched",
			})
		}
	}
	return results, nil
}

func (e *Evaluator) evaluateUnwatchedTVLow(ctx context.Context, rule *models.MaintenanceRule) ([]CandidateResult, error) {
	var params models.UnwatchedTVLowParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if params.Days <= 0 {
		params.Days = DefaultDays
	}
	if params.MaxPercent <= 0 {
		params.MaxPercent = DefaultMaxPercent
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -params.Days)
	items, err := e.store.ListLibraryItems(ctx, rule.ServerID, rule.LibraryID)
	if err != nil {
		return nil, err
	}

	var results []CandidateResult
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if item.MediaType != models.MediaTypeTV {
			continue
		}
		if item.AddedAt.After(cutoff) {
			continue
		}

		watchedCount, err := e.store.GetWatchedEpisodeCount(ctx, rule.ServerID, item.ItemID)
		if err != nil {
			return nil, err
		}

		// If episode count is 0 (e.g., Emby doesn't provide it), fall back to
		// treating 0 watched as "low percentage"
		if item.EpisodeCount > 0 {
			watchedPct := float64(watchedCount) / float64(item.EpisodeCount) * 100
			if watchedPct < float64(params.MaxPercent) {
				results = append(results, CandidateResult{
					LibraryItemID: item.ID,
					Reason:        fmt.Sprintf("%.1f%% watched (%d/%d episodes)", watchedPct, watchedCount, item.EpisodeCount),
				})
			}
		} else if watchedCount == 0 {
			results = append(results, CandidateResult{
				LibraryItemID: item.ID,
				Reason:        "No episodes watched (episode count unknown)",
			})
		}
	}
	return results, nil
}

func (e *Evaluator) evaluateLowResolution(ctx context.Context, rule *models.MaintenanceRule) ([]CandidateResult, error) {
	var params models.LowResolutionParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if params.MaxHeight <= 0 {
		params.MaxHeight = DefaultMaxHeight
	}

	items, err := e.store.ListLibraryItems(ctx, rule.ServerID, rule.LibraryID)
	if err != nil {
		return nil, err
	}

	var results []CandidateResult
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if item.VideoResolution == "" {
			continue
		}

		height := parseResolutionHeight(item.VideoResolution)
		if height > 0 && height <= params.MaxHeight {
			results = append(results, CandidateResult{
				LibraryItemID: item.ID,
				Reason:        fmt.Sprintf("Resolution %s below %dp", item.VideoResolution, params.MaxHeight),
			})
		}
	}
	return results, nil
}

func (e *Evaluator) evaluateLargeFiles(ctx context.Context, rule *models.MaintenanceRule) ([]CandidateResult, error) {
	var params models.LargeFilesParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}
	if params.MinSizeGB <= 0 {
		params.MinSizeGB = DefaultMinSizeGB
	}

	minSizeBytes := int64(params.MinSizeGB * 1024 * 1024 * 1024)

	items, err := e.store.ListLibraryItems(ctx, rule.ServerID, rule.LibraryID)
	if err != nil {
		return nil, err
	}

	var results []CandidateResult
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if item.FileSize <= 0 {
			continue
		}

		if item.FileSize >= minSizeBytes {
			sizeGB := float64(item.FileSize) / (1024 * 1024 * 1024)
			results = append(results, CandidateResult{
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
	// Normalize to lowercase for case-insensitive matching
	lower := strings.ToLower(res)

	// Handle named resolutions
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

	// Try to parse numeric resolution (e.g., "1080p", "720", "576p")
	if matches := resolutionRegex.FindStringSubmatch(lower); len(matches) == 2 {
		if height, err := strconv.Atoi(matches[1]); err == nil {
			return height
		}
	}

	return 0
}
