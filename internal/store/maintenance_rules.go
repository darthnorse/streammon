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

const maintenanceRuleColumns = `id, name, media_type, criterion_type, parameters, enabled, created_at, updated_at`

// querier is satisfied by both *sql.DB and *sql.Tx
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func scanMaintenanceRule(scanner interface{ Scan(...any) error }) (models.MaintenanceRule, error) {
	var rule models.MaintenanceRule
	var params string
	var enabled int
	err := scanner.Scan(&rule.ID, &rule.Name, &rule.MediaType,
		&rule.CriterionType, &params, &enabled, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return rule, err
	}
	rule.Parameters = json.RawMessage(params)
	rule.Enabled = intToBool(enabled)
	return rule, nil
}

// loadRuleLibraries fetches junction rows for the given rule IDs and returns a map of ruleID -> []RuleLibrary.
func loadRuleLibraries(ctx context.Context, db querier, ruleIDs ...int64) (map[int64][]models.RuleLibrary, error) {
	if len(ruleIDs) == 0 {
		return nil, nil
	}

	query := `SELECT rule_id, server_id, library_id FROM maintenance_rule_libraries WHERE rule_id IN (`
	args := make([]any, len(ruleIDs))
	for i, id := range ruleIDs {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += `) ORDER BY rule_id, server_id, library_id`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load rule libraries: %w", err)
	}
	defer rows.Close()

	result := make(map[int64][]models.RuleLibrary)
	for rows.Next() {
		var ruleID int64
		var lib models.RuleLibrary
		if err := rows.Scan(&ruleID, &lib.ServerID, &lib.LibraryID); err != nil {
			return nil, fmt.Errorf("scan rule library: %w", err)
		}
		result[ruleID] = append(result[ruleID], lib)
	}
	return result, rows.Err()
}

// setRuleLibraries replaces all junction rows for a rule within a transaction.
func setRuleLibraries(ctx context.Context, tx *sql.Tx, ruleID int64, libs []models.RuleLibrary) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM maintenance_rule_libraries WHERE rule_id = ?`, ruleID); err != nil {
		return fmt.Errorf("clear rule libraries: %w", err)
	}

	if len(libs) == 0 {
		return nil
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO maintenance_rule_libraries (rule_id, server_id, library_id)
		VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare rule library insert: %w", err)
	}
	defer stmt.Close()

	for _, lib := range libs {
		if _, err := stmt.ExecContext(ctx, ruleID, lib.ServerID, lib.LibraryID); err != nil {
			return fmt.Errorf("insert rule library: %w", err)
		}
	}
	return nil
}

// CreateMaintenanceRule creates a new maintenance rule with its library associations.
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO maintenance_rules (name, media_type, criterion_type, parameters, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		input.Name, input.MediaType, input.CriterionType, params, enabled, now, now)
	if err != nil {
		return nil, fmt.Errorf("create maintenance rule: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}

	if err := setRuleLibraries(ctx, tx, id, input.Libraries); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &models.MaintenanceRule{
		ID:            id,
		Name:          input.Name,
		MediaType:     input.MediaType,
		CriterionType: input.CriterionType,
		Parameters:    json.RawMessage(params),
		Enabled:       input.Enabled,
		Libraries:     input.Libraries,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// GetMaintenanceRule returns a rule by ID with its library associations.
func (s *Store) GetMaintenanceRule(ctx context.Context, id int64) (*models.MaintenanceRule, error) {
	rule, err := scanMaintenanceRule(s.db.QueryRowContext(ctx,
		`SELECT `+maintenanceRuleColumns+` FROM maintenance_rules WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get maintenance rule: %w", err)
	}

	libs, err := loadRuleLibraries(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	rule.Libraries = libs[id]
	if rule.Libraries == nil {
		rule.Libraries = []models.RuleLibrary{}
	}
	return &rule, nil
}

// UpdateMaintenanceRule updates an existing rule and optionally its library associations.
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
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

	// Always rewrite library associations — the client must send the full list.
	// Validation ensures at least one library is present.
	if err := setRuleLibraries(ctx, tx, id, input.Libraries); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s.GetMaintenanceRule(ctx, id)
}

// DeleteMaintenanceRule deletes a rule and its candidates (CASCADE handles junction cleanup).
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

