package store

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
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

// Does not handle semicolons inside string literals — fine for DDL-only migrations.
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

// vacuumAfter lists migrations that free pages holding secrets. SQLite leaves
// freed pages readable, and VACUUM cannot run inside applyMigration's
// transaction, so the reclaim happens here instead.
var vacuumAfter = map[int]bool{
	56: true, // drops provider_tokens (stored Plex account tokens)
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

	needVacuum := false
	for _, f := range files {
		applied, version, err := s.applyMigration(migrationsDir, f)
		if err != nil {
			return err
		}
		if applied && vacuumAfter[version] {
			needVacuum = true
		}
	}

	if needVacuum {
		if err := s.reclaimFreedPages(); err != nil {
			return fmt.Errorf("reclaiming freed pages: %w", err)
		}
	}

	return nil
}

func (s *Store) reclaimFreedPages() error {
	start := time.Now()
	if _, err := s.db.Exec("VACUUM"); err != nil {
		return err
	}
	if _, err := s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return err
	}
	log.Printf("reclaimed freed database pages (took %s)", time.Since(start).Round(time.Millisecond))
	return nil
}

// applyMigration reports whether it applied the file, plus its version.
func (s *Store) applyMigration(dir, f string) (bool, int, error) {
	parts := strings.SplitN(f, "_", 2)
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return false, 0, fmt.Errorf("invalid migration filename %q: expected numeric prefix", f)
	}

	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count); err != nil {
		return false, version, err
	}
	if count > 0 {
		return false, version, nil
	}

	log.Printf("applying migration %s", f)
	start := time.Now()

	content, err := os.ReadFile(filepath.Join(dir, f))
	if err != nil {
		return false, version, fmt.Errorf("reading migration %s: %w", f, err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return false, version, err
	}
	defer tx.Rollback()

	for _, stmt := range splitStatements(string(content)) {
		if _, err := tx.Exec(stmt); err != nil {
			if isIgnorableAlterError(stmt, err) {
				continue
			}
			return false, version, fmt.Errorf("executing migration %s: %w", f, err)
		}
	}

	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
		return false, version, err
	}

	if err := tx.Commit(); err != nil {
		return false, version, err
	}
	log.Printf("applied migration %s (took %s)", f, time.Since(start).Round(time.Millisecond))
	return true, version, nil
}
