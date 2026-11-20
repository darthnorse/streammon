package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"streammon/internal/models"
)

var ErrInvalidAlias = errors.New("invalid alias")

// UserAliases returns every alias mapped to the name of the user it belongs to.
func (s *Store) UserAliases() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT a.alias, u.name FROM user_aliases a JOIN users u ON u.id = a.user_id`)
	if err != nil {
		return nil, fmt.Errorf("listing user aliases: %w", err)
	}
	defer rows.Close()

	aliases := make(map[string]string)
	for rows.Next() {
		var alias, name string
		if err := rows.Scan(&alias, &name); err != nil {
			return nil, err
		}
		aliases[alias] = name
	}
	return aliases, rows.Err()
}

// ListUserAliases returns the aliases merged into the named user, sorted.
func (s *Store) ListUserAliases(name string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT a.alias FROM user_aliases a JOIN users u ON u.id = a.user_id WHERE u.name = ? ORDER BY a.alias`, name)
	if err != nil {
		return nil, fmt.Errorf("listing aliases for %q: %w", name, err)
	}
	defer rows.Close()

	aliases := []string{}
	for rows.Next() {
		var alias string
		if err := rows.Scan(&alias); err != nil {
			return nil, err
		}
		aliases = append(aliases, alias)
	}
	return aliases, rows.Err()
}

// LinkUserAlias merges alias into the named user. A name that only appears in
// watch history gets a placeholder user row first so the merge path is shared.
func (s *Store) LinkUserAlias(name, alias string) (*MergeUsersResult, error) {
	alias = strings.TrimSpace(alias)
	if alias == "" || alias == name {
		return nil, ErrInvalidAlias
	}
	keep, err := s.GetUser(name)
	if err != nil {
		return nil, err
	}
	other, err := s.GetOrCreateUser(alias)
	if err != nil {
		return nil, err
	}
	return s.MergeUsers(keep.ID, other.ID)
}

// UnlinkUserAlias stops mapping alias to the named user. History already merged stays.
func (s *Store) UnlinkUserAlias(name, alias string) error {
	result, err := s.db.Exec(
		`DELETE FROM user_aliases WHERE alias = ? AND user_id = (SELECT id FROM users WHERE name = ?)`, alias, name)
	if err != nil {
		return fmt.Errorf("unlinking alias: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("alias %q: %w", alias, models.ErrNotFound)
	}
	return nil
}

func recordMergeAlias(tx *sql.Tx, keepID, deleteID int64, deleteName string) error {
	if _, err := tx.Exec(`UPDATE user_aliases SET user_id = ? WHERE user_id = ?`, keepID, deleteID); err != nil {
		return fmt.Errorf("moving aliases: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO user_aliases (alias, user_id) VALUES (?, ?)
		 ON CONFLICT(alias) DO UPDATE SET user_id = excluded.user_id`, deleteName, keepID); err != nil {
		return fmt.Errorf("recording alias: %w", err)
	}
	return nil
}

func resolveAlias(aliases map[string]string, name string) string {
	if canonical, ok := aliases[name]; ok {
		return canonical
	}
	return name
}
