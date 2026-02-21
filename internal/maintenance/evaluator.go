package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"streammon/internal/media"
	"streammon/internal/mediautil"
	"streammon/internal/models"
	"streammon/internal/store"
	"streammon/internal/tmdb"
)

const (
	DefaultDays        = 365
	DefaultMaxHeight   = 720
	DefaultMinSizeGB   = 10.0
	DefaultKeepSeasons = 3
)

type MediaServerResolver interface {
	GetServer(serverID int64) (media.MediaServer, bool)
}

type Evaluator struct {
	store   *store.Store
	tmdb    *tmdb.Client
	servers MediaServerResolver
}

func NewEvaluator(s *store.Store, tmdb *tmdb.Client, servers MediaServerResolver) *Evaluator {
	return &Evaluator{store: s, tmdb: tmdb, servers: servers}
}

func countByMediaType(items []models.LibraryItemCache, mt models.MediaType) int {
	n := 0
	for _, item := range items {
		if item.MediaType == mt {
			n++
		}
	}
	return n
}

func getItemRefTime(item models.LibraryItemCache) (time.Time, bool) {
	if item.LastWatchedAt != nil {
		return *item.LastWatchedAt, true
	}
	return item.AddedAt, false
}

func (e *Evaluator) EvaluateRule(ctx context.Context, rule *models.MaintenanceRule) ([]models.BatchCandidate, error) {
	var candidates []models.BatchCandidate
	var items []models.LibraryItemCache
	var err error

	switch rule.CriterionType {
	case models.CriterionUnwatchedMovie:
		candidates, items, err = e.evaluateUnwatched(ctx, rule, models.MediaTypeMovie,
			"Not watched in %d days", "Never watched (added %d days ago)")
	case models.CriterionUnwatchedTVNone:
		candidates, items, err = e.evaluateUnwatched(ctx, rule, models.MediaTypeTV,
			"Last watched %d days ago", "Never watched (%d days inactive)")
	case models.CriterionLowResolution:
		candidates, items, err = e.evaluateLowResolution(ctx, rule)
	case models.CriterionLargeFiles:
		candidates, items, err = e.evaluateLargeFiles(ctx, rule)
	case models.CriterionKeepLatestSeasons:
		candidates, items, err = e.evaluateKeepLatestSeasons(ctx, rule)
	default:
		return nil, fmt.Errorf("unknown criterion type: %s", rule.CriterionType)
	}
	if err != nil {
		return nil, err
	}
	return deduplicateCandidates(candidates, items), nil
}

type unwatchedParams struct {
	Days int `json:"days"`
}

func (e *Evaluator) evaluateUnwatched(ctx context.Context, rule *models.MaintenanceRule, mediaType models.MediaType, watchedFmt, neverFmt string) ([]models.BatchCandidate, []models.LibraryItemCache, error) {
	var params unwatchedParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		return nil, nil, fmt.Errorf("parse params: %w", err)
	}
	if params.Days <= 0 {
		params.Days = DefaultDays
	}

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -params.Days)
	items, err := e.store.ListItemsForLibraries(ctx, rule.Libraries)
	if err != nil {
		return nil, nil, err
	}

	itemIDs := make([]int64, len(items))
	for i, item := range items {
		itemIDs[i] = item.ID
	}

	watchTimes, err := e.store.GetCrossServerWatchTimes(ctx, itemIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("cross-server watch times: %w", err)
	}

	for i := range items {
		if t, ok := watchTimes[items[i].ID]; ok && t != nil {
			if items[i].LastWatchedAt == nil || t.After(*items[i].LastWatchedAt) {
				items[i].LastWatchedAt = t
			}
		}
	}

	// Merge StreamMon's own watch_history which captures ALL users' sessions,
	// not just the API user whose watch data the media server reports.
	smTimes, err := e.store.GetStreamMonWatchTimes(ctx, itemIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("streammon watch times: %w", err)
	}

	for i := range items {
		if t, ok := smTimes[items[i].ID]; ok && t != nil {
			if items[i].LastWatchedAt == nil || t.After(*items[i].LastWatchedAt) {
				items[i].LastWatchedAt = t
			}
		}
	}

	total := countByMediaType(items, mediaType)

	var results []models.BatchCandidate
	processed := 0
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		if item.MediaType != mediaType {
			continue
		}

		processed++
		mediautil.SendProgress(ctx, mediautil.SyncProgress{
			Phase:   mediautil.PhaseEvaluating,
			Current: processed,
			Total:   total,
			Library: item.LibraryID,
		})

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
	return results, items, nil
}

func (e *Evaluator) evaluateLowResolution(ctx context.Context, rule *models.MaintenanceRule) ([]models.BatchCandidate, []models.LibraryItemCache, error) {
	var params models.LowResolutionParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		return nil, nil, fmt.Errorf("parse params: %w", err)
	}
	if params.MaxHeight <= 0 {
		params.MaxHeight = DefaultMaxHeight
	}

	items, err := e.store.ListItemsForLibraries(ctx, rule.Libraries)
	if err != nil {
		return nil, nil, err
	}

	total := countByMediaType(items, rule.MediaType)

	var results []models.BatchCandidate
	processed := 0
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		if item.MediaType != rule.MediaType {
			continue
		}

		processed++
		mediautil.SendProgress(ctx, mediautil.SyncProgress{
			Phase:   mediautil.PhaseEvaluating,
			Current: processed,
			Total:   total,
			Library: item.LibraryID,
		})

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
	return results, items, nil
}

