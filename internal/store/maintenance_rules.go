package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"streammon/internal/models"
)

const maintenanceRuleColumns = `id, server_id, library_id, name, criterion_type, parameters, enabled, created_at, updated_at`

func scanMaintenanceRule(scanner interface{ Scan(...any) error }) (models.MaintenanceRule, error) {
	var rule models.MaintenanceRule
	var params string
	var enabled int
	err := scanner.Scan(&rule.ID, &rule.ServerID, &rule.LibraryID, &rule.Name,
		&rule.CriterionType, &params, &enabled, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return rule, err
	}
	rule.Parameters = json.RawMessage(params)
	rule.Enabled = intToBool(enabled)
	return rule, nil
}

// CreateMaintenanceRule creates a new maintenance rule
func (s *Store) CreateMaintenanceRule(ctx context.Context, input *models.MaintenanceRuleInput) (*models.MaintenanceRule, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid maintenance rule: %w", err)
	}
	params := "{}"
	if len(input.Parameters) > 0 {
		params = string(input.Parameters)
	}
	enabled := boolToInt(input.Enabled)
	now := time.Now().UTC()

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO maintenance_rules (server_id, library_id, name, criterion_type, parameters, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		input.ServerID, input.LibraryID, input.Name, input.CriterionType, params, enabled, now, now)
	if err != nil {
		return nil, fmt.Errorf("create maintenance rule: %w", err)
	}

	id, _ := result.LastInsertId()
	return &models.MaintenanceRule{
		ID:            id,
		ServerID:      input.ServerID,
		LibraryID:     input.LibraryID,
		Name:          input.Name,
		CriterionType: input.CriterionType,
		Parameters:    json.RawMessage(params),
		Enabled:       input.Enabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// GetMaintenanceRule returns a rule by ID
func (s *Store) GetMaintenanceRule(ctx context.Context, id int64) (*models.MaintenanceRule, error) {
	rule, err := scanMaintenanceRule(s.db.QueryRowContext(ctx,
		`SELECT `+maintenanceRuleColumns+` FROM maintenance_rules WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get maintenance rule: %w", err)
	}
	return &rule, nil
}

// UpdateMaintenanceRule updates an existing rule
func (s *Store) UpdateMaintenanceRule(ctx context.Context, id int64, input *models.MaintenanceRuleUpdateInput) (*models.MaintenanceRule, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid maintenance rule: %w", err)
	}
	params := "{}"
	if len(input.Parameters) > 0 {
		params = string(input.Parameters)
	}
	enabled := boolToInt(input.Enabled)
	now := time.Now().UTC()

	result, err := s.db.ExecContext(ctx, `
		UPDATE maintenance_rules SET name = ?, criterion_type = ?, parameters = ?, enabled = ?, updated_at = ?
		WHERE id = ?`,
		input.Name, input.CriterionType, params, enabled, now, id)
	if err != nil {
		return nil, fmt.Errorf("update maintenance rule: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("maintenance rule %d: %w", id, models.ErrNotFound)
	}

	return s.GetMaintenanceRule(ctx, id)
}

// DeleteMaintenanceRule deletes a rule and its candidates
func (s *Store) DeleteMaintenanceRule(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM maintenance_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete maintenance rule: %w", err)
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

// ListMaintenanceRules returns all rules, optionally filtered by server/library
func (s *Store) ListMaintenanceRules(ctx context.Context, serverID int64, libraryID string) ([]models.MaintenanceRule, error) {
	query := `SELECT ` + maintenanceRuleColumns + ` FROM maintenance_rules WHERE 1=1`
	var args []any

	if serverID > 0 {
		query += ` AND server_id = ?`
		args = append(args, serverID)
	}
	if libraryID != "" {
		query += ` AND library_id = ?`
		args = append(args, libraryID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list maintenance rules: %w", err)
	}
	defer rows.Close()

	var rules []models.MaintenanceRule
	for rows.Next() {
		rule, err := scanMaintenanceRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// ListMaintenanceRulesWithCounts returns rules with candidate counts
func (s *Store) ListMaintenanceRulesWithCounts(ctx context.Context, serverID int64, libraryID string) ([]models.MaintenanceRuleWithCount, error) {
	query := `SELECT r.id, r.server_id, r.library_id, r.name, r.criterion_type, r.parameters,
		r.enabled, r.created_at, r.updated_at, COUNT(c.id) as candidate_count
		FROM maintenance_rules r
		LEFT JOIN maintenance_candidates c ON r.id = c.rule_id
		WHERE 1=1`
	var args []any

	if serverID > 0 {
		query += ` AND r.server_id = ?`
		args = append(args, serverID)
	}
	if libraryID != "" {
		query += ` AND r.library_id = ?`
		args = append(args, libraryID)
	}
	query += ` GROUP BY r.id ORDER BY r.created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list maintenance rules with counts: %w", err)
	}
	defer rows.Close()

	var rules []models.MaintenanceRuleWithCount
	for rows.Next() {
		var rule models.MaintenanceRuleWithCount
		var params string
		var enabled int
		err := rows.Scan(&rule.ID, &rule.ServerID, &rule.LibraryID, &rule.Name,
			&rule.CriterionType, &params, &enabled, &rule.CreatedAt, &rule.UpdatedAt,
			&rule.CandidateCount)
		if err != nil {
			return nil, err
		}
		rule.Parameters = json.RawMessage(params)
		rule.Enabled = intToBool(enabled)
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}
