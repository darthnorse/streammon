package maintenance

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"streammon/internal/models"
	"streammon/internal/overseerr"
	"streammon/internal/radarr"
	"streammon/internal/sonarr"
	"streammon/internal/store"
)

const cascadeTimeout = 15 * time.Second

// CascadeResult records the outcome of a single external service cleanup.
type CascadeResult struct {
	Service string
	Success bool
	Error   string
}

// CascadeDeleter coordinates deletion of items from external services
// (Radarr, Sonarr, Overseerr/Seerr) after a media server delete.
type CascadeDeleter struct {
	store *store.Store
}

func NewCascadeDeleter(s *store.Store) *CascadeDeleter {
	return &CascadeDeleter{store: s}
}

// DeleteExternalReferences removes the item from configured external services.
// All operations are best-effort and run concurrently.
func (cd *CascadeDeleter) DeleteExternalReferences(ctx context.Context, item *models.LibraryItemCache) []CascadeResult {
	type indexedResult struct {
		index  int
		result CascadeResult
	}

	var tasks []func() CascadeResult
	if item.MediaType == models.MediaTypeMovie && item.TMDBID != "" {
		tasks = append(tasks, func() CascadeResult { return cd.deleteFromRadarr(ctx, item.TMDBID, item.Title) })
	}
	if item.MediaType == models.MediaTypeTV && item.TVDBID != "" {
		tasks = append(tasks, func() CascadeResult { return cd.deleteFromSonarr(ctx, item.TVDBID, item.Title) })
	}
	if item.TMDBID != "" {
		mediaType := "movie"
		if item.MediaType == models.MediaTypeTV {
			mediaType = "tv"
		}
		tmdbID := item.TMDBID
		title := item.Title
		tasks = append(tasks, func() CascadeResult { return cd.deleteFromOverseerr(ctx, tmdbID, mediaType, title) })
	}

	if len(tasks) == 0 {
		return nil
	}

	ch := make(chan indexedResult, len(tasks))
	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, fn func() CascadeResult) {
			defer wg.Done()
			ch <- indexedResult{index: idx, result: fn()}
		}(i, task)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	results := make([]CascadeResult, len(tasks))
	for ir := range ch {
		results[ir.index] = ir.result
	}
	return results
}

// runCascade handles the common boilerplate for cascade operations:
// config check, timeout context, and error capture.
func (cd *CascadeDeleter) runCascade(ctx context.Context, service, title string, cfg store.IntegrationConfig, err error, fn func(ctx context.Context) (bool, string)) CascadeResult {
	result := CascadeResult{Service: service}

	if err != nil || cfg.URL == "" || cfg.APIKey == "" || !cfg.Enabled {
		return result
	}

	opCtx, cancel := context.WithTimeout(ctx, cascadeTimeout)
	defer cancel()

	success, errMsg := fn(opCtx)
	result.Error = errMsg
	result.Success = success
	return result
}

func (cd *CascadeDeleter) deleteFromRadarr(ctx context.Context, tmdbID, title string) CascadeResult {
	cfg, err := cd.store.GetRadarrConfig()
	return cd.runCascade(ctx, "radarr", title, cfg, err, func(opCtx context.Context) (bool, string) {
		client, err := radarr.NewClient(cfg.URL, cfg.APIKey)
		if err != nil {
			return false, fmt.Sprintf("create client: %v", err)
		}

		movieID, err := client.LookupMovieByTMDB(opCtx, tmdbID)
		if err != nil {
			return false, fmt.Sprintf("lookup TMDB %s: %v", tmdbID, err)
		}
		if movieID == 0 {
			log.Printf("cascade radarr %q: not found in Radarr (TMDB %s)", title, tmdbID)
			return false, ""
		}

		if err := client.DeleteMovie(opCtx, movieID, true); err != nil {
			return false, fmt.Sprintf("delete movie %d: %v", movieID, err)
		}

		log.Printf("cascade radarr %q: deleted (TMDB %s, Radarr ID %d)", title, tmdbID, movieID)
		return true, ""
	})
}

func (cd *CascadeDeleter) deleteFromSonarr(ctx context.Context, tvdbID, title string) CascadeResult {
	cfg, err := cd.store.GetSonarrConfig()
	return cd.runCascade(ctx, "sonarr", title, cfg, err, func(opCtx context.Context) (bool, string) {
		client, err := sonarr.NewClient(cfg.URL, cfg.APIKey)
		if err != nil {
			return false, fmt.Sprintf("create client: %v", err)
		}

		seriesID, err := client.LookupSeriesByTVDB(opCtx, tvdbID)
		if err != nil {
			return false, fmt.Sprintf("lookup TVDB %s: %v", tvdbID, err)
		}
		if seriesID == 0 {
			log.Printf("cascade sonarr %q: not found in Sonarr (TVDB %s)", title, tvdbID)
			return false, ""
		}

		if err := client.DeleteSeries(opCtx, seriesID, true); err != nil {
			return false, fmt.Sprintf("delete series %d: %v", seriesID, err)
		}

		log.Printf("cascade sonarr %q: deleted (TVDB %s, Sonarr ID %d)", title, tvdbID, seriesID)
		return true, ""
	})
}

func (cd *CascadeDeleter) deleteFromOverseerr(ctx context.Context, tmdbID, mediaType, title string) CascadeResult {
	cfg, err := cd.store.GetOverseerrConfig()
	return cd.runCascade(ctx, "overseerr", title, cfg, err, func(opCtx context.Context) (bool, string) {
		client, err := overseerr.NewClient(cfg.URL, cfg.APIKey)
		if err != nil {
			return false, fmt.Sprintf("create client: %v", err)
		}

		tmdbInt, err := strconv.Atoi(tmdbID)
		if err != nil {
			return false, fmt.Sprintf("invalid TMDB ID %q: %v", tmdbID, err)
		}

		requestID, err := client.FindRequestByTMDB(opCtx, tmdbInt, mediaType)
		if err != nil {
			return false, fmt.Sprintf("find request TMDB %s: %v", tmdbID, err)
		}
		if requestID == 0 {
			log.Printf("cascade overseerr %q: no request found (TMDB %s)", title, tmdbID)
			return false, ""
		}

		if err := client.DeleteRequest(opCtx, requestID); err != nil {
			return false, fmt.Sprintf("delete request %d: %v", requestID, err)
		}

		log.Printf("cascade overseerr %q: deleted request %d (TMDB %s)", title, requestID, tmdbID)
		return true, ""
	})
}
