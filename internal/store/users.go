package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"streammon/internal/models"
)

const userColumns = `id, name, email, role, thumb_url, created_at, updated_at`
const userColumnsWithProvider = `id, name, email, role, thumb_url, created_at, updated_at, provider, provider_id`

func scanUser(scanner interface{ Scan(...any) error }) (models.User, error) {
	var u models.User
	err := scanner.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.ThumbURL, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

// AdminUser extends User with provider information
type AdminUser struct {
	models.User
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"`
}

func scanAdminUser(scanner interface{ Scan(...any) error }) (AdminUser, error) {
	var u AdminUser
	err := scanner.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.ThumbURL, &u.CreatedAt, &u.UpdatedAt, &u.Provider, &u.ProviderID)
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
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("user %q: %w", name, models.ErrNotFound)
	}
	return nil
}

type UserSummary struct {
	Name                       string  `json:"name"`
	ThumbURL                   string  `json:"thumb_url"`
	LastStreamedAt             *string `json:"last_streamed_at"`
	LastIP                     string  `json:"last_ip"`
	TotalPlays                 int     `json:"total_plays"`
	TotalWatchedMs             int64   `json:"total_watched_ms"`
	TrustScore                 int     `json:"trust_score"`
	LastPlayedTitle            string  `json:"last_played_title"`
	LastPlayedGrandparentTitle string  `json:"last_played_grandparent_title"`
	LastPlayedMediaType        string  `json:"last_played_media_type"`
	LastPlayedServerID         int     `json:"last_played_server_id"`
	LastPlayedItemID           string  `json:"last_played_item_id"`
	LastPlayedGrandparentID    string  `json:"last_played_grandparent_item_id"`
}

