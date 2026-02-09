package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
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

// GetUserByEmail retrieves a user by email address
func (s *Store) GetUserByEmail(email string) (*models.User, error) {
	if email == "" {
		return nil, fmt.Errorf("user: %w", models.ErrNotFound)
	}
	u, err := scanUser(s.db.QueryRow(
		`SELECT `+userColumns+` FROM users WHERE email = ?`, email,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("user: %w", models.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by email: %w", err)
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

// ErrNoPassword is returned when trying to unlink a user with no password set
var ErrNoPassword = errors.New("user has no password set")

// UnlinkUserProvider removes the provider link from a user, allowing re-linking.
// Returns ErrNoPassword if the user has no password (would be locked out).
func (s *Store) UnlinkUserProvider(userID int64) error {
	var hasPassword bool
	err := s.db.QueryRow(
		`SELECT password_hash != '' AND password_hash IS NOT NULL FROM users WHERE id = ?`, userID,
	).Scan(&hasPassword)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("user %d: %w", userID, models.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("checking user password: %w", err)
	}
	if !hasPassword {
		return ErrNoPassword
	}

	result, err := s.db.Exec(
		`UPDATE users SET provider = '', provider_id = '', updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("unlinking provider: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user %d: %w", userID, models.ErrNotFound)
	}
	return nil
}

// GetUnlinkedUserByName finds a user by name (case-insensitive) that has no provider linked.
// This is used for auto-linking streaming users to their login accounts.
func (s *Store) GetUnlinkedUserByName(name string) (*models.User, error) {
	if name == "" {
		return nil, fmt.Errorf("user: %w", models.ErrNotFound)
	}
	u, err := scanUser(s.db.QueryRow(
		`SELECT `+userColumns+` FROM users WHERE LOWER(name) = LOWER(?) AND (provider_id = '' OR provider_id IS NULL)`,
		name,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("user: %w", models.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("getting unlinked user by name: %w", err)
	}
	return &u, nil
}

func (s *Store) maybeUpdateAvatar(user *models.User, thumbURL string) {
	if thumbURL != "" && thumbURL != user.ThumbURL {
		if err := s.UpdateUserAvatar(user.Name, thumbURL); err != nil {
			log.Printf("warning: failed to update avatar for %s: %v", user.Name, err)
			return
		}
		user.ThumbURL = thumbURL
	}
}

// GetOrLinkUserByEmail finds user by provider ID first, then by email, then by username for account linking.
// SECURITY: Account linking by email is only performed for OAuth providers (plex, oidc)
// where the email is verified by the provider. Local accounts are not linked by email.
// Username linking only matches unlinked users (no provider_id set).
func (s *Store) GetOrLinkUserByEmail(email, name, provider, providerID, thumbURL string) (*models.User, error) {
	return s.GetOrLinkUser(email, []string{name}, name, provider, providerID, thumbURL)
}

// GetOrLinkUser finds user by provider ID, email, or usernames (in order) for account linking.
// namesToTry is a list of usernames to try matching against unlinked users (e.g., [username, display_name]).
// displayName is used when creating a new user.
func (s *Store) GetOrLinkUser(email string, namesToTry []string, displayName, provider, providerID, thumbURL string) (*models.User, error) {
	// Tier 1: Try to find by provider + providerID (exact match)
	if providerID != "" {
		user, err := s.GetUserByProvider(provider, providerID)
		if err == nil {
			s.maybeUpdateAvatar(user, thumbURL)
			return user, nil
		}
	}

	// Tier 2: Try to find by email for account linking
	// SECURITY: Only link for OAuth providers where email is verified
	// Only link if user has no provider or the same provider (don't overwrite different provider)
	if email != "" && provider != "local" {
		var existingUser models.User
		var existingProvider string
		err := s.db.QueryRow(
			`SELECT `+userColumns+`, COALESCE(provider, '') FROM users WHERE email = ? AND email != ''`, email,
		).Scan(&existingUser.ID, &existingUser.Name, &existingUser.Email, &existingUser.Role,
			&existingUser.ThumbURL, &existingUser.CreatedAt, &existingUser.UpdatedAt, &existingProvider)

		if err == nil {
			// Only link if user has no provider or same provider (don't overwrite different provider)
			if existingProvider == "" || existingProvider == provider {
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
			// User exists with different provider - fall through to create new account
			// Clear email since it belongs to the existing user
			log.Printf("info: user %s already linked to %s, creating new account for %s login",
				existingUser.Name, existingProvider, provider)
			email = ""
		}
	}

	// Tier 3: Try to find by username (case-insensitive) for unlinked users
	// This links streaming users (from watch history) to their login accounts
	// SECURITY: Only link if unlinked user has no email (pure streaming user)
	// to prevent account hijacking via display name collision
	if provider != "local" {
		for _, name := range namesToTry {
			if name == "" {
				continue
			}
			existingUser, err := s.GetUnlinkedUserByName(name)
			if err == nil {
				// Only auto-link if the existing user has no email set
				// (they're a pure streaming user with no prior OAuth identity)
				if existingUser.Email != "" {
					log.Printf("info: skipping username link for %q - existing user has email set", name)
					continue
				}

				log.Printf("info: auto-linking provider %s (id=%s) to existing user %s (id=%d) via username match on %q",
					provider, providerID, existingUser.Name, existingUser.ID, name)

				if providerID != "" {
					if err := s.LinkProviderAccount(existingUser.ID, provider, providerID); err != nil {
						return nil, fmt.Errorf("linking provider account: %w", err)
					}
				}
				// Update email if we have one
				if email != "" {
					if _, err := s.db.Exec(`UPDATE users SET email = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
						email, existingUser.ID); err != nil {
						log.Printf("warning: failed to update email for user %s: %v", existingUser.Name, err)
					} else {
						existingUser.Email = email
					}
				}
				s.maybeUpdateAvatar(existingUser, thumbURL)
				return existingUser, nil
			}
		}
	}

	// Tier 4: Create new user
	if displayName == "" && email == "" {
		return nil, fmt.Errorf("cannot create user: no name or email provided")
	}
	if displayName == "" {
		displayName = email
	}

	// Try to create user with desired name, handling conflicts atomically
	// Uses INSERT with retry on UNIQUE constraint to avoid race conditions
	const maxUsernameRetries = 100
	finalName := displayName
	for i := 1; i <= maxUsernameRetries; i++ {
		_, err := s.db.Exec(
			`INSERT INTO users (name, email, role, provider, provider_id, thumb_url)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			finalName, email, models.RoleViewer, provider, providerID, thumbURL,
		)
		if err == nil {
			if finalName != displayName {
				log.Printf("info: username %s already exists, created user as %s", displayName, finalName)
			}
			return s.GetUser(finalName)
		}

		// Only retry on name uniqueness violations, not provider conflicts
		if !strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			!strings.Contains(err.Error(), "users.name") {
			return nil, fmt.Errorf("creating user: %w", err)
		}

		// Try next suffix
		finalName = fmt.Sprintf("%s_%d", displayName, i+1)
	}

	return nil, fmt.Errorf("cannot create user: too many users with name %s", displayName)
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

// ErrEmailInUse is returned when a user tries to set an email already used by another user
var ErrEmailInUse = errors.New("email already in use")

// GetPasswordHashByUserID retrieves just the password hash for a user by ID
func (s *Store) GetPasswordHashByUserID(userID int64) (string, error) {
	var passwordHash sql.NullString
	err := s.db.QueryRow(
		`SELECT password_hash FROM users WHERE id = ?`, userID,
	).Scan(&passwordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("user %d: %w", userID, models.ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("getting password hash: %w", err)
	}
	return passwordHash.String, nil
}

// UpdateUserEmail updates a user's email address.
// Returns ErrEmailInUse if another user already has this email.
func (s *Store) UpdateUserEmail(userID int64, email string) error {
	result, err := s.db.Exec(
		`UPDATE users SET email = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		email, userID,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrEmailInUse
		}
		return fmt.Errorf("updating email: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user %d: %w", userID, models.ErrNotFound)
	}
	return nil
}

func (s *Store) GetGuestAccess() (bool, error) {
	val, err := s.GetSetting("auth.guest_access")
	if err != nil {
		log.Printf("reading guest access setting: %v", err)
		return false, nil
	}
	if val == "" {
		val, err = s.GetSetting("auth.plex.guest_access")
		if err != nil {
			log.Printf("reading legacy guest access setting: %v", err)
			return false, nil
		}
		if val != "" {
			_ = s.SetSetting("auth.guest_access", val)
			_ = s.SetSetting("auth.plex.guest_access", "")
		}
	}
	return val == "true", nil
}

func (s *Store) SetGuestAccess(allowed bool) error {
	val := "false"
	if allowed {
		val = "true"
	}
	return s.SetSetting("auth.guest_access", val)
}

// UpdateSessionActivity updates the last_used_at timestamp for a session
func (s *Store) UpdateSessionActivity(token string) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET last_used_at = ? WHERE id = ?`,
		time.Now().UTC(), token,
	)
	return err
}
