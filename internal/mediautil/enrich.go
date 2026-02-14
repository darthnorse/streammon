package mediautil

import (
	"context"
	"log/slog"
	"time"

	"streammon/internal/models"
)

// SizeFetcher fetches the total file size for a series by its item ID.
type SizeFetcher func(ctx context.Context, itemID string) (int64, error)

// HistoryFetcher fetches a map of series ID → most recent watch time.
type HistoryFetcher func(ctx context.Context, libraryID string) (map[string]time.Time, error)

// EnrichSeriesData fills in missing file sizes and watch history for a slice of series items.
// It sends progress events throughout and is used by both Plex and Emby/Jellyfin adapters.
func EnrichSeriesData(ctx context.Context, series []models.LibraryItemCache, libraryID, serverType string, fetchSize SizeFetcher, fetchHistory HistoryFetcher) {
	for i := range series {
		if series[i].FileSize == 0 {
			size, err := fetchSize(ctx, series[i].ItemID)
			if err != nil {
				slog.Warn("failed to get episode sizes", "server_type", serverType, "title", series[i].Title, "error", err)
			} else {
				series[i].FileSize = size
			}
		}
		SendProgress(ctx, SyncProgress{
			Phase:   PhaseItems,
			Current: i + 1,
			Total:   len(series),
			Library: libraryID,
		})
	}
	if len(series) > 0 {
		SendProgress(ctx, SyncProgress{
			Phase:   PhaseHistory,
			Library: libraryID,
		})
		historyMap, err := fetchHistory(ctx, libraryID)
		if err != nil {
			slog.Warn("failed to fetch series watch history, using series-level data",
				"server_type", serverType, "error", err)
		} else {
			EnrichLastWatched(series, historyMap)
		}
	}
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
