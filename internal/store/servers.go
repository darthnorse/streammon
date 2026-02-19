package store

import (
	"database/sql"
	"errors"
	"fmt"

	"streammon/internal/models"
)

const serverColumns = `id, name, type, url, api_key, machine_id, enabled, show_recent_media, created_at, updated_at, deleted_at`

func scanServer(scanner interface{ Scan(...any) error }) (models.Server, error) {
	var srv models.Server
	var deletedAt sql.NullTime
	err := scanner.Scan(&srv.ID, &srv.Name, &srv.Type, &srv.URL, &srv.APIKey, &srv.MachineID, &srv.Enabled, &srv.ShowRecentMedia, &srv.CreatedAt, &srv.UpdatedAt, &deletedAt)
	if deletedAt.Valid {
		srv.DeletedAt = &deletedAt.Time
	}
	return srv, err
}

func (s *Store) CreateServer(srv *models.Server) error {
	created, err := scanServer(s.db.QueryRow(
		`INSERT INTO servers (name, type, url, api_key, machine_id, enabled, show_recent_media) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING `+serverColumns,
		srv.Name, srv.Type, srv.URL, srv.APIKey, srv.MachineID, srv.Enabled, srv.ShowRecentMedia,
	))
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}
	*srv = created
	return nil
}

// GetServer returns a server by ID, including soft-deleted servers.
// Callers that need to guard against deleted servers should check DeletedAt.
func (s *Store) GetServer(id int64) (*models.Server, error) {
	srv, err := scanServer(s.db.QueryRow(
		`SELECT `+serverColumns+` FROM servers WHERE id = ?`, id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("server %d: %w", id, models.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("getting server: %w", err)
	}
	return &srv, nil
}

func (s *Store) listServers(query string) ([]models.Server, error) {
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("listing servers: %w", err)
	}
	defer rows.Close()

	servers := []models.Server{}
	for rows.Next() {
		srv, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, srv)
	}
	return servers, rows.Err()
}

// ListServers returns only active (non-deleted) servers.
func (s *Store) ListServers() ([]models.Server, error) {
	return s.listServers(`SELECT ` + serverColumns + ` FROM servers WHERE deleted_at IS NULL ORDER BY id`)
}

// ListAllServers returns all servers including soft-deleted ones.
func (s *Store) ListAllServers() ([]models.Server, error) {
	return s.listServers(`SELECT ` + serverColumns + ` FROM servers ORDER BY id`)
}

func (s *Store) UpdateServer(srv *models.Server) error {
	updated, err := scanServer(s.db.QueryRow(
		`UPDATE servers SET name = ?, type = ?, url = ?, api_key = ?, machine_id = ?, enabled = ?, show_recent_media = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? RETURNING `+serverColumns,
		srv.Name, srv.Type, srv.URL, srv.APIKey, srv.MachineID, srv.Enabled, srv.ShowRecentMedia, srv.ID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("server %d: %w", srv.ID, models.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("updating server: %w", err)
	}
	*srv = updated
	return nil
}

// UpdateServerAtomic updates a server and clears maintenance data if identity changed.
// Identity is defined by URL, Type, and MachineID - if any of these change, cached
// library items and maintenance rules are cleared to prevent stale data from causing
// deletions on wrong content. All operations happen in a single transaction.
func (s *Store) UpdateServerAtomic(existing, srv *models.Server) error {
	identityChanged := existing.URL != srv.URL ||
		existing.Type != srv.Type ||
		existing.MachineID != srv.MachineID

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if identityChanged {
		if _, err := tx.Exec(`DELETE FROM library_items WHERE server_id = ?`, srv.ID); err != nil {
			return fmt.Errorf("delete library items: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM maintenance_rule_libraries WHERE server_id = ?`, srv.ID); err != nil {
			return fmt.Errorf("delete rule library associations: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM maintenance_rules WHERE id NOT IN (SELECT DISTINCT rule_id FROM maintenance_rule_libraries)`); err != nil {
			return fmt.Errorf("delete orphaned rules: %w", err)
		}
	}

	updated, err := scanServer(tx.QueryRow(
		`UPDATE servers SET name = ?, type = ?, url = ?, api_key = ?, machine_id = ?, enabled = ?, show_recent_media = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? RETURNING `+serverColumns,
		srv.Name, srv.Type, srv.URL, srv.APIKey, srv.MachineID, srv.Enabled, srv.ShowRecentMedia, srv.ID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("server %d: %w", srv.ID, models.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("updating server: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	*srv = updated
	return nil
}

// DeleteServer permanently removes a server and all its watch history.
// Works on both active and soft-deleted servers.
func (s *Store) DeleteServer(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("deleting server: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM watch_history WHERE server_id = ?`, id); err != nil {
		return fmt.Errorf("deleting server history: %w", err)
	}
	result, err := tx.Exec(`DELETE FROM servers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting server: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("server %d: %w", id, models.ErrNotFound)
	}

	if _, err := tx.Exec(`DELETE FROM maintenance_rules WHERE id NOT IN (SELECT DISTINCT rule_id FROM maintenance_rule_libraries)`); err != nil {
		return fmt.Errorf("delete orphaned rules: %w", err)
	}

	return tx.Commit()
}

// SoftDeleteServer marks a server as deleted without removing any data.
func (s *Store) SoftDeleteServer(id int64) error {
	result, err := s.db.Exec(
		`UPDATE servers SET deleted_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft-deleting server: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("server %d: %w", id, models.ErrNotFound)
	}
	return nil
}

// RestoreServer clears the deleted_at timestamp, making the server active again.
func (s *Store) RestoreServer(id int64) error {
	result, err := s.db.Exec(
		`UPDATE servers SET deleted_at = NULL WHERE id = ? AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return fmt.Errorf("restoring server: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("server %d: %w", id, models.ErrNotFound)
	}
	return nil
}
