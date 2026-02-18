package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// isIgnorableAlterError handles repair migrations that re-add columns already present.
func isIgnorableAlterError(stmt string, err error) bool {
	upper := strings.ToUpper(strings.TrimSpace(stmt))
	if !strings.HasPrefix(upper, "ALTER TABLE") || !strings.Contains(upper, "ADD COLUMN") {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column")
}

// splitStatements splits SQL content into individual statements on semicolons.
// NOTE: This does not handle semicolons inside string literals (e.g. VALUES ('a;b')).
// For StreamMon's DDL-only migrations this is sufficient.
func splitStatements(content string) []string {
	raw := strings.Split(content, ";")
	var stmts []string
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s != "" {
			stmts = append(stmts, s)
		}
	}
	return stmts
}

func (s *Store) Migrate(migrationsDir string) error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		if err := s.applyMigration(migrationsDir, f); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) applyMigration(dir, f string) error {
	parts := strings.SplitN(f, "_", 2)
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid migration filename %q: expected numeric prefix", f)
	}

	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	content, err := os.ReadFile(filepath.Join(dir, f))
	if err != nil {
		return fmt.Errorf("reading migration %s: %w", f, err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, stmt := range splitStatements(string(content)) {
		if _, err := tx.Exec(stmt); err != nil {
			if isIgnorableAlterError(stmt, err) {
				continue
			}
			return fmt.Errorf("executing migration %s: %w", f, err)
		}
	}

	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
		return err
	}

	return tx.Commit()
}
