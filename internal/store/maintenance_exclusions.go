package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"streammon/internal/models"
)

const exclusionSelectColumns = `
	e.id, e.library_item_id, e.excluded_by, e.excluded_at,
	i.id, i.server_id, i.library_id, i.item_id, i.media_type, i.title, i.year,
	i.added_at, i.last_watched_at, i.video_resolution, i.file_size, i.episode_count, i.thumb_url, i.synced_at`

func scanExclusion(scanner interface{ Scan(...any) error }) (models.MaintenanceExclusion, error) {
	var e models.MaintenanceExclusion
	var item models.LibraryItemCache
	var lastWatchedAt sql.NullString
	err := scanner.Scan(&e.ID, &e.LibraryItemID, &e.ExcludedBy, &e.ExcludedAt,
		&item.ID, &item.ServerID, &item.LibraryID, &item.ItemID, &item.MediaType,
		&item.Title, &item.Year, &item.AddedAt, &lastWatchedAt, &item.VideoResolution, &item.FileSize,
		&item.EpisodeCount, &item.ThumbURL, &item.SyncedAt)
	if err != nil {
		return e, err
	}
	if lastWatchedAt.Valid && lastWatchedAt.String != "" {
		t, parseErr := parseSQLiteTime(lastWatchedAt.String)
		if parseErr == nil {
			item.LastWatchedAt = &t
		}
	}
	e.Item = &item
	return e, nil
}

var validExclusionSortColumns = map[string]string{
	"title":       "i.title",
	"type":        "i.media_type",
	"year":        "i.year",
	"size":        "i.file_size",
	"excluded_at": "e.excluded_at",
	"excluded_by": "e.excluded_by",
}

// CreateExclusions bulk creates global exclusions, returns count of newly created exclusions.
// Uses a single transaction to ensure accurate count even under concurrent modifications.
func (s *Store) CreateExclusions(ctx context.Context, libraryItemIDs []int64, excludedBy string) (int, error) {
	if len(libraryItemIDs) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	placeholders := make([]string, len(libraryItemIDs))
	countArgs := make([]any, len(libraryItemIDs))
	for i, id := range libraryItemIDs {
		placeholders[i] = "?"
		countArgs[i] = id
	}
	inClause := strings.Join(placeholders, ",")

	var beforeCount int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maintenance_exclusions WHERE library_item_id IN (`+inClause+`)`, countArgs...).Scan(&beforeCount)
	if err != nil {
		return 0, fmt.Errorf("count before: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO maintenance_exclusions (library_item_id, excluded_by, excluded_at)
		VALUES (?, ?, ?)
		ON CONFLICT(library_item_id) DO NOTHING`)
	if err != nil {
		return 0, fmt.Errorf("prepare: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now().UTC()
	for _, itemID := range libraryItemIDs {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		if _, err := stmt.ExecContext(ctx, itemID, excludedBy, now); err != nil {
			return 0, fmt.Errorf("insert exclusion: %w", err)
		}
	}

	var afterCount int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maintenance_exclusions WHERE library_item_id IN (`+inClause+`)`, countArgs...).Scan(&afterCount)
	if err != nil {
		return 0, fmt.Errorf("count after: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	return afterCount - beforeCount, nil
}

func (s *Store) DeleteExclusion(ctx context.Context, libraryItemID int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM maintenance_exclusions
		WHERE library_item_id = ?`, libraryItemID)
	if err != nil {
		return fmt.Errorf("delete exclusion: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if n == 0 {
		return models.ErrNotFound
	}
	return nil
}

// DeleteExclusions bulk removes exclusions, returns count of actually removed exclusions.
func (s *Store) DeleteExclusions(ctx context.Context, libraryItemIDs []int64) (int, error) {
	if len(libraryItemIDs) == 0 {
		return 0, nil
	}

	placeholders := make([]string, len(libraryItemIDs))
	args := make([]any, len(libraryItemIDs))
	for i, id := range libraryItemIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	inClause := strings.Join(placeholders, ",")

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM maintenance_exclusions WHERE library_item_id IN (`+inClause+`)`, args...)
	if err != nil {
		return 0, fmt.Errorf("delete exclusions: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}

	return int(n), nil
}

func (s *Store) CountExclusions(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maintenance_exclusions`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count exclusions: %w", err)
	}
	return count, nil
}

