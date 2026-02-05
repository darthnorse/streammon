package store

import (
	"context"
	"fmt"
	"time"

	"streammon/internal/models"
)

// UpsertMaintenanceCandidate inserts or updates a candidate
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

// DeleteCandidatesForRule removes all candidates for a rule
func (s *Store) DeleteCandidatesForRule(ctx context.Context, ruleID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM maintenance_candidates WHERE rule_id = ?`, ruleID)
	if err != nil {
		return fmt.Errorf("delete candidates for rule: %w", err)
	}
	return nil
}

// ListCandidatesForRule returns candidates with their library items
func (s *Store) ListCandidatesForRule(ctx context.Context, ruleID int64, page, perPage int) (*models.PaginatedResult[models.MaintenanceCandidate], error) {
	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM maintenance_candidates WHERE rule_id = ?`, ruleID).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count candidates: %w", err)
	}

	offset := (page - 1) * perPage
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.rule_id, c.library_item_id, c.reason, c.computed_at,
			i.id, i.server_id, i.library_id, i.item_id, i.media_type, i.title, i.year,
			i.added_at, i.video_resolution, i.file_size, i.episode_count, i.thumb_url, i.synced_at
		FROM maintenance_candidates c
		JOIN library_items i ON c.library_item_id = i.id
		WHERE c.rule_id = ?
		ORDER BY i.added_at DESC
		LIMIT ? OFFSET ?`,
		ruleID, perPage, offset)
	if err != nil {
		return nil, fmt.Errorf("list candidates: %w", err)
	}
	defer rows.Close()

	var candidates []models.MaintenanceCandidate
	for rows.Next() {
		var c models.MaintenanceCandidate
		var item models.LibraryItemCache
		err := rows.Scan(&c.ID, &c.RuleID, &c.LibraryItemID, &c.Reason, &c.ComputedAt,
			&item.ID, &item.ServerID, &item.LibraryID, &item.ItemID, &item.MediaType,
			&item.Title, &item.Year, &item.AddedAt, &item.VideoResolution, &item.FileSize,
			&item.EpisodeCount, &item.ThumbURL, &item.SyncedAt)
		if err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		c.Item = &item
		candidates = append(candidates, c)
	}

	return &models.PaginatedResult[models.MaintenanceCandidate]{
		Items:   candidates,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	}, rows.Err()
}

// CountCandidatesForRule returns the count of candidates for a rule
func (s *Store) CountCandidatesForRule(ctx context.Context, ruleID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM maintenance_candidates WHERE rule_id = ?`, ruleID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count candidates for rule: %w", err)
	}
	return count, nil
}

// BatchUpsertCandidates inserts multiple candidates in a transaction
func (s *Store) BatchUpsertCandidates(ctx context.Context, ruleID int64, candidates []struct {
	LibraryItemID int64
	Reason        string
}) error {
	if len(candidates) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Clear existing candidates for this rule
	if _, err := tx.ExecContext(ctx, `DELETE FROM maintenance_candidates WHERE rule_id = ?`, ruleID); err != nil {
		return fmt.Errorf("clear candidates: %w", err)
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
