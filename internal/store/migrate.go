package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// isIgnorableAlterError returns true for ALTER TABLE ADD COLUMN errors
// caused by the column already existing. This handles repair migrations
// that may run on databases where the column was already added.
func isIgnorableAlterError(stmt string, err error) bool {
	upper := strings.ToUpper(strings.TrimSpace(stmt))
	if !strings.HasPrefix(upper, "ALTER TABLE") || !strings.Contains(upper, "ADD COLUMN") {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column")
}

func splitStatements(content string) []string {
	raw := strings.Split(content, ";")
	stmts := make([]string, 0, len(raw))
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
		parts := strings.SplitN(f, "_", 2)
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid migration filename %q: expected numeric prefix", f)
		}

		var count int
		err = s.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
		if err != nil {
			return err
		}
		if count > 0 {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, f))
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

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}