func (s *Store) ListExclusions(ctx context.Context, opts models.ExclusionListOptions) (*models.PaginatedResult[models.MaintenanceExclusion], error) {
	var total int
	var args []any

	baseWhere := `1=1`

	if opts.Search != "" {
		searchPattern := "%" + escapeLikePattern(opts.Search) + "%"
		baseWhere += ` AND (i.title LIKE ? ESCAPE '\' OR CAST(i.year AS TEXT) LIKE ? ESCAPE '\' OR i.video_resolution LIKE ? ESCAPE '\')`
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	countQuery := `
		SELECT COUNT(*) FROM maintenance_exclusions e
		JOIN library_items i ON e.library_item_id = i.id
		WHERE ` + baseWhere
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count exclusions: %w", err)
	}

	orderBy := "e.excluded_at DESC"
	if col, ok := validExclusionSortColumns[opts.SortBy]; ok {
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
		SELECT `+exclusionSelectColumns+`
		FROM maintenance_exclusions e
		JOIN library_items i ON e.library_item_id = i.id
		WHERE `+baseWhere+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, fmt.Errorf("list exclusions: %w", err)
	}
	defer rows.Close()

	exclusions := []models.MaintenanceExclusion{}
	for rows.Next() {
		e, err := scanExclusion(rows)
		if err != nil {
			return nil, fmt.Errorf("scan exclusion: %w", err)
		}
		exclusions = append(exclusions, e)
	}

	return &models.PaginatedResult[models.MaintenanceExclusion]{
		Items:   exclusions,
		Total:   total,
		Page:    page,
		PerPage: opts.PerPage,
	}, rows.Err()
}

// ListExcludedCandidatesForRule returns excluded items that are also candidates for a specific rule.
func (s *Store) ListExcludedCandidatesForRule(ctx context.Context, ruleID int64, opts models.ExclusionListOptions) (*models.PaginatedResult[models.MaintenanceExclusion], error) {
	var total int
	var args []any

	baseWhere := `c.rule_id = ?`
	args = append(args, ruleID)

	if opts.Search != "" {
		searchPattern := "%" + escapeLikePattern(opts.Search) + "%"
		baseWhere += ` AND (i.title LIKE ? ESCAPE '\' OR CAST(i.year AS TEXT) LIKE ? ESCAPE '\' OR i.video_resolution LIKE ? ESCAPE '\')`
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	countQuery := `
		SELECT COUNT(*) FROM maintenance_exclusions e
		JOIN library_items i ON e.library_item_id = i.id
		JOIN maintenance_candidates c ON c.library_item_id = e.library_item_id
		WHERE ` + baseWhere
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count excluded candidates: %w", err)
	}

	orderBy := "e.excluded_at DESC"
	if col, ok := validExclusionSortColumns[opts.SortBy]; ok {
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
		SELECT `+exclusionSelectColumns+`
		FROM maintenance_exclusions e
		JOIN library_items i ON e.library_item_id = i.id
		JOIN maintenance_candidates c ON c.library_item_id = e.library_item_id
		WHERE `+baseWhere+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, fmt.Errorf("list excluded candidates: %w", err)
	}
	defer rows.Close()

	exclusions := []models.MaintenanceExclusion{}
	for rows.Next() {
		e, err := scanExclusion(rows)
		if err != nil {
			return nil, fmt.Errorf("scan exclusion: %w", err)
		}
		exclusions = append(exclusions, e)
	}

	return &models.PaginatedResult[models.MaintenanceExclusion]{
		Items:   exclusions,
		Total:   total,
		Page:    page,
		PerPage: opts.PerPage,
	}, rows.Err()
}

func (s *Store) IsItemExcluded(ctx context.Context, libraryItemID int64) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maintenance_exclusions
		WHERE library_item_id = ?`, libraryItemID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check exclusion: %w", err)
	}
	return count > 0, nil
}
