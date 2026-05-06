package store

import (
	"context"
	"fmt"
)

// LibraryServerSummary is the per-server breakdown of library item counts.
//
// Note on the data model: rows with media_type='episode' represent TV SERIES,
// one row per series; the episode_count column on each row carries the actual
// episode count. So Shows = number of series; Episodes = sum of episodes
// across all series; Movies = count of media_type='movie' rows.
type LibraryServerSummary struct {
	ServerID   int64 `json:"server_id"`
	TotalItems int   `json:"total_items"`
	Movies     int   `json:"movies"`
	Shows      int   `json:"shows"`
	Episodes   int   `json:"episodes"`
	Libraries  int   `json:"libraries"`
}

// LibrarySummary returns one entry per server that has any library items,
// with counts grouped by media type plus a distinct-library count.
func (s *Store) LibrarySummary(ctx context.Context) ([]LibraryServerSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			server_id,
			COUNT(*)                                                                AS total,
			SUM(CASE WHEN media_type = 'movie'   THEN 1             ELSE 0 END)     AS movies,
			SUM(CASE WHEN media_type = 'episode' THEN 1             ELSE 0 END)     AS shows,
			SUM(CASE WHEN media_type = 'episode' THEN episode_count ELSE 0 END)     AS episodes,
			COUNT(DISTINCT library_id)                                              AS libraries
		FROM library_items
		GROUP BY server_id
		ORDER BY server_id`)
	if err != nil {
		return nil, fmt.Errorf("querying library summary: %w", err)
	}
	defer rows.Close()

	out := []LibraryServerSummary{}
	for rows.Next() {
		var e LibraryServerSummary
		if err := rows.Scan(&e.ServerID, &e.TotalItems, &e.Movies, &e.Shows, &e.Episodes, &e.Libraries); err != nil {
			return nil, fmt.Errorf("scanning library summary row: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
