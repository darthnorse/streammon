package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"streammon/internal/models"
)

const candidateBaseColumns = `
	c.id, c.rule_id, c.library_item_id, c.reason, c.computed_at,
	i.id, i.server_id, i.library_id, i.item_id, i.media_type, i.title, i.year,
	i.added_at, i.last_watched_at, i.video_resolution, i.video_width, i.video_height, i.file_size, i.episode_count, i.thumb_url,
	i.tmdb_id, i.tvdb_id, i.imdb_id, i.tmdb_status, i.synced_at`

// candidateSelectColumns computes play_count with a correlated subquery,
// re-run once per returned row. That's fine for the paginated path
// (ListCandidatesForRule bounds it to PerPage rows) but far too slow applied
// to hundreds/thousands of rows -- see candidateSelectColumnsAgg below,
// used by the unbounded list paths.
var candidateSelectColumns = candidateBaseColumns + `,
	(SELECT COUNT(*) FROM watch_history wh WHERE wh.server_id = i.server_id AND (wh.item_id = i.item_id OR wh.grandparent_item_id = i.item_id) AND ` + minPlayCond("wh") + `) as play_count`

// candidatePlayCountCTE pre-aggregates watch_history into a play count per
// (server_id, k) -- the same key shape watchAggCTE uses (library_detail.go)
// -- where k is grandparent_item_id when set, else item_id, so one pass over
// watch_history covers every candidate row instead of one correlated
// subquery per row.
//
// This is intentionally a separate aggregate from watchAggCTE rather than a
// shared one: watchAggCTE's plays column is an unfiltered raw play count
// (used for library-detail/summary display) and always has been, even before
// it existed as a CTE. This candidate list's play_count has always applied
// minPlayCond to exclude very short/interrupted plays from longer content --
// that's the existing, intentional semantics being preserved here, not
// something to unify away.
//
// MATERIALIZED for the same reason as watchAggCTE: GetMaintenanceCandidates
// below joins via an id IN (...) list, a point-lookup shape that makes
// SQLite choose a per-row CO-ROUTINE plan (re-aggregating all of
// watch_history per outer row) without the hint.
var candidatePlayCountCTE = `
WITH play_counts AS MATERIALIZED (
	SELECT server_id,
	       COALESCE(NULLIF(grandparent_item_id, ''), item_id) AS k,
	       COUNT(*) AS play_count
	FROM watch_history
	WHERE COALESCE(NULLIF(grandparent_item_id, ''), item_id) != ''
	  AND ` + minPlayCond("") + `
	GROUP BY server_id, k
)`

// candidateSelectColumnsAgg is candidateSelectColumns' shape with play_count
// sourced from a LEFT JOIN against candidatePlayCountCTE instead of a
// correlated subquery. Callers must prefix their query with
// candidatePlayCountCTE and join play_counts pc on pc.server_id =
// i.server_id and pc.k = i.item_id.
var candidateSelectColumnsAgg = candidateBaseColumns + `,
	COALESCE(pc.play_count, 0) as play_count`

// SQL mirror of mediautil.HeightFromWidth + resolveLogicalHeight; keep in sync.
const candidateLogicalHeightSQL = `
	CASE
		WHEN i.video_width >= 3840 THEN MAX(2160, i.video_height)
		WHEN i.video_width >= 1920 THEN MAX(1080, i.video_height)
		WHEN i.video_width >= 1280 THEN MAX(720, i.video_height)
		WHEN i.video_width >= 720  THEN MAX(480, i.video_height)
		WHEN i.video_height > 0    THEN i.video_height
		ELSE 0
	END`

