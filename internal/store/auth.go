package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"streammon/internal/models"
)

// GetUserByUsername retrieves a user by username along with password hash
func (s *Store) GetUserByUsername(username string) (*models.User, string, error) {
	var u models.User
	var passwordHash sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, email, role, thumb_url, created_at, updated_at, password_hash
		 FROM users WHERE name = ?`, username,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.ThumbURL, &u.CreatedAt, &u.UpdatedAt, &passwordHash)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", fmt.Errorf("user %q: %w", username, models.ErrNotFound)
	}
	if err != nil {
		return nil, "", fmt.Errorf("getting user by username: %w", err)
	}
	return &u, passwordHash.String, nil
}

// GetUserByProvider retrieves a user by provider type and provider-specific ID
func (s *Store) GetUserByProvider(provider, providerID string) (*models.User, error) {
	u, err := scanUser(s.db.QueryRow(
		`SELECT `+userColumns+` FROM users WHERE provider = ? AND provider_id = ?`,
		provider, providerID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("user: %w", models.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by provider: %w", err)
	}
	return &u, nil
}

// CreateLocalUser creates a new local user with password
func (s *Store) CreateLocalUser(username, email, passwordHash string, role models.Role) (*models.User, error) {
	result, err := s.db.Exec(
		`INSERT INTO users (name, email, password_hash, role, provider, provider_id)
		 VALUES (?, ?, ?, ?, 'local', ?)`,
		username, email, passwordHash, role, email,
	)
	if err != nil {
		return nil, fmt.Errorf("creating local user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert id: %w", err)
	}

	return &models.User{
		ID:        id,
		Name:      username,
		Email:     email,
		Role:      role,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// UpdatePassword updates a user's password hash
func (s *Store) UpdatePassword(userID int64, passwordHash string) error {
	result, err := s.db.Exec(
		`UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		passwordHash, userID,
	)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user %d: %w", userID, models.ErrNotFound)
	}
	return nil
}

// LinkProviderAccount links a provider identity to an existing user
func (s *Store) LinkProviderAccount(userID int64, provider, providerID string) error {
	_, err := s.db.Exec(
		`UPDATE users SET provider = ?, provider_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		provider, providerID, userID,
	)
	if err != nil {
		return fmt.Errorf("linking provider account: %w", err)
	}
	return nil
}

// maybeUpdateAvatar updates user's avatar if changed, logging warnings on failure
func (s *Store) maybeUpdateAvatar(user *models.User, thumbURL string) {
	if thumbURL != "" && thumbURL != user.ThumbURL {
		if err := s.UpdateUserAvatar(user.Name, thumbURL); err != nil {
			log.Printf("warning: failed to update avatar for %s: %v", user.Name, err)
		}
		user.ThumbURL = thumbURL
	}
}

// GetOrLinkUserByEmail finds user by provider ID first, then by email for account linking.
// SECURITY: Account linking by email is only performed for OAuth providers (plex, oidc)
// where the email is verified by the provider. Local accounts are not linked by email.
func (s *Store) GetOrLinkUserByEmail(email, name, provider, providerID, thumbURL string) (*models.User, error) {
	// First try to find by provider + providerID (exact match)
	if providerID != "" {
		user, err := s.GetUserByProvider(provider, providerID)
		if err == nil {
			s.maybeUpdateAvatar(user, thumbURL)
			return user, nil
		}
	}

	// Try to find by email for account linking
	// SECURITY: Only link for OAuth providers where email is verified
	if email != "" && provider != "local" {
		var existingUser models.User
		err := s.db.QueryRow(
			`SELECT `+userColumns+` FROM users WHERE email = ? AND email != ''`, email,
		).Scan(&existingUser.ID, &existingUser.Name, &existingUser.Email, &existingUser.Role,
			&existingUser.ThumbURL, &existingUser.CreatedAt, &existingUser.UpdatedAt)

		if err == nil {
			log.Printf("info: linking provider %s (id=%s) to existing user %s (id=%d) via email %s",
				provider, providerID, existingUser.Name, existingUser.ID, email)

			if providerID != "" {
				if err := s.LinkProviderAccount(existingUser.ID, provider, providerID); err != nil {
					return nil, fmt.Errorf("linking provider account: %w", err)
				}
			}
			s.maybeUpdateAvatar(&existingUser, thumbURL)
			return &existingUser, nil
		}
	}

	// No existing user found - create new one
	displayName := name
	if displayName == "" {
		displayName = email
	}
	if displayName == "" {
		return nil, fmt.Errorf("cannot create user: no name or email provided")
	}

	_, err := s.db.Exec(
		`INSERT INTO users (name, email, role, provider, provider_id, thumb_url)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		displayName, email, models.RoleViewer, provider, providerID, thumbURL,
	)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	return s.GetUser(displayName)
}

// IsSetupRequired returns true if no users exist (first run)
func (s *Store) IsSetupRequired() (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking user count: %w", err)
	}
	return count == 0, nil
}

// CreateFirstAdmin atomically creates the first admin user if no users exist.
// Returns ErrSetupComplete if setup is already complete.
var ErrSetupComplete = errors.New("setup already complete")

func (s *Store) CreateFirstAdmin(username, email, passwordHash, provider, providerID, thumbURL string) (*models.User, error) {
	// Use INSERT ... WHERE NOT EXISTS for atomic check-and-insert
	// This prevents race conditions without needing IMMEDIATE transactions
	result, err := s.db.Exec(
		`INSERT INTO users (name, email, password_hash, role, provider, provider_id, thumb_url)
		 SELECT ?, ?, ?, ?, ?, ?, ?
		 WHERE NOT EXISTS (SELECT 1 FROM users LIMIT 1)`,
		username, email, passwordHash, models.RoleAdmin, provider, providerID, thumbURL,
	)
	if err != nil {
		return nil, fmt.Errorf("creating first admin: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return nil, ErrSetupComplete
	}

	id, _ := result.LastInsertId()
	return &models.User{
		ID:        id,
		Name:      username,
		Email:     email,
		Role:      models.RoleAdmin,
		ThumbURL:  thumbURL,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// GetPlexGuestAccess returns whether Plex guests are allowed
func (s *Store) GetPlexGuestAccess() (bool, error) {
	val, err := s.GetSetting("auth.plex.guest_access")
	if err != nil {
		return false, nil // Default to false (require server access)
	}
	return val == "true", nil
}

// SetPlexGuestAccess sets whether Plex guests are allowed
func (s *Store) SetPlexGuestAccess(allowed bool) error {
	val := "false"
	if allowed {
		val = "true"
	}
	return s.SetSetting("auth.plex.guest_access", val)
}

// UpdateSessionActivity updates the last_used_at timestamp for a session
func (s *Store) UpdateSessionActivity(token string) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET last_used_at = ? WHERE id = ?`,
		time.Now().UTC(), token,
	)
	return err
}