// ListMaintenanceRules returns all rules, optionally filtered by server/library via junction table.
func (s *Store) ListMaintenanceRules(ctx context.Context, serverID int64, libraryID string) ([]models.MaintenanceRule, error) {
	query := `SELECT DISTINCT r.` + maintenanceRuleColumns + ` FROM maintenance_rules r`
	var args []any

	if serverID > 0 || libraryID != "" {
		query += ` JOIN maintenance_rule_libraries mrl ON r.id = mrl.rule_id`
		if serverID > 0 {
			query += ` AND mrl.server_id = ?`
			args = append(args, serverID)
		}
		if libraryID != "" {
			query += ` AND mrl.library_id = ?`
			args = append(args, libraryID)
		}
	}
	query += ` ORDER BY r.created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list maintenance rules: %w", err)
	}
	defer rows.Close()

	var rules []models.MaintenanceRule
	var ruleIDs []int64
	for rows.Next() {
		rule, err := scanMaintenanceRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
		ruleIDs = append(ruleIDs, rule.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ruleIDs) > 0 {
		libs, err := loadRuleLibraries(ctx, s.db, ruleIDs...)
		if err != nil {
			return nil, err
		}
		for i := range rules {
			rules[i].Libraries = libs[rules[i].ID]
			if rules[i].Libraries == nil {
				rules[i].Libraries = []models.RuleLibrary{}
			}
		}
	}

	return rules, nil
}

// ListMaintenanceRulesWithCounts returns rules with candidate counts, optionally filtered by server/library.
func (s *Store) ListMaintenanceRulesWithCounts(ctx context.Context, serverID int64, libraryID string) ([]models.MaintenanceRuleWithCount, error) {
	query := `SELECT r.id, r.name, r.media_type, r.criterion_type, r.parameters,
		r.enabled, r.created_at, r.updated_at, COUNT(DISTINCT c.id) as candidate_count,
		(SELECT COUNT(*) FROM maintenance_exclusions e2 WHERE e2.rule_id = r.id) as exclusion_count
		FROM maintenance_rules r
		LEFT JOIN maintenance_candidates c ON r.id = c.rule_id
			AND NOT EXISTS (
				SELECT 1 FROM maintenance_exclusions e
				WHERE e.rule_id = c.rule_id AND e.library_item_id = c.library_item_id
			)`

	var args []any
	if serverID > 0 || libraryID != "" {
		query += ` JOIN maintenance_rule_libraries mrl ON r.id = mrl.rule_id`
		if serverID > 0 {
			query += ` AND mrl.server_id = ?`
			args = append(args, serverID)
		}
		if libraryID != "" {
			query += ` AND mrl.library_id = ?`
			args = append(args, libraryID)
		}
	}
	query += ` GROUP BY r.id ORDER BY r.created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list maintenance rules with counts: %w", err)
	}
	defer rows.Close()

	var rules []models.MaintenanceRuleWithCount
	var ruleIDs []int64
	for rows.Next() {
		var rule models.MaintenanceRuleWithCount
		var params string
		var enabled int
		err := rows.Scan(&rule.ID, &rule.Name, &rule.MediaType,
			&rule.CriterionType, &params, &enabled, &rule.CreatedAt, &rule.UpdatedAt,
			&rule.CandidateCount, &rule.ExclusionCount)
		if err != nil {
			return nil, err
		}
		rule.Parameters = json.RawMessage(params)
		rule.Enabled = intToBool(enabled)
		rules = append(rules, rule)
		ruleIDs = append(ruleIDs, rule.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ruleIDs) > 0 {
		libs, err := loadRuleLibraries(ctx, s.db, ruleIDs...)
		if err != nil {
			return nil, err
		}
		for i := range rules {
			rules[i].Libraries = libs[rules[i].ID]
			if rules[i].Libraries == nil {
				rules[i].Libraries = []models.RuleLibrary{}
			}
		}
	}

	return rules, nil
}

// ListAllMaintenanceRules returns all enabled maintenance rules with their libraries.
// Used by the scheduler to evaluate all rules across all servers.
func (s *Store) ListAllMaintenanceRules(ctx context.Context) ([]models.MaintenanceRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+maintenanceRuleColumns+` FROM maintenance_rules WHERE enabled = 1 ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list all maintenance rules: %w", err)
	}
	defer rows.Close()

	var rules []models.MaintenanceRule
	var ruleIDs []int64
	for rows.Next() {
		rule, err := scanMaintenanceRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
		ruleIDs = append(ruleIDs, rule.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ruleIDs) > 0 {
		libs, err := loadRuleLibraries(ctx, s.db, ruleIDs...)
		if err != nil {
			return nil, err
		}
		for i := range rules {
			rules[i].Libraries = libs[rules[i].ID]
			if rules[i].Libraries == nil {
				rules[i].Libraries = []models.RuleLibrary{}
			}
		}
	}

	return rules, nil
}