func scanCandidate(scanner interface{ Scan(...any) error }) (models.MaintenanceCandidate, error) {
	var c models.MaintenanceCandidate
	var item models.LibraryItemCache
	var lastWatchedAt sql.NullString
	err := scanner.Scan(
		&c.ID, &c.RuleID, &c.LibraryItemID, &c.Reason, &c.ComputedAt,
		&item.ID, &item.ServerID, &item.LibraryID, &item.ItemID, &item.MediaType,
		&item.Title, &item.Year, &item.AddedAt, &lastWatchedAt, &item.VideoResolution, &item.VideoWidth, &item.VideoHeight, &item.FileSize,
		&item.EpisodeCount, &item.ThumbURL,
		&item.TMDBID, &item.TVDBID, &item.IMDBID, &item.TMDBStatus, &item.SyncedAt,
		&c.PlayCount,
	)
	if err != nil {
		return c, err
	}
	if lastWatchedAt.Valid && lastWatchedAt.String != "" {
		t, parseErr := parseSQLiteTime(lastWatchedAt.String)
		if parseErr == nil {
			item.LastWatchedAt = &t
		}
	}
	c.Item = &item
	return c, nil
}

func (s *Store) UpsertMaintenanceCandidate(ctx context.Context, ruleID, libraryItemID int64, reason string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO maintenance_candidates (rule_id, library_item_id, reason, computed_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(rule_id, library_item_id) DO UPDATE SET
			reason = excluded.reason,
			computed_at = excluded.computed_at`,
		ruleID, libraryItemID, reason, now)
	if err != nil {
		return fmt.Errorf("upsert maintenance candidate: %w", err)
	}
	return nil
}

func (s *Store) DeleteCandidatesForRule(ctx context.Context, ruleID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM maintenance_candidates WHERE rule_id = ?`, ruleID)
	if err != nil {
		return fmt.Errorf("delete candidates for rule: %w", err)
	}
	return nil
}

var validCandidateSortColumns = map[string]string{
	"title":      "i.title",
	"year":       "i.year",
	"resolution": "i.video_resolution",
	"size":       "i.file_size",
	"reason":     "c.reason",
	"added_at":   "i.added_at",
	"watches":    "play_count",
	"status":     "i.tmdb_status",
}

