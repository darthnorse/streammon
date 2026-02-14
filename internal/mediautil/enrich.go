package mediautil

import (
	"time"

	"streammon/internal/models"
)

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
