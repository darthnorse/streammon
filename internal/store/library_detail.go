package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"streammon/internal/models"
)

type LibraryItemQuery struct {
	ServerID   int64
	LibraryID  string
	Page       int
	PerPage    int
	Search     string
	Filter     string
	SortColumn string
	SortOrder  string
}

// watchAggCTE pre-aggregates a server's watch_history once by each row's owning
// library-item key (grandparent_item_id for episodes, else item_id), then the
// queries below join to library_items by that indexed key. This replaced an
// OR-join evaluated per item, which scanned all of a server's history for every
// library item (~13s on a 2.5k-item / 33k-history Plex library). It also fixes
// the old title fallback's over-attribution, which double-counted plays across
// same-titled items.
const watchAggCTE = `
WITH wh_agg AS (
	SELECT COALESCE(NULLIF(grandparent_item_id, ''), item_id) AS k,
	       COUNT(*)                     AS plays,
	       MAX(stopped_at)              AS last_played_at,
	       COALESCE(SUM(watched_ms), 0) AS watched_ms,
	       COUNT(DISTINCT user_name)    AS unique_viewers,
	       COUNT(DISTINCT item_id)      AS episodes_watched,
	       -- last_viewer is a BARE column: with exactly one MAX() in this aggregate,
	       -- SQLite resolves it from the most-recent-play row. Keep it the only
	       -- min/max here or last_viewer becomes arbitrary.
	       user_name                    AS last_viewer
	FROM watch_history
	WHERE server_id = ? AND COALESCE(NULLIF(grandparent_item_id, ''), item_id) != ''
	GROUP BY k
)`

const libraryDetailSelect = watchAggCTE + `
	SELECT li.id, li.server_id, li.item_id, li.title, li.year, li.media_type, li.thumb_url,
	       li.added_at, li.file_size, li.video_resolution, li.episode_count, li.tmdb_status,
	       COALESCE(a.plays, 0)            AS plays,
	       a.last_played_at                AS last_played_at,
	       COALESCE(a.watched_ms, 0)       AS watched_ms,
	       COALESCE(a.unique_viewers, 0)   AS unique_viewers,
	       COALESCE(a.episodes_watched, 0) AS episodes_watched,
	       a.last_viewer                   AS last_viewer,
	       EXISTS(SELECT 1 FROM maintenance_candidates mc WHERE mc.library_item_id = li.id) AS flagged,
	       EXISTS(SELECT 1 FROM maintenance_exclusions me WHERE me.library_item_id = li.id) AS protected
	FROM library_items li
	LEFT JOIN wh_agg a ON a.k = li.item_id`