// ListCandidatesForRule returns candidates with their library items, excluding excluded items.
func (s *Store) ListCandidatesForRule(ctx context.Context, ruleID int64, opts models.CandidateListOptions) (*models.CandidatesResponse, error) {
	var total int
	var totalSize int64
	var args []any

	baseWhere := `c.rule_id = ? AND e.id IS NULL`
	args = append(args, ruleID)

	if opts.ServerID > 0 && opts.LibraryID != "" {
		baseWhere += ` AND i.server_id = ? AND i.library_id = ?`
		args = append(args, opts.ServerID, opts.LibraryID)
	}

	if opts.Status != "" {
		baseWhere += ` AND i.tmdb_status = ?`
		args = append(args, opts.Status)
	}

	if opts.Search != "" {
		searchPattern := "%" + escapeLikePattern(opts.Search) + "%"
		baseWhere += ` AND (i.title LIKE ? ESCAPE '\' OR CAST(i.year AS TEXT) LIKE ? ESCAPE '\' OR i.video_resolution LIKE ? ESCAPE '\' OR i.tmdb_status LIKE ? ESCAPE '\' OR c.reason LIKE ? ESCAPE '\')`
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern)
	}

	statsQuery := `
		SELECT COUNT(*), COALESCE(SUM(i.file_size), 0) FROM maintenance_candidates c
		JOIN library_items i ON c.library_item_id = i.id
		LEFT JOIN maintenance_exclusions e ON c.library_item_id = e.library_item_id
		WHERE ` + baseWhere
	err := s.db.QueryRowContext(ctx, statsQuery, args...).Scan(&total, &totalSize)
	if err != nil {
		return nil, fmt.Errorf("count candidates: %w", err)
	}

	var exclusionCount int
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maintenance_candidates c
		JOIN maintenance_exclusions e ON c.library_item_id = e.library_item_id
		WHERE c.rule_id = ?`, ruleID).Scan(&exclusionCount)
	if err != nil {
		return nil, fmt.Errorf("count exclusions: %w", err)
	}

	orderBy := "i.added_at DESC"
	if col, ok := validCandidateSortColumns[opts.SortBy]; ok {
		if opts.SortBy == "resolution" {
			widthAware, err := s.GetMaintenanceResolutionWidthAware()
			if err != nil {
				return nil, fmt.Errorf("get resolution mode: %w", err)
			}
			if widthAware {
				col = candidateLogicalHeightSQL
			}
		}
		dir := "DESC"
		if opts.SortOrder == "asc" {
			dir = "ASC"
		}
		orderBy = col + " " + dir
	}

	page := opts.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * opts.PerPage
	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, opts.PerPage, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+candidateSelectColumns+`
		FROM maintenance_candidates c
		JOIN library_items i ON c.library_item_id = i.id
		LEFT JOIN maintenance_exclusions e ON c.library_item_id = e.library_item_id
		WHERE `+baseWhere+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, fmt.Errorf("list candidates: %w", err)
	}
	defer rows.Close()

	candidates := []models.MaintenanceCandidate{}
	for rows.Next() {
		c, err := scanCandidate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := s.populateOtherCopies(ctx, candidates); err != nil {
		return nil, fmt.Errorf("populate other copies: %w", err)
	}

	var statuses []string
	statusWhere := `c.rule_id = ? AND e.id IS NULL AND i.tmdb_status != ''`
	statusArgs := []any{ruleID}
	if opts.ServerID > 0 && opts.LibraryID != "" {
		statusWhere += ` AND i.server_id = ? AND i.library_id = ?`
		statusArgs = append(statusArgs, opts.ServerID, opts.LibraryID)
	}
	statusRows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT i.tmdb_status FROM maintenance_candidates c
		JOIN library_items i ON c.library_item_id = i.id
		LEFT JOIN maintenance_exclusions e ON c.library_item_id = e.library_item_id
		WHERE `+statusWhere+`
		ORDER BY i.tmdb_status`, statusArgs...)
	if err != nil {
		return nil, fmt.Errorf("distinct statuses: %w", err)
	}
	defer statusRows.Close()
	for statusRows.Next() {
		var s string
		if err := statusRows.Scan(&s); err != nil {
			return nil, fmt.Errorf("scan status: %w", err)
		}
		statuses = append(statuses, s)
	}
	if err := statusRows.Err(); err != nil {
		return nil, fmt.Errorf("status rows: %w", err)
	}

	return &models.CandidatesResponse{
		Items:          candidates,
		Total:          total,
		TotalSize:      totalSize,
		ExclusionCount: exclusionCount,
		Page:           page,
		PerPage:        opts.PerPage,
		Statuses:       statuses,
	}, nil
}

