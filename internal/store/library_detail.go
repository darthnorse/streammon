package store

import (
	"context"
	"fmt"

	"streammon/internal/models"
)

// libraryWatchJoin is the LEFT JOIN match between watch_history and a library item,
// reused by both the summary and the detail list. Mirrors GetStreamMonWatchTimes.
const libraryWatchJoin = `
	LEFT JOIN watch_history wh ON wh.server_id = li.server_id
		AND (
			wh.item_id = li.item_id
			OR wh.grandparent_item_id = li.item_id
			OR (li.title != '' AND wh.media_type = li.media_type
				AND (wh.title = li.title OR wh.grandparent_title = li.title))
		)`

func (s *Store) GetLibrarySummary(ctx context.Context, serverID int64, libraryID string) (*models.LibrarySummary, error) {
	query := `
		SELECT COUNT(*)                                                       AS total_titles,
		       COALESCE(SUM(file_size), 0)                                    AS total_size,
		       COALESCE(SUM(CASE WHEN plays > 0 THEN 1 ELSE 0 END), 0)        AS watched_titles,
		       COALESCE(SUM(CASE WHEN plays = 0 THEN 1 ELSE 0 END), 0)        AS never_played,
		       COALESCE(SUM(CASE WHEN plays = 0 THEN file_size ELSE 0 END), 0) AS reclaimable_size
		FROM (
			SELECT li.id, li.file_size, COUNT(wh.id) AS plays
			FROM library_items li` + libraryWatchJoin + `
			WHERE li.server_id = ? AND li.library_id = ?
			GROUP BY li.id
		)`
	var sum models.LibrarySummary
	err := s.db.QueryRowContext(ctx, query, serverID, libraryID).Scan(
		&sum.TotalTitles, &sum.TotalSize, &sum.WatchedTitles, &sum.NeverPlayed, &sum.ReclaimableSize)
	if err != nil {
		return nil, fmt.Errorf("get library summary: %w", err)
	}
	return &sum, nil
}