func (s *Store) ListUserSummaries() ([]UserSummary, error) {
	// Users derived from watch_history (source of truth); users table only has OIDC logins
	rows, err := s.db.Query(`
		WITH ranked AS (
			SELECT
				user_name,
				ip_address,
				title,
				grandparent_title,
				media_type,
				server_id,
				item_id,
				grandparent_item_id,
				ROW_NUMBER() OVER (PARTITION BY user_name ORDER BY started_at DESC) as rn
			FROM watch_history
		),
		stats AS (
			SELECT
				user_name,
				MAX(started_at) as last_streamed_at,
				COUNT(*) as total_plays,
				SUM(watched_ms) as total_watched_ms
			FROM watch_history
			GROUP BY user_name
		),
		last_entry AS (
			SELECT user_name, ip_address as last_ip, title, grandparent_title, media_type, server_id, item_id, grandparent_item_id
			FROM ranked
			WHERE rn = 1
		)
		SELECT
			s.user_name,
			COALESCE(u.thumb_url, '') as thumb_url,
			s.last_streamed_at,
			COALESCE(le.last_ip, '') as last_ip,
			s.total_plays,
			s.total_watched_ms,
			COALESCE(t.score, 100) as trust_score,
			COALESCE(le.title, '') as last_played_title,
			COALESCE(le.grandparent_title, '') as last_played_grandparent_title,
			COALESCE(le.media_type, '') as last_played_media_type,
			COALESCE(le.server_id, 0) as last_played_server_id,
			COALESCE(le.item_id, '') as last_played_item_id,
			COALESCE(le.grandparent_item_id, '') as last_played_grandparent_item_id
		FROM stats s
		LEFT JOIN last_entry le ON s.user_name = le.user_name
		LEFT JOIN users u ON s.user_name = u.name
		LEFT JOIN user_trust_scores t ON s.user_name = t.user_name
		ORDER BY s.user_name`)
	if err != nil {
		return nil, fmt.Errorf("listing user summaries: %w", err)
	}
	defer rows.Close()

	summaries := []UserSummary{}
	for rows.Next() {
		var s UserSummary
		if err := rows.Scan(
			&s.Name, &s.ThumbURL, &s.LastStreamedAt, &s.LastIP,
			&s.TotalPlays, &s.TotalWatchedMs, &s.TrustScore,
			&s.LastPlayedTitle, &s.LastPlayedGrandparentTitle, &s.LastPlayedMediaType,
			&s.LastPlayedServerID, &s.LastPlayedItemID, &s.LastPlayedGrandparentID,
		); err != nil {
			return nil, fmt.Errorf("scanning user summary: %w", err)
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

func (s *Store) GetOrCreateUserByEmail(email, name string) (*models.User, error) {
	_, err := s.db.Exec(
		`INSERT INTO users (name, email) VALUES (?, ?) ON CONFLICT(name) DO UPDATE SET email = excluded.email`,
		name, email,
	)
	if err != nil {
		return nil, fmt.Errorf("upserting user by email: %w", err)
	}

	u, err := scanUser(s.db.QueryRow(
		`SELECT `+userColumns+` FROM users WHERE name = ?`, name,
	))
	if err != nil {
		return nil, fmt.Errorf("querying user: %w", err)
	}
	return &u, nil
}

func (s *Store) UpdateUserAvatar(name, thumbURL string) error {
	_, err := s.db.Exec(
		`INSERT INTO users (name, thumb_url) VALUES (?, ?)
		 ON CONFLICT(name) DO UPDATE SET thumb_url = excluded.thumb_url, updated_at = CURRENT_TIMESTAMP`,
		name, thumbURL,
	)
	if err != nil {
		return fmt.Errorf("upserting user avatar: %w", err)
	}
	return nil
}

type SyncUserAvatarsResult struct {
	Synced  int `json:"synced"`
	Updated int `json:"updated"`
}

func (s *Store) SyncUsersFromServer(serverID int64, users []models.MediaUser) (*SyncUserAvatarsResult, error) {
	result := &SyncUserAvatarsResult{}

	existingUsers, err := s.ListUsers()
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	existingByName := make(map[string]string, len(existingUsers))
	for _, u := range existingUsers {
		existingByName[u.Name] = u.ThumbURL
	}

	for _, u := range users {
		if u.ThumbURL == "" {
			continue
		}

		thumbURL := u.ThumbURL
		if !isFullURL(thumbURL) {
			thumbURL = fmt.Sprintf("/api/servers/%d/thumb/%s", serverID, u.ThumbURL)
		}

		existingThumb, exists := existingByName[u.Name]
		if exists && existingThumb == thumbURL {
			continue
		}

		if err := s.UpdateUserAvatar(u.Name, thumbURL); err != nil {
			return nil, fmt.Errorf("updating avatar for %q: %w", u.Name, err)
		}

		if exists {
			result.Updated++
		} else {
			result.Synced++
		}
	}

	return result, nil
}

func isFullURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// GetUserByID retrieves a user by ID
func (s *Store) GetUserByID(id int64) (*models.User, error) {
	u, err := scanUser(s.db.QueryRow(
		`SELECT `+userColumns+` FROM users WHERE id = ?`, id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("user %d: %w", id, models.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return &u, nil
}

// ListAdminUsers returns all users with provider information (for admin UI)
func (s *Store) ListAdminUsers() ([]AdminUser, error) {
	rows, err := s.db.Query(`SELECT ` + userColumnsWithProvider + ` FROM users ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing admin users: %w", err)
	}
	defer rows.Close()

	users := []AdminUser{}
	for rows.Next() {
		u, err := scanAdminUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetAdminUserByID retrieves a user by ID with provider information (for admin UI)
func (s *Store) GetAdminUserByID(id int64) (*AdminUser, error) {
	u, err := scanAdminUser(s.db.QueryRow(
		`SELECT `+userColumnsWithProvider+` FROM users WHERE id = ?`, id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("user %d: %w", id, models.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("getting admin user by id: %w", err)
	}
	return &u, nil
}

// ErrLastAdmin is returned when trying to delete or demote the last admin
var ErrLastAdmin = errors.New("cannot remove the last admin")

// checkLastAdmin verifies the user exists and checks if removing their admin role
// would leave no admins. Returns the user's current role (even on ErrLastAdmin).
func checkLastAdmin(tx *sql.Tx, id int64) (models.Role, error) {
	var role models.Role
	err := tx.QueryRow(`SELECT role FROM users WHERE id = ?`, id).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("user %d: %w", id, models.ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("checking user role: %w", err)
	}

	if role == models.RoleAdmin {
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ?`, models.RoleAdmin).Scan(&count); err != nil {
			return role, fmt.Errorf("counting admins: %w", err)
		}
		if count <= 1 {
			return role, ErrLastAdmin
		}
	}
	return role, nil
}

// DeleteUser deletes a user by ID, including their sessions.
// Returns ErrLastAdmin if this is the last admin user.
func (s *Store) DeleteUser(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := checkLastAdmin(tx, id); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return fmt.Errorf("deleting user sessions: %w", err)
	}

	result, err := tx.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("user %d: %w", id, models.ErrNotFound)
	}

	return tx.Commit()
}

// CountAdmins returns the number of admin users
func (s *Store) CountAdmins() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ?`, models.RoleAdmin).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting admins: %w", err)
	}
	return count, nil
}

// UpdateUserRoleByIDSafe updates a user's role by ID with last-admin protection.
// Returns ErrLastAdmin if trying to demote the last admin.
func (s *Store) UpdateUserRoleByIDSafe(id int64, newRole models.Role) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = checkLastAdmin(tx, id)
	// Block only if demoting (ErrLastAdmin + new role is not admin)
	if err != nil && !(errors.Is(err, ErrLastAdmin) && newRole == models.RoleAdmin) {
		return err
	}

	result, err := tx.Exec(
		`UPDATE users SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newRole, id,
	)
	if err != nil {
		return fmt.Errorf("updating user role: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("user %d: %w", id, models.ErrNotFound)
	}

	return tx.Commit()
}

// MergeUsersResult contains the result of a user merge operation
type MergeUsersResult struct {
	WatchHistoryMoved int `json:"watch_history_moved"`
}

// scanUserFromTx scans a user from a transaction query row
func scanUserFromTx(tx *sql.Tx, id int64) (models.User, error) {
	var u models.User
	err := tx.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id).Scan(
		&u.ID, &u.Name, &u.Email, &u.Role, &u.ThumbURL, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return u, fmt.Errorf("user %d: %w", id, models.ErrNotFound)
	}
	if err != nil {
		return u, fmt.Errorf("getting user %d: %w", id, err)
	}
	return u, nil
}

// MergeUsers merges one user into another, transferring all watch history.
// The "from" user is deleted after merging.
// Returns ErrLastAdmin if trying to delete the last admin.
func (s *Store) MergeUsers(keepID, deleteID int64) (*MergeUsersResult, error) {
	if keepID == deleteID {
		return nil, fmt.Errorf("cannot merge user with itself")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	keepUser, err := scanUserFromTx(tx, keepID)
	if err != nil {
		return nil, err
	}
	deleteUser, err := scanUserFromTx(tx, deleteID)
	if err != nil {
		return nil, err
	}

	if _, err := checkLastAdmin(tx, deleteID); err != nil {
		return nil, err
	}

	result, err := tx.Exec(
		`UPDATE watch_history SET user_name = ? WHERE user_name = ?`,
		keepUser.Name, deleteUser.Name,
	)
	if err != nil {
		return nil, fmt.Errorf("transferring watch history: %w", err)
	}
	historyMoved, _ := result.RowsAffected()

	// Transfer rule violations - delete conflicting keep-user violations first
	// Only for non-empty session_keys (unique index only applies when session_key != '')
	if _, err := tx.Exec(`DELETE FROM rule_violations WHERE user_name = ? AND session_key != '' AND (rule_id, session_key) IN
		(SELECT rule_id, session_key FROM rule_violations WHERE user_name = ? AND session_key != '')`,
		keepUser.Name, deleteUser.Name); err != nil {
		log.Printf("warning: failed to clear conflicting violations for %s: %v", keepUser.Name, err)
	}
	if _, err := tx.Exec(`UPDATE rule_violations SET user_name = ? WHERE user_name = ?`,
		keepUser.Name, deleteUser.Name); err != nil {
		log.Printf("warning: failed to transfer rule violations during merge: %v", err)
	}

	// Transfer trust scores only if delete-user has one (avoid losing keep-user's data)
	var hasDeleteUserTrust bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM user_trust_scores WHERE user_name = ?)`,
		deleteUser.Name).Scan(&hasDeleteUserTrust); err == nil && hasDeleteUserTrust {
		if _, err := tx.Exec(`DELETE FROM user_trust_scores WHERE user_name = ?`, keepUser.Name); err != nil {
			log.Printf("warning: failed to clear trust scores for %s: %v", keepUser.Name, err)
		}
		if _, err := tx.Exec(`UPDATE user_trust_scores SET user_name = ? WHERE user_name = ?`,
			keepUser.Name, deleteUser.Name); err != nil {
			log.Printf("warning: failed to transfer trust scores during merge: %v", err)
		}
	}

	// Transfer household locations only if delete-user has some (avoid losing keep-user's data)
	var hasDeleteUserLocations bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM household_locations WHERE user_name = ?)`,
		deleteUser.Name).Scan(&hasDeleteUserLocations); err == nil && hasDeleteUserLocations {
		if _, err := tx.Exec(`DELETE FROM household_locations WHERE user_name = ?`, keepUser.Name); err != nil {
			log.Printf("warning: failed to clear household locations for %s: %v", keepUser.Name, err)
		}
		if _, err := tx.Exec(`UPDATE household_locations SET user_name = ? WHERE user_name = ?`,
			keepUser.Name, deleteUser.Name); err != nil {
			log.Printf("warning: failed to transfer household locations during merge: %v", err)
		}
	}

	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, deleteID); err != nil {
		return nil, fmt.Errorf("deleting sessions: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM users WHERE id = ?`, deleteID); err != nil {
		return nil, fmt.Errorf("deleting user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &MergeUsersResult{
		WatchHistoryMoved: int(historyMoved),
	}, nil
}