// populateOtherCopies finds library_items sharing external IDs with the given candidates
// and attaches them as OtherCopies. Only called for paginated results (max 25 items).
func (s *Store) populateOtherCopies(ctx context.Context, candidates []models.MaintenanceCandidate) error {
	if len(candidates) == 0 {
		return nil
	}

	var itemIDs []int64
	tmdbIDs := make(map[string]struct{})
	tvdbIDs := make(map[string]struct{})
	imdbIDs := make(map[string]struct{})

	for _, c := range candidates {
		if c.Item == nil {
			continue
		}
		itemIDs = append(itemIDs, c.Item.ID)
		if c.Item.TMDBID != "" {
			tmdbIDs[c.Item.TMDBID] = struct{}{}
		}
		if c.Item.TVDBID != "" {
			tvdbIDs[c.Item.TVDBID] = struct{}{}
		}
		if c.Item.IMDBID != "" {
			imdbIDs[c.Item.IMDBID] = struct{}{}
		}
	}

	if len(tmdbIDs) == 0 && len(tvdbIDs) == 0 && len(imdbIDs) == 0 {
		return nil
	}

	var conditions []string
	var args []any

	addInClause := func(column string, ids map[string]struct{}) {
		if len(ids) == 0 {
			return
		}
		phs := make([]string, 0, len(ids))
		for id := range ids {
			phs = append(phs, "?")
			args = append(args, id)
		}
		conditions = append(conditions, column+" IN ("+strings.Join(phs, ",")+")")
	}
	addInClause("tmdb_id", tmdbIDs)
	addInClause("tvdb_id", tvdbIDs)
	addInClause("imdb_id", imdbIDs)

	itemPhs := make([]string, len(itemIDs))
	for i, id := range itemIDs {
		itemPhs[i] = "?"
		args = append(args, id)
	}

	query := `SELECT id, server_id, library_id, tmdb_id, tvdb_id, imdb_id
		FROM library_items
		WHERE (` + strings.Join(conditions, " OR ") + `)
		AND id NOT IN (` + strings.Join(itemPhs, ",") + `)`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query other copies: %w", err)
	}
	defer rows.Close()

	type copyRow struct {
		id        int64
		serverID  int64
		libraryID string
		tmdbID    string
		tvdbID    string
		imdbID    string
	}
	var copies []copyRow
	for rows.Next() {
		var r copyRow
		if err := rows.Scan(&r.id, &r.serverID, &r.libraryID, &r.tmdbID, &r.tvdbID, &r.imdbID); err != nil {
			return fmt.Errorf("scan other copy: %w", err)
		}
		copies = append(copies, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Match copies to candidates by shared external IDs, deduplicated by (server_id, library_id)
	for i := range candidates {
		item := candidates[i].Item
		if item == nil {
			continue
		}
		seen := make(map[string]bool)
		for _, cp := range copies {
			if cp.serverID == item.ServerID && cp.libraryID == item.LibraryID {
				continue
			}
			if (item.TMDBID != "" && item.TMDBID == cp.tmdbID) ||
				(item.TVDBID != "" && item.TVDBID == cp.tvdbID) ||
				(item.IMDBID != "" && item.IMDBID == cp.imdbID) {
				key := fmt.Sprintf("%d:%s", cp.serverID, cp.libraryID)
				if seen[key] {
					continue
				}
				seen[key] = true
				candidates[i].OtherCopies = append(candidates[i].OtherCopies, models.RuleLibrary{
					ServerID:  cp.serverID,
					LibraryID: cp.libraryID,
				})
			}
		}
	}

	return nil
}

// CountCandidatesForRule returns the count of candidates for a rule, excluding excluded items
func (s *Store) CountCandidatesForRule(ctx context.Context, ruleID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maintenance_candidates c
		LEFT JOIN maintenance_exclusions e ON c.library_item_id = e.library_item_id
		WHERE c.rule_id = ? AND e.id IS NULL`, ruleID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count candidates for rule: %w", err)
	}
	return count, nil
}

// HasKeepLatestSeasonsCandidate returns true if the given library item is a
// candidate for any maintenance rule with criterion_type = 'keep_latest_seasons'.
func (s *Store) HasKeepLatestSeasonsCandidate(ctx context.Context, libraryItemID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM maintenance_candidates c
			JOIN maintenance_rules r ON r.id = c.rule_id
			WHERE c.library_item_id = ? AND r.criterion_type = 'keep_latest_seasons'
		)`, libraryItemID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check keep_latest_seasons candidate: %w", err)
	}
	return exists, nil
}

