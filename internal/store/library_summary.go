package store

import (
	"context"
	"fmt"
)

// LibraryCounts is the bare count payload shared by per-server entries and
// the aggregated top-level total. New media buckets only need to be added
// here once.
//
// Note on the data model: rows with media_type='episode' represent TV SERIES,
// one row per series; the episode_count column on each row carries the actual
// episode count. So Shows = number of series; Episodes = sum of episodes
// across all series; Movies = count of media_type='movie' rows; Other =
// count of any other media_type (track/audiobook/book/livetv/etc).
type LibraryCounts struct {
	TotalItems int `json:"total_items"`
	Movies     int `json:"movies"`
	Shows      int `json:"shows"`
	Episodes   int `json:"episodes"`
	Other      int `json:"other"`
	Libraries  int `json:"libraries"`
}

// LibraryServerSummary is the per-server breakdown of library item counts.
type LibraryServerSummary struct {
	ServerID   int64  `json:"server_id"`
	ServerName string `json:"server_name"`
	LibraryCounts
}

// LibrarySummary returns one entry per active (non-soft-deleted) server with
// library items, with counts grouped by media type plus a distinct-library
// count. Soft-deleted servers are excluded so widget consumers don't see
// stale data with empty server names.
func (s *Store) LibrarySummary(ctx context.Context) ([]LibraryServerSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			li.server_id,
			s.name                                                                    AS server_name,
			COUNT(*)                                                                  AS total,
			SUM(CASE WHEN li.media_type = 'movie'   THEN 1                ELSE 0 END) AS movies,
			SUM(CASE WHEN li.media_type = 'episode' THEN 1                ELSE 0 END) AS shows,
			SUM(CASE WHEN li.media_type = 'episode' THEN li.episode_count ELSE 0 END) AS episodes,
			SUM(CASE WHEN li.media_type NOT IN ('movie','episode') THEN 1 ELSE 0 END) AS other,
			COUNT(DISTINCT li.library_id)                                             AS libraries
		FROM library_items li
		JOIN servers s ON s.id = li.server_id AND s.deleted_at IS NULL
		GROUP BY li.server_id, s.name
		ORDER BY li.server_id`)
	if err != nil {
		return nil, fmt.Errorf("querying library summary: %w", err)
	}
	defer rows.Close()

	out := []LibraryServerSummary{}
	for rows.Next() {
		var e LibraryServerSummary
		if err := rows.Scan(&e.ServerID, &e.ServerName, &e.TotalItems, &e.Movies, &e.Shows, &e.Episodes, &e.Other, &e.Libraries); err != nil {
			return nil, fmt.Errorf("scanning library summary row: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
