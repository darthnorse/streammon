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

// watchAggCTE pre-aggregates ALL servers' watch_history once by (server_id, k),
// where k is each row's owning library-item key (grandparent_item_id for
// episodes, else item_id). It is NOT filtered to one server: callers join on
// `a.server_id = li.server_id AND a.k = li.item_id` to scope the match
// themselves, which lets the same aggregate be reused by a query whose
// library_items rows span multiple servers (see GetStreamMonWatchTimes in
// library_items.go, which batches items across servers in one call).
//
// This replaced an OR-join evaluated per item, which scanned all of a
// server's history for every library item (~13s on a 2.5k-item / 33k-history
// Plex library; ~22s across a full GetStreamMonWatchTimes call measured on a
// prod DB snapshot). It also fixes the old title fallback's over-attribution,
// which double-counted plays across same-titled items.
//
// MATERIALIZED forces SQLite to compute wh_agg once per query instead of
// re-running it as a correlated subquery per outer row. Without the hint,
// point-lookup shapes such as `li.id IN (...)` make SQLite choose a
// per-row CO-ROUTINE plan that re-aggregates the whole of watch_history on
// every outer iteration -- measured slower than the OR-join this replaces.
const watchAggCTE = `
WITH wh_agg AS MATERIALIZED (
	SELECT server_id,
	       COALESCE(NULLIF(grandparent_item_id, ''), item_id) AS k,
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
	WHERE COALESCE(NULLIF(grandparent_item_id, ''), item_id) != ''
	GROUP BY server_id, k
)`

// libraryDetailSelect appends a COUNT(*) OVER() window column so the caller
// gets the total matching-row count from the same query as the page of rows,
// instead of running the whole-server aggregation a second time in a separate
// COUNT query.
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
	       EXISTS(SELECT 1 FROM maintenance_exclusions me WHERE me.library_item_id = li.id) AS protected,
	       COUNT(*) OVER()                 AS total_count
	FROM library_items li
	LEFT JOIN wh_agg a ON a.server_id = li.server_id AND a.k = li.item_id`

func (s *Store) ListLibraryItemDetails(ctx context.Context, q LibraryItemQuery) (*models.PaginatedResult[models.LibraryItemDetail], error) {
	where, args := q.buildWhere()
	filter := q.playFilter()

	// COUNT(*) OVER() is computed before LIMIT/OFFSET truncate the result set,
	// so a single query yields both the page of rows and the total match count
	// -- the wh_agg aggregation (the expensive part) now runs once per page,
	// not twice.
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
	total := 0
	for rows.Next() {
		item, rowTotal, err := scanLibraryItemDetail(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		total = rowTotal
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &models.PaginatedResult[models.LibraryItemDetail]{
		Items: items, Total: total, Page: q.Page, PerPage: q.PerPage,
	}, nil
}

func scanLibraryItemDetail(rows *sql.Rows) (models.LibraryItemDetail, int, error) {
	var it models.LibraryItemDetail
	var watchedMs int64
	var lastPlayed sql.NullString
	var lastViewer sql.NullString
	var total int
	err := rows.Scan(&it.ID, &it.ServerID, &it.ItemID, &it.Title, &it.Year, &it.MediaType, &it.ThumbURL,
		&it.AddedAt, &it.FileSize, &it.VideoResolution, &it.EpisodeCount, &it.TMDBStatus,
		&it.Plays, &lastPlayed, &watchedMs, &it.UniqueViewers, &it.EpisodesWatched,
		&lastViewer, &it.FlaggedForDeletion, &it.Protected, &total)
	if err != nil {
		return it, 0, fmt.Errorf("scan library item detail: %w", err)
	}
	it.LastViewer = lastViewer.String
	it.TotalHours = float64(watchedMs) / 3600000.0
	if lastPlayed.Valid && lastPlayed.String != "" {
		if t, perr := parseSQLiteTime(lastPlayed.String); perr == nil {
			it.LastPlayedAt = &t
		}
	}
	return it, total, nil
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
		LEFT JOIN wh_agg a ON a.server_id = li.server_id AND a.k = li.item_id
		WHERE li.server_id = ? AND li.library_id = ?`
	var sum models.LibrarySummary
	err := s.db.QueryRowContext(ctx, query, serverID, libraryID).Scan(
		&sum.TotalTitles, &sum.TotalSize, &sum.WatchedTitles, &sum.NeverPlayed, &sum.ReclaimableSize)
	if err != nil {
		return nil, fmt.Errorf("get library summary: %w", err)
	}
	return &sum, nil
}