// BatchUpsertCandidates replaces all candidates for a rule: the existing set
// is deleted, then the new set is inserted in chunks of writeChunkSize, each
// its own transaction, so the write lock is released periodically instead of
// held for a whole rule's (potentially large) candidate set -- see
// writeChunkSize.
//
// This trades the old single-transaction all-or-nothing replace for a
// narrower guarantee: each chunk is atomic, but the full replace no longer
// is. A reader (e.g. the maintenance dashboard) can observe a transient
// partial state mid-call, and a failure partway through leaves the delete
// applied plus whatever chunks committed before the error -- not the old
// candidate set, and not the full new one either. That's acceptable here:
// candidates are informational/advisory, never auto-deleted (deleting a
// library item always requires an explicit authenticated admin action, see
// the deletion-safety note on GetStreamMonWatchTimes in library_items.go),
// and the next scheduled or manual re-evaluation recomputes the set from
// scratch regardless.
func (s *Store) BatchUpsertCandidates(ctx context.Context, ruleID int64, candidates []models.BatchCandidate) error {
	if err := s.DeleteCandidatesForRule(ctx, ruleID); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, chunk := range chunkSlice(candidates, writeChunkSize) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.insertCandidateChunk(ctx, ruleID, chunk, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) insertCandidateChunk(ctx context.Context, ruleID int64, chunk []models.BatchCandidate, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO maintenance_candidates (rule_id, library_item_id, reason, computed_at)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, c := range chunk {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := stmt.ExecContext(ctx, ruleID, c.LibraryItemID, c.Reason, now); err != nil {
			return fmt.Errorf("insert candidate: %w", err)
		}
	}

	return tx.Commit()
}

func (s *Store) GetMaintenanceCandidate(ctx context.Context, id int64) (*models.MaintenanceCandidate, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+candidateSelectColumns+`
		FROM maintenance_candidates c
		JOIN library_items i ON c.library_item_id = i.id
		WHERE c.id = ?`, id)

	c, err := scanCandidate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get maintenance candidate: %w", err)
	}
	return &c, nil
}

func (s *Store) DeleteMaintenanceCandidate(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM maintenance_candidates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete maintenance candidate: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteLibraryItem(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM library_items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete library item: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return models.ErrNotFound
	}
	return nil
}

func (s *Store) GetMaintenanceCandidates(ctx context.Context, ids []int64) ([]models.MaintenanceCandidate, error) {
	if len(ids) == 0 {
		return []models.MaintenanceCandidate{}, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := candidatePlayCountCTE + `
		SELECT ` + candidateSelectColumnsAgg + `
		FROM maintenance_candidates c
		JOIN library_items i ON c.library_item_id = i.id
		LEFT JOIN play_counts pc ON pc.server_id = i.server_id AND pc.k = i.item_id
		WHERE c.id IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get candidates: %w", err)
	}
	defer rows.Close()

	candidates := []models.MaintenanceCandidate{}
	for rows.Next() {
		c, err := scanCandidate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

func (s *Store) RecordDeleteAction(ctx context.Context, serverID int64, itemID, title, mediaType string, fileSize int64, deletedBy string, serverDeleted bool, errMsg string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO maintenance_delete_log (server_id, item_id, title, media_type, file_size, deleted_by, deleted_at, server_deleted, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		serverID, itemID, title, mediaType, fileSize, deletedBy, time.Now().UTC(), serverDeleted, errMsg)
	if err != nil {
		return fmt.Errorf("record delete action: %w", err)
	}
	return nil
}

// ListAllCandidatesForRule returns all candidates without pagination, excluding excluded items
func (s *Store) ListAllCandidatesForRule(ctx context.Context, ruleID int64) ([]models.MaintenanceCandidate, error) {
	rows, err := s.db.QueryContext(ctx, candidatePlayCountCTE+`
		SELECT `+candidateSelectColumnsAgg+`
		FROM maintenance_candidates c
		JOIN library_items i ON c.library_item_id = i.id
		LEFT JOIN maintenance_exclusions e ON c.library_item_id = e.library_item_id
		LEFT JOIN play_counts pc ON pc.server_id = i.server_id AND pc.k = i.item_id
		WHERE c.rule_id = ? AND e.id IS NULL
		ORDER BY i.added_at DESC`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("list all candidates: %w", err)
	}
	defer rows.Close()

	candidates := []models.MaintenanceCandidate{}
	for rows.Next() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		c, err := scanCandidate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}
