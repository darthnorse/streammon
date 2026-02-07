package store

import (
	"context"
	"fmt"
	"time"

	"streammon/internal/models"
)

const exclusionSelectColumns = `
	e.id, e.rule_id, e.library_item_id, e.excluded_by, e.excluded_at,
	i.id, i.server_id, i.library_id, i.item_id, i.media_type, i.title, i.year,
	i.added_at, i.video_resolution, i.file_size, i.episode_count, i.thumb_url, i.synced_at`

func scanExclusion(scanner interface{ Scan(...any) error }) (models.MaintenanceExclusion, error) {
	var e models.MaintenanceExclusion
	var item models.LibraryItemCache
	err := scanner.Scan(&e.ID, &e.RuleID, &e.LibraryItemID, &e.ExcludedBy, &e.ExcludedAt,
		&item.ID, &item.ServerID, &item.LibraryID, &item.ItemID, &item.MediaType,
		&item.Title, &item.Year, &item.AddedAt, &item.VideoResolution, &item.FileSize,
		&item.EpisodeCount, &item.ThumbURL, &item.SyncedAt)
	if err != nil {
		return e, err
	}
	e.Item = &item
	return e, nil
}

// CreateExclusions bulk creates exclusions for a rule, returns count of newly created exclusions.
// Uses a single transaction to ensure accurate count even under concurrent modifications.
func (s *Store) CreateExclusions(ctx context.Context, ruleID int64, libraryItemIDs []int64, excludedBy string) (int, error) {
	if len(libraryItemIDs) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Count existing exclusions before insert (within transaction for consistency)
	var beforeCount int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maintenance_exclusions WHERE rule_id = ?`, ruleID).Scan(&beforeCount)
	if err != nil {
		return 0, fmt.Errorf("count before: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO maintenance_exclusions (rule_id, library_item_id, excluded_by, excluded_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(rule_id, library_item_id) DO NOTHING`)
	if err != nil {
		return 0, fmt.Errorf("prepare: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now().UTC()
	for _, itemID := range libraryItemIDs {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		if _, err := stmt.ExecContext(ctx, ruleID, itemID, excludedBy, now); err != nil {
			return 0, fmt.Errorf("insert exclusion: %w", err)
		}
	}

	// Count after insert (within same transaction for accurate diff)
	var afterCount int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maintenance_exclusions WHERE rule_id = ?`, ruleID).Scan(&afterCount)
	if err != nil {
		return 0, fmt.Errorf("count after: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	return afterCount - beforeCount, nil
}

// DeleteExclusion removes a single exclusion
func (s *Store) DeleteExclusion(ctx context.Context, ruleID, libraryItemID int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM maintenance_exclusions
		WHERE rule_id = ? AND library_item_id = ?`, ruleID, libraryItemID)
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
// Uses a single transaction to ensure accurate count even under concurrent modifications.
func (s *Store) DeleteExclusions(ctx context.Context, ruleID int64, libraryItemIDs []int64) (int, error) {
	if len(libraryItemIDs) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Count existing exclusions before delete (within transaction for consistency)
	var beforeCount int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maintenance_exclusions WHERE rule_id = ?`, ruleID).Scan(&beforeCount)
	if err != nil {
		return 0, fmt.Errorf("count before: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		DELETE FROM maintenance_exclusions
		WHERE rule_id = ? AND library_item_id = ?`)
	if err != nil {
		return 0, fmt.Errorf("prepare: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, itemID := range libraryItemIDs {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		if _, err := stmt.ExecContext(ctx, ruleID, itemID); err != nil {
			return 0, fmt.Errorf("delete exclusion: %w", err)
		}
	}

	// Count after delete (within same transaction for accurate diff)
	var afterCount int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maintenance_exclusions WHERE rule_id = ?`, ruleID).Scan(&afterCount)
	if err != nil {
		return 0, fmt.Errorf("count after: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	return beforeCount - afterCount, nil
}

// CountExclusionsForRule returns the count of exclusions for a rule
func (s *Store) CountExclusionsForRule(ctx context.Context, ruleID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maintenance_exclusions WHERE rule_id = ?`, ruleID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count exclusions: %w", err)
	}
	return count, nil
}

func (s *Store) ListExclusionsForRule(ctx context.Context, ruleID int64, page, perPage int, search string) (*models.PaginatedResult[models.MaintenanceExclusion], error) {
	var total int
	var args []any

	baseWhere := `e.rule_id = ?`
	args = append(args, ruleID)

	if search != "" {
		searchPattern := "%" + search + "%"
		baseWhere += ` AND (i.title LIKE ? OR CAST(i.year AS TEXT) LIKE ?)`
		args = append(args, searchPattern, searchPattern)
	}

	countQuery := `
		SELECT COUNT(*) FROM maintenance_exclusions e
		JOIN library_items i ON e.library_item_id = i.id
		WHERE ` + baseWhere
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count exclusions: %w", err)
	}

	offset := (page - 1) * perPage
	listArgs := append(args, perPage, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+exclusionSelectColumns+`
		FROM maintenance_exclusions e
		JOIN library_items i ON e.library_item_id = i.id
		WHERE `+baseWhere+`
		ORDER BY e.excluded_at DESC
		LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, fmt.Errorf("list exclusions: %w", err)
	}
	defer rows.Close()

	var exclusions []models.MaintenanceExclusion
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
		PerPage: perPage,
	}, rows.Err()
}

// IsItemExcluded checks if a library item is excluded from a rule
func (s *Store) IsItemExcluded(ctx context.Context, ruleID, libraryItemID int64) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM maintenance_exclusions
		WHERE rule_id = ? AND library_item_id = ?`, ruleID, libraryItemID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check exclusion: %w", err)
	}
	return count > 0, nil
}

// GetExcludedItemIDs returns all excluded library item IDs for a rule
func (s *Store) GetExcludedItemIDs(ctx context.Context, ruleID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT library_item_id FROM maintenance_exclusions WHERE rule_id = ?`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("get excluded item ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
