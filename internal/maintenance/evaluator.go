package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

// CandidateResult represents a single evaluation result
type CandidateResult struct {
	LibraryItemID int64
	Reason        string
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
		params.Days = 365
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
		params.Days = 365
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
		// For TV, we look at shows (which have episode_count > 0)
		if item.MediaType != models.MediaTypeTV || item.EpisodeCount == 0 {
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
		params.Days = 365
	}
	if params.MaxPercent <= 0 {
		params.MaxPercent = 10
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
		if item.MediaType != models.MediaTypeTV || item.EpisodeCount == 0 {
			continue
		}
		if item.AddedAt.After(cutoff) {
			continue
		}

		watchedCount, err := e.store.GetWatchedEpisodeCount(ctx, rule.ServerID, item.ItemID)
		if err != nil {
			return nil, err
		}

		watchedPct := float64(watchedCount) / float64(item.EpisodeCount) * 100
		if watchedPct < float64(params.MaxPercent) {
			results = append(results, CandidateResult{
				LibraryItemID: item.ID,
				Reason:        fmt.Sprintf("%.1f%% watched (%d/%d episodes)", watchedPct, watchedCount, item.EpisodeCount),
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
		params.MaxHeight = 720
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
		if height > 0 && height < params.MaxHeight {
			results = append(results, CandidateResult{
				LibraryItemID: item.ID,
				Reason:        fmt.Sprintf("Resolution %s below %dp", item.VideoResolution, params.MaxHeight),
			})
		}
	}
	return results, nil
}

// parseResolutionHeight extracts height from resolution strings like "1080p", "720p", "4K", "480"
func parseResolutionHeight(res string) int {
	switch res {
	case "4K", "4k", "2160p", "2160":
		return 2160
	case "1080p", "1080", "FHD":
		return 1080
	case "720p", "720", "HD":
		return 720
	case "480p", "480", "SD":
		return 480
	case "360p", "360":
		return 360
	case "240p", "240":
		return 240
	default:
		return 0
	}
}