func (s *Store) ListLibraryItemDetails(ctx context.Context, q LibraryItemQuery) (*models.PaginatedResult[models.LibraryItemDetail], error) {
	where, args := q.buildWhere()
	filter := q.playFilter()
	// The wh_agg CTE consumes server_id first; buildWhere's args follow. The
	// wh_agg <-> library_items join is 1:1 (both keys unique), so no GROUP BY.
	args = append([]any{q.ServerID}, args...)

	countQuery := watchAggCTE + `
		SELECT COUNT(*) FROM library_items li
		LEFT JOIN wh_agg a ON a.k = li.item_id` + where + filter
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count library item details: %w", err)
	}

	listQuery := libraryDetailSelect + where + filter + q.orderClause()
	listArgs := args
	if q.PerPage > 0 {
		listQuery += ` LIMIT ? OFFSET ?`
		offset := (q.Page - 1) * q.PerPage
		if offset < 0 {
			offset = 0
		}
		listArgs = append(append([]any{}, args...), q.PerPage, offset)
	}

	rows, err := s.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, fmt.Errorf("list library item details: %w", err)
	}
	defer rows.Close()

	items := make([]models.LibraryItemDetail, 0)
	for rows.Next() {
		item, err := scanLibraryItemDetail(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &models.PaginatedResult[models.LibraryItemDetail]{
		Items: items, Total: total, Page: q.Page, PerPage: q.PerPage,
	}, nil
}

func scanLibraryItemDetail(rows *sql.Rows) (models.LibraryItemDetail, error) {
	var it models.LibraryItemDetail
	var watchedMs int64
	var lastPlayed sql.NullString
	var lastViewer sql.NullString
	err := rows.Scan(&it.ID, &it.ServerID, &it.ItemID, &it.Title, &it.Year, &it.MediaType, &it.ThumbURL,
		&it.AddedAt, &it.FileSize, &it.VideoResolution, &it.EpisodeCount, &it.TMDBStatus,
		&it.Plays, &lastPlayed, &watchedMs, &it.UniqueViewers, &it.EpisodesWatched,
		&lastViewer, &it.FlaggedForDeletion, &it.Protected)
	if err != nil {
		return it, fmt.Errorf("scan library item detail: %w", err)
	}
	it.LastViewer = lastViewer.String
	it.TotalHours = float64(watchedMs) / 3600000.0
	if lastPlayed.Valid && lastPlayed.String != "" {
		if t, perr := parseSQLiteTime(lastPlayed.String); perr == nil {
			it.LastPlayedAt = &t
		}
	}
	return it, nil
}

func (q LibraryItemQuery) buildWhere() (string, []any) {
	clauses := []string{"li.server_id = ?", "li.library_id = ?"}
	args := []any{q.ServerID, q.LibraryID}
	if q.Search != "" {
		clauses = append(clauses, `li.title LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLikePattern(q.Search)+"%")
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (q LibraryItemQuery) playFilter() string {
	switch q.Filter {
	case "played":
		return " AND COALESCE(a.plays, 0) > 0"
	case "unplayed":
		return " AND COALESCE(a.plays, 0) = 0"
	default:
		return ""
	}
}

func (q LibraryItemQuery) orderClause() string {
	if q.SortColumn == "" {
		return " ORDER BY li.added_at DESC"
	}
	order := "ASC"
	if strings.ToLower(q.SortOrder) == "desc" {
		order = "DESC"
	}
	// SortColumn is a pre-validated safe expression from the handler allow-list.
	return fmt.Sprintf(" ORDER BY %s %s, li.added_at DESC", q.SortColumn, order)
}

// libraryWatchMatch is the ON predicate matching watch_history rows to a library
// item — by item_id, by series grandparent_item_id, or by a same-server/same-type
// title fallback for legacy rows. Used by GetStreamMonWatchTimes (batched by item
// id, where the OR is cheap); the library detail/summary queries use watchAggCTE
// instead, which is far faster over a whole library.
const libraryWatchMatch = `wh.server_id = li.server_id
		AND (
			wh.item_id = li.item_id
			OR wh.grandparent_item_id = li.item_id
			OR (li.title != '' AND wh.media_type = li.media_type
				AND (wh.title = li.title OR wh.grandparent_title = li.title))
		)`

func (s *Store) GetLibrarySummary(ctx context.Context, serverID int64, libraryID string) (*models.LibrarySummary, error) {
	query := watchAggCTE + `
		SELECT COUNT(*)                                                        AS total_titles,
		       COALESCE(SUM(li.file_size), 0)                                  AS total_size,
		       COALESCE(SUM(CASE WHEN COALESCE(a.plays, 0) > 0 THEN 1 ELSE 0 END), 0) AS watched_titles,
		       COALESCE(SUM(CASE WHEN COALESCE(a.plays, 0) = 0 THEN 1 ELSE 0 END), 0) AS never_played,
		       -- reclaimable = space held by never-played titles that aren't protected
		       -- (protected items live in maintenance_exclusions and won't be deleted).
		       COALESCE(SUM(CASE WHEN COALESCE(a.plays, 0) = 0
		            AND NOT EXISTS(SELECT 1 FROM maintenance_exclusions me WHERE me.library_item_id = li.id)
		            THEN li.file_size ELSE 0 END), 0)                          AS reclaimable_size
		FROM library_items li
		LEFT JOIN wh_agg a ON a.k = li.item_id
		WHERE li.server_id = ? AND li.library_id = ?`
	var sum models.LibrarySummary
	err := s.db.QueryRowContext(ctx, query, serverID, serverID, libraryID).Scan(
		&sum.TotalTitles, &sum.TotalSize, &sum.WatchedTitles, &sum.NeverPlayed, &sum.ReclaimableSize)
	if err != nil {
		return nil, fmt.Errorf("get library summary: %w", err)
	}
	return &sum, nil
}
