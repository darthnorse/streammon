package store

import (
	"database/sql"
	"errors"
	"fmt"

	"streammon/internal/models"
)

const serverColumns = `id, name, type, url, api_key, enabled, created_at, updated_at`

func scanServer(scanner interface{ Scan(...any) error }) (models.Server, error) {
	var srv models.Server
	err := scanner.Scan(&srv.ID, &srv.Name, &srv.Type, &srv.URL, &srv.APIKey, &srv.Enabled, &srv.CreatedAt, &srv.UpdatedAt)
	return srv, err
}

func (s *Store) CreateServer(srv *models.Server) error {
	result, err := s.db.Exec(
		`INSERT INTO servers (name, type, url, api_key, enabled) VALUES (?, ?, ?, ?, ?)`,
		srv.Name, srv.Type, srv.URL, srv.APIKey, srv.Enabled,
	)
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	srv.ID = id
	return nil
}

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

func (s *Store) ListServers() ([]models.Server, error) {
	rows, err := s.db.Query(`SELECT ` + serverColumns + ` FROM servers ORDER BY id`)
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

func (s *Store) UpdateServer(srv *models.Server) error {
	updated, err := scanServer(s.db.QueryRow(
		`UPDATE servers SET name = ?, type = ?, url = ?, api_key = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? RETURNING `+serverColumns,
		srv.Name, srv.Type, srv.URL, srv.APIKey, srv.Enabled, srv.ID,
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

func (s *Store) DeleteServer(id int64) error {
	result, err := s.db.Exec(`DELETE FROM servers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting server: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("server %d: %w", id, models.ErrNotFound)
	}
	return nil
}
