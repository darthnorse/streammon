package store

import (
	"database/sql"
	"errors"
	"fmt"

	"streammon/internal/models"
)

const userColumns = `id, name, email, role, thumb_url, created_at, updated_at`

func scanUser(scanner interface{ Scan(...any) error }) (models.User, error) {
	var u models.User
	err := scanner.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.ThumbURL, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (s *Store) GetOrCreateUser(name string) (*models.User, error) {
	_, err := s.db.Exec(
		`INSERT INTO users (name) VALUES (?) ON CONFLICT(name) DO NOTHING`, name,
	)
	if err != nil {
		return nil, fmt.Errorf("upserting user: %w", err)
	}

	u, err := scanUser(s.db.QueryRow(
		`SELECT `+userColumns+` FROM users WHERE name = ?`, name,
	))
	if err != nil {
		return nil, fmt.Errorf("querying user: %w", err)
	}
	return &u, nil
}

func (s *Store) ListUsers() ([]models.User, error) {
	rows, err := s.db.Query(`SELECT ` + userColumns + ` FROM users ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) GetUser(name string) (*models.User, error) {
	u, err := scanUser(s.db.QueryRow(
		`SELECT `+userColumns+` FROM users WHERE name = ?`, name,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("user %q: %w", name, models.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}
	return &u, nil
}

func (s *Store) UpdateUserRole(name string, role models.Role) error {
	result, err := s.db.Exec(
		`UPDATE users SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?`, role, name,
	)
	if err != nil {
		return fmt.Errorf("updating user role: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user %q: %w", name, models.ErrNotFound)
	}
	return nil
}
