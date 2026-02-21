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

const candidateSelectColumns = `
	c.id, c.rule_id, c.library_item_id, c.reason, c.computed_at,
	i.id, i.server_id, i.library_id, i.item_id, i.media_type, i.title, i.year,
	i.added_at, i.last_watched_at, i.video_resolution, i.file_size, i.episode_count, i.thumb_url,
	i.tmdb_id, i.tvdb_id, i.imdb_id, i.synced_at,
	(SELECT COUNT(*) FROM watch_history wh WHERE wh.server_id = i.server_id AND (wh.item_id = i.item_id OR wh.grandparent_item_id = i.item_id)) as play_count`

func scanCandidate(scanner interface{ Scan(...any) error }) (models.MaintenanceCandidate, error) {
	var c models.MaintenanceCandidate
	var item models.LibraryItemCache
	var lastWatchedAt sql.NullString
	err := scanner.Scan(
		&c.ID, &c.RuleID, &c.LibraryItemID, &c.Reason, &c.ComputedAt,
		&item.ID, &item.ServerID, &item.LibraryID, &item.ItemID, &item.MediaType,
		&item.Title, &item.Year, &item.AddedAt, &lastWatchedAt, &item.VideoResolution, &item.FileSize,
		&item.EpisodeCount, &item.ThumbURL,
		&item.TMDBID, &item.TVDBID, &item.IMDBID, &item.SyncedAt,
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
}

// ListCandidatesForRule returns candidates with their library items, excluding excluded items.
// Optional serverID/libraryID filters scope results to a single library.
func (s *Store) ListCandidatesForRule(ctx context.Context, ruleID int64, page, perPage int, search, sortBy, sortOrder string, serverID int64, libraryID string) (*models.CandidatesResponse, error) {
	var total int
	var totalSize int64
	var args []any

	baseWhere := `c.rule_id = ? AND e.id IS NULL`
	args = append(args, ruleID)

	if serverID > 0 && libraryID != "" {
		baseWhere += ` AND i.server_id = ? AND i.library_id = ?`
		args = append(args, serverID, libraryID)
	}

	if search != "" {
		searchPattern := "%" + escapeLikePattern(search) + "%"
		baseWhere += ` AND (i.title LIKE ? ESCAPE '\' OR CAST(i.year AS TEXT) LIKE ? ESCAPE '\' OR i.video_resolution LIKE ? ESCAPE '\' OR c.reason LIKE ? ESCAPE '\')`
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern)
	}

	statsQuery := `
		SELECT COUNT(*), COALESCE(SUM(i.file_size), 0) FROM maintenance_candidates c
		JOIN library_items i ON c.library_item_id = i.id
		LEFT JOIN maintenance_exclusions e ON c.rule_id = e.rule_id AND c.library_item_id = e.library_item_id
		WHERE ` + baseWhere
	err := s.db.QueryRowContext(ctx, statsQuery, args...).Scan(&total, &totalSize)
	if err != nil {
		return nil, fmt.Errorf("count candidates: %w", err)
	}

	var exclusionCount int
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM maintenance_exclusions WHERE rule_id = ?`, ruleID).Scan(&exclusionCount)
	if err != nil {
		return nil, fmt.Errorf("count exclusions: %w", err)
	}

	orderBy := "i.added_at DESC"
	if col, ok := validCandidateSortColumns[sortBy]; ok {
		dir := "DESC"
		if sortOrder == "asc" {
			dir = "ASC"
		}
		orderBy = col + " " + dir
	}

	offset := (page - 1) * perPage
	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, perPage, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+candidateSelectColumns+`
		FROM maintenance_candidates c
		JOIN library_items i ON c.library_item_id = i.id
		LEFT JOIN maintenance_exclusions e ON c.rule_id = e.rule_id AND c.library_item_id = e.library_item_id
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

	return &models.CandidatesResponse{
		Items:          candidates,
		Total:          total,
		TotalSize:      totalSize,
		ExclusionCount: exclusionCount,
		Page:           page,
		PerPage:        perPage,
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
		LEFT JOIN maintenance_exclusions e ON c.rule_id = e.rule_id AND c.library_item_id = e.library_item_id
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

// BatchUpsertCandidates replaces all candidates for a rule in a transaction
func (s *Store) BatchUpsertCandidates(ctx context.Context, ruleID int64, candidates []models.BatchCandidate) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM maintenance_candidates WHERE rule_id = ?`, ruleID); err != nil {
		return fmt.Errorf("clear candidates: %w", err)
	}

	if len(candidates) == 0 {
		return tx.Commit()
	}

	now := time.Now().UTC()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO maintenance_candidates (rule_id, library_item_id, reason, computed_at)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, c := range candidates {
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

	query := `
		SELECT ` + candidateSelectColumns + `
		FROM maintenance_candidates c
		JOIN library_items i ON c.library_item_id = i.id
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
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+candidateSelectColumns+`
		FROM maintenance_candidates c
		JOIN library_items i ON c.library_item_id = i.id
		LEFT JOIN maintenance_exclusions e ON c.rule_id = e.rule_id AND c.library_item_id = e.library_item_id
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
