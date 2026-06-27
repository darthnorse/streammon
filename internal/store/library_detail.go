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

const libraryDetailSelect = `
	SELECT li.id, li.server_id, li.item_id, li.title, li.year, li.media_type, li.thumb_url,
	       li.added_at, li.file_size, li.video_resolution, li.episode_count, li.tmdb_status,
	       COUNT(wh.id)                    AS plays,
	       MAX(wh.stopped_at)              AS last_played_at,
	       COALESCE(SUM(wh.watched_ms), 0) AS watched_ms,
	       COUNT(DISTINCT wh.user_name)    AS unique_viewers,
	       COUNT(DISTINCT wh.item_id)      AS episodes_watched,
	       -- last_viewer is a bare column: with exactly one max() aggregate in the
	       -- query, SQLite resolves bare columns from the MAX(wh.stopped_at) row —
	       -- i.e. the most recent play's user. NULL for never-played (LEFT JOIN) rows.
	       wh.user_name                    AS last_viewer,
	       EXISTS(SELECT 1 FROM maintenance_candidates mc WHERE mc.library_item_id = li.id) AS flagged,
	       EXISTS(SELECT 1 FROM maintenance_exclusions me WHERE me.library_item_id = li.id) AS protected
	FROM library_items li` + libraryWatchJoin

func (s *Store) ListLibraryItemDetails(ctx context.Context, q LibraryItemQuery) (*models.PaginatedResult[models.LibraryItemDetail], error) {
	where, args := q.buildWhere()
	having := q.buildHaving()

	// total = number of grouped rows after WHERE/HAVING
	countQuery := `SELECT COUNT(*) FROM (SELECT li.id FROM library_items li` +
		libraryWatchJoin + where + ` GROUP BY li.id` + having + `)`
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count library item details: %w", err)
	}

	order := q.orderClause()
	listQuery := libraryDetailSelect + where + ` GROUP BY li.id` + having + order
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

func (q LibraryItemQuery) buildHaving() string {
	switch q.Filter {
	case "played":
		return " HAVING COUNT(wh.id) > 0"
	case "unplayed":
		return " HAVING COUNT(wh.id) = 0"
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
// title fallback for legacy rows. Single-sourced here and reused by libraryWatchJoin
// and GetStreamMonWatchTimes so the match logic can't silently diverge.
const libraryWatchMatch = `wh.server_id = li.server_id
		AND (
			wh.item_id = li.item_id
			OR wh.grandparent_item_id = li.item_id
			OR (li.title != '' AND wh.media_type = li.media_type
				AND (wh.title = li.title OR wh.grandparent_title = li.title))
		)`

// libraryWatchJoin LEFT JOINs watch_history to a library item, reused by the
// summary and detail-list aggregates.
const libraryWatchJoin = `
	LEFT JOIN watch_history wh ON ` + libraryWatchMatch

func (s *Store) GetLibrarySummary(ctx context.Context, serverID int64, libraryID string) (*models.LibrarySummary, error) {
	query := `
		SELECT COUNT(*)                                                       AS total_titles,
		       COALESCE(SUM(file_size), 0)                                    AS total_size,
		       COALESCE(SUM(CASE WHEN plays > 0 THEN 1 ELSE 0 END), 0)        AS watched_titles,
		       COALESCE(SUM(CASE WHEN plays = 0 THEN 1 ELSE 0 END), 0)        AS never_played,
		       -- reclaimable = space held by never-played titles that aren't protected
		       -- (protected items live in maintenance_exclusions and won't be deleted).
		       COALESCE(SUM(CASE WHEN plays = 0 AND protected = 0 THEN file_size ELSE 0 END), 0) AS reclaimable_size
		FROM (
			SELECT li.id, li.file_size, COUNT(wh.id) AS plays,
			       EXISTS(SELECT 1 FROM maintenance_exclusions me WHERE me.library_item_id = li.id) AS protected
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
