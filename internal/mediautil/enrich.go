package mediautil

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"streammon/internal/models"
)

// SizeFetcher fetches the total file size for a series by its item ID.
type SizeFetcher func(ctx context.Context, itemID string) (int64, error)

// seriesEnrichConcurrency bounds the parallel per-series size lookups. Plex
// fetches episode sizes one HTTP round-trip per show (allLeaves) and this is the
// dominant cost of a large TV-library sync, so we allow a wider fan-out than the
// other phases.
const seriesEnrichConcurrency = 24

// ParallelEnrich runs `do` concurrently (bounded by limit) over each item for
// which `needs` reports true, handing each goroutine its own *items[i] — distinct
// per goroutine, so writing through the pointer is race-free. It emits one
// progress event per completed item under `phase` and returns how many items it
// processed (so callers can log a count).
func ParallelEnrich[T any](ctx context.Context, items []T, limit int, phase, libraryID string, needs func(*T) bool, do func(context.Context, *T)) int {
	var todo []int
	for i := range items {
		if needs(&items[i]) {
			todo = append(todo, i)
		}
	}
	if len(todo) == 0 {
		return 0
	}

	var g errgroup.Group
	g.SetLimit(limit)
	var done int64
	total := len(todo)

	for _, i := range todo {
		if ctx.Err() != nil {
			break
		}
		g.Go(func() error {
			do(ctx, &items[i])
			SendProgress(ctx, SyncProgress{
				Phase:   phase,
				Current: int(atomic.AddInt64(&done, 1)),
				Total:   total,
				Library: libraryID,
			})
			return nil
		})
	}
	_ = g.Wait()
	return total
}

// EnrichSeriesData fills in missing file sizes for a slice of series items, with
// bounded concurrency. Sends progress throughout; used by Plex and Emby/Jellyfin.
func EnrichSeriesData(ctx context.Context, series []models.LibraryItemCache, libraryID, serverType string, fetchSize SizeFetcher) {
	ParallelEnrich(ctx, series, seriesEnrichConcurrency, PhaseItems, libraryID,
		func(it *models.LibraryItemCache) bool { return it.FileSize == 0 },
		func(ctx context.Context, it *models.LibraryItemCache) {
			size, err := fetchSize(ctx, it.ItemID)
			if err != nil {
				slog.Warn("failed to get episode sizes", "server_type", serverType, "title", it.Title, "error", err)
				return
			}
			it.FileSize = size
		})
}

// EnrichLastWatched updates each item's LastWatchedAt from the history map
// if the history timestamp is newer. It never downgrades a newer existing value.
func EnrichLastWatched(items []models.LibraryItemCache, historyMap map[string]time.Time) {
	for i := range items {
		histTime, ok := historyMap[items[i].ItemID]
		if !ok {
			continue
		}
		if items[i].LastWatchedAt == nil || histTime.After(*items[i].LastWatchedAt) {
			items[i].LastWatchedAt = &histTime
		}
	}
}

// LogSyncSummary logs a structured summary of a completed library sync.
func LogSyncSummary(serverType, libraryID string, movieCount, seriesCount int, items []models.LibraryItemCache) {
	var totalSize int64
	var zeroSize int
	for _, item := range items {
		totalSize += item.FileSize
		if item.FileSize == 0 {
			zeroSize++
		}
	}
	slog.Info("library sync complete",
		"server_type", serverType, "library", libraryID,
		"movies", movieCount, "series", seriesCount,
		"total", len(items), "zero_size", zeroSize, "total_bytes", totalSize)
}
