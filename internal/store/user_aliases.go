package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"streammon/internal/models"
)

var (
	ErrInvalidAlias = errors.New("invalid alias")
	ErrAliasTaken   = errors.New("alias belongs to another user")
)

const aliasOwnerSQL = `SELECT u.name FROM user_aliases a JOIN users u ON u.id = a.user_id WHERE a.alias = ?`

// UserAliasMap maps merged-away usernames to the user they were merged into.
type UserAliasMap map[string]string

// Resolve returns the user name is merged into, or name itself.
func (m UserAliasMap) Resolve(name string) string {
	if canonical, ok := m[name]; ok {
		return canonical
	}
	return name
}

func (s *Store) UserAliases() (UserAliasMap, error) {
	rows, err := s.db.Query(`SELECT a.alias, u.name FROM user_aliases a JOIN users u ON u.id = a.user_id`)
	if err != nil {
		return nil, fmt.Errorf("listing user aliases: %w", err)
	}
	defer rows.Close()

	aliases := make(UserAliasMap)
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

// LinkUserAlias merges alias into the named user. Names that only appear in
// watch history get a placeholder user row so the merge path is shared.
// Relinking an alias the user already owns is a no-op; one owned by another
// user returns ErrAliasTaken rather than silently moving it.
func (s *Store) LinkUserAlias(ctx context.Context, name, alias string) (*MergeUsersResult, error) {
	alias = strings.TrimSpace(alias)
	if alias == "" || alias == name {
		return nil, ErrInvalidAlias
	}

	var owner string
	err := s.db.QueryRowContext(ctx, aliasOwnerSQL, alias).Scan(&owner)
	switch {
	case err == nil && owner == name:
		return &MergeUsersResult{}, nil
	case err == nil:
		return nil, ErrAliasTaken
	case !errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("checking alias owner: %w", err)
	}

	keep, err := s.GetUser(name)
	if errors.Is(err, models.ErrNotFound) {
		hasHistory, hErr := s.UserHasHistory(ctx, name)
		if hErr != nil {
			return nil, hErr
		}
		if !hasHistory {
			return nil, err
		}
		keep, err = s.GetOrCreateUser(name)
	}
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

// recordMergeAlias moves the deleted user's aliases to the kept user and adds
// the deleted name. A kept user is never also an alias, or its own plays would
// keep redirecting to whoever that alias pointed at.
func recordMergeAlias(tx *sql.Tx, keepID, deleteID int64, keepName, deleteName string) error {
	if _, err := tx.Exec(`UPDATE user_aliases SET user_id = ? WHERE user_id = ?`, keepID, deleteID); err != nil {
		return fmt.Errorf("moving aliases: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM user_aliases WHERE alias = ?`, keepName); err != nil {
		return fmt.Errorf("clearing alias for kept user: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO user_aliases (alias, user_id) VALUES (?, ?)
		 ON CONFLICT(alias) DO UPDATE SET user_id = excluded.user_id`, deleteName, keepID); err != nil {
		return fmt.Errorf("recording alias: %w", err)
	}
	return nil
}

// resolveUserNameTx maps a merged-away username inside the write transaction,
// so a merge committing mid-import cannot leave rows under the deleted name.
func resolveUserNameTx(ctx context.Context, tx *sql.Tx, name string) (string, error) {
	var canonical string
	err := tx.QueryRowContext(ctx, aliasOwnerSQL, name).Scan(&canonical)
	if errors.Is(err, sql.ErrNoRows) {
		return name, nil
	}
	if err != nil {
		return "", fmt.Errorf("resolving user alias: %w", err)
	}
	return canonical, nil
}
