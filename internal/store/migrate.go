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

// pendingVacuumKey is set by a migration that frees pages holding secrets.
// SQLite leaves freed pages readable and VACUUM cannot run inside
// applyMigration's transaction, so the marker survives until the reclaim below
// actually succeeds — a crash or a busy checkpoint retries on the next start.
const pendingVacuumKey = "db.pending_vacuum"

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

	pending, err := s.pendingVacuum()
	if err != nil {
		return err
	}
	if pending {
		// Leave the marker on failure so the next start retries; a database
		// that cannot be compacted is not a reason to refuse to boot.
		if err := s.reclaimFreedPages(); err != nil {
			log.Printf("WARNING: could not reclaim freed database pages, retrying next start: %v", err)
		}
	}

	return nil
}

// pendingVacuum reports whether a migration asked for a reclaim. The settings
// table is absent when Migrate runs against a partial migration set, which is
// not an error — there is simply nothing scheduled.
func (s *Store) pendingVacuum() (bool, error) {
	var hasSettings int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'settings'`,
	).Scan(&hasSettings); err != nil {
		return false, fmt.Errorf("checking settings table: %w", err)
	}
	if hasSettings == 0 {
		return false, nil
	}

	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM settings WHERE key = ?`, pendingVacuumKey).Scan(&n); err != nil {
		return false, fmt.Errorf("checking pending vacuum: %w", err)
	}
	return n > 0, nil
}

func (s *Store) reclaimFreedPages() error {
	start := time.Now()
	if _, err := s.db.Exec("VACUUM"); err != nil {
		return err
	}

	// wal_checkpoint reports contention in its result row rather than as an
	// error, so a discarded row would hide frames still holding the old pages.
	var busy, logFrames, checkpointed int
	if err := s.db.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed); err != nil {
		return fmt.Errorf("wal checkpoint: %w", err)
	}
	if busy != 0 || logFrames != 0 {
		return fmt.Errorf("wal checkpoint incomplete: busy=%d frames=%d", busy, logFrames)
	}

	if _, err := s.db.Exec(`DELETE FROM settings WHERE key = ?`, pendingVacuumKey); err != nil {
		return fmt.Errorf("clearing pending vacuum marker: %w", err)
	}
	log.Printf("reclaimed freed database pages (took %s)", time.Since(start).Round(time.Millisecond))
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

	log.Printf("applying migration %s", f)
	start := time.Now()

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

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("applied migration %s (took %s)", f, time.Since(start).Round(time.Millisecond))
	return nil
}
