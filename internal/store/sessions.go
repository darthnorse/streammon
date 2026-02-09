package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"streammon/internal/models"
)

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Store) CreateSession(userID int64, expiresAt time.Time) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generating session token: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expiresAt.UTC(),
	)
	if err != nil {
		return "", fmt.Errorf("creating session: %w", err)
	}
	return token, nil
}

func (s *Store) GetSessionUser(token string) (*models.User, error) {
	u, err := scanUser(s.db.QueryRow(
		`SELECT u.id, u.name, u.email, u.role, u.thumb_url, u.created_at, u.updated_at FROM users u
		 INNER JOIN sessions s ON s.user_id = u.id
		 WHERE s.id = ? AND s.expires_at > ?`,
		token, time.Now().UTC(),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("session %q: %w", token, models.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("getting session user: %w", err)
	}
	return &u, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, token)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

func (s *Store) DeleteUserSessionsExcept(userID int64, exceptToken string) error {
	_, err := s.db.Exec(
		`DELETE FROM sessions WHERE user_id = ? AND id != ?`,
		userID, exceptToken,
	)
	if err != nil {
		return fmt.Errorf("deleting user sessions: %w", err)
	}
	return nil
}

func (s *Store) DeleteExpiredSessions() (int64, error) {
	result, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("deleting expired sessions: %w", err)
	}
	return result.RowsAffected()
}