func (e *Evaluator) evaluateLargeFiles(ctx context.Context, rule *models.MaintenanceRule) ([]models.BatchCandidate, []models.LibraryItemCache, error) {
	var params models.LargeFilesParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		return nil, nil, fmt.Errorf("parse params: %w", err)
	}
	if params.MinSizeGB <= 0 {
		params.MinSizeGB = DefaultMinSizeGB
	}

	minSizeBytes := int64(params.MinSizeGB * 1024 * 1024 * 1024)

	items, err := e.store.ListItemsForLibraries(ctx, rule.Libraries)
	if err != nil {
		return nil, nil, err
	}

	total := countByMediaType(items, rule.MediaType)

	var results []models.BatchCandidate
	processed := 0
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		if item.MediaType != rule.MediaType {
			continue
		}

		processed++
		mediautil.SendProgress(ctx, mediautil.SyncProgress{
			Phase:   mediautil.PhaseEvaluating,
			Current: processed,
			Total:   total,
			Library: item.LibraryID,
		})

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
	return results, items, nil
}

func (e *Evaluator) evaluateKeepLatestSeasons(ctx context.Context, rule *models.MaintenanceRule) ([]models.BatchCandidate, []models.LibraryItemCache, error) {
	var params models.KeepLatestSeasonsParams
	if err := json.Unmarshal(rule.Parameters, &params); err != nil {
		return nil, nil, fmt.Errorf("parse params: %w", err)
	}
	if params.KeepSeasons <= 0 {
		params.KeepSeasons = DefaultKeepSeasons
	}

	items, err := e.store.ListItemsForLibraries(ctx, rule.Libraries)
	if err != nil {
		return nil, nil, err
	}

	genreFilter := make(map[int]bool, len(params.GenreIDs))
	for _, gid := range params.GenreIDs {
		genreFilter[gid] = true
	}

	total := countByMediaType(items, models.MediaTypeTV)

	var results []models.BatchCandidate
	processed := 0
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		if item.MediaType != models.MediaTypeTV {
			continue
		}

		processed++
		mediautil.SendProgress(ctx, mediautil.SyncProgress{
			Phase:   mediautil.PhaseEvaluating,
			Current: processed,
			Total:   total,
			Library: item.LibraryID,
		})

		if len(genreFilter) > 0 {
			if item.TMDBID == "" {
				continue // unknown genre, skip when filtering
			}
			if e.tmdb != nil {
				tmdbID, parseErr := strconv.Atoi(item.TMDBID)
				if parseErr != nil {
					continue
				}
				raw, tmdbErr := e.tmdb.GetTV(ctx, tmdbID)
				if tmdbErr != nil {
					log.Printf("keep_latest_seasons: tmdb lookup for %q (id=%d): %v", item.Title, tmdbID, tmdbErr)
					continue // can't verify genre, skip to be safe
				}
				var parsed struct {
					Genres []struct {
						ID int `json:"id"`
					} `json:"genres"`
				}
				if jsonErr := json.Unmarshal(raw, &parsed); jsonErr == nil {
					matched := false
					for _, g := range parsed.Genres {
						if genreFilter[g.ID] {
							matched = true
							break
						}
					}
					if !matched {
						continue
					}
				}
			}
		}

		if e.servers == nil {
			continue
		}

		ms, ok := e.servers.GetServer(item.ServerID)
		if !ok {
			continue
		}

		seasons, seasonsErr := ms.GetSeasons(ctx, item.ItemID)
		if seasonsErr != nil {
			log.Printf("keep_latest_seasons: get seasons for %q (item=%s): %v", item.Title, item.ItemID, seasonsErr)
			continue
		}

		regularCount := 0
		for _, s := range seasons {
			if s.Number > 0 {
				regularCount++
			}
		}

		if regularCount > params.KeepSeasons {
			results = append(results, models.BatchCandidate{
				LibraryItemID: item.ID,
				Reason:        fmt.Sprintf("%d seasons \u2014 keeping latest %d", regularCount, params.KeepSeasons),
			})
		}
	}
	return results, items, nil
}

// Items sharing any key represent the same movie/show.
func externalIDKeys(item *models.LibraryItemCache) []string {
	var keys []string
	if item.TMDBID != "" {
		keys = append(keys, "tmdb:"+item.TMDBID)
	}
	if item.IMDBID != "" {
		keys = append(keys, "imdb:"+item.IMDBID)
	}
	if item.TVDBID != "" {
		keys = append(keys, "tvdb:"+item.TVDBID)
	}
	return keys
}

// Ordering depends on ListItemsForLibraries (added_at DESC), so which copy
// survives is determined by the most recently added item.
func deduplicateCandidates(candidates []models.BatchCandidate, items []models.LibraryItemCache) []models.BatchCandidate {
	itemMap := make(map[int64]*models.LibraryItemCache, len(items))
	for i := range items {
		itemMap[items[i].ID] = &items[i]
	}

	seen := make(map[string]bool)
	result := make([]models.BatchCandidate, 0, len(candidates))
	for _, c := range candidates {
		item := itemMap[c.LibraryItemID]
		if item == nil {
			result = append(result, c)
			continue
		}
		keys := externalIDKeys(item)
		if len(keys) == 0 {
			result = append(result, c)
			continue
		}
		duplicate := false
		for _, k := range keys {
			if seen[k] {
				duplicate = true
				break
			}
		}
		if !duplicate {
			for _, k := range keys {
				seen[k] = true
			}
			result = append(result, c)
		}
	}
	return result
}

var resolutionRegex = regexp.MustCompile(`^(\d+)p?$`)

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
