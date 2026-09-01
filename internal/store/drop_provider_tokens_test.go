package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// A fresh database has no token and no guest.store_plex_tokens row, so only an
// upgrade from the pre-056 state actually exercises migration 056.
func TestMigration056_RemovesTokensOnUpgrade(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "upgrade.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	if err := s.Migrate(migrationsDir()); err != nil {
		t.Fatalf("initial migrate: %v", err)
	}

	const marker = "MARKER-PLEX-TOKEN-CIPHERTEXT"
	seed := []string{
		`CREATE TABLE provider_tokens (
			user_id INTEGER NOT NULL, provider TEXT NOT NULL, token TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
			PRIMARY KEY (user_id, provider))`,
		`INSERT INTO settings (key, value) VALUES ('guest.store_plex_tokens', 'true')`,
		`DELETE FROM schema_migrations WHERE version = 56`,
	}
	for _, stmt := range seed {
		if _, err := s.db.Exec(stmt); err != nil {
			t.Fatalf("seeding pre-056 state: %v", err)
		}
	}
	// Enough rows to span pages.
	for i := 0; i < 200; i++ {
		if _, err := s.db.Exec(
			`INSERT INTO provider_tokens (user_id, provider, token) VALUES (?, 'plex', ?)`,
			i, marker,
		); err != nil {
			t.Fatalf("seeding tokens: %v", err)
		}
	}

	if err := s.Migrate(migrationsDir()); err != nil {
		t.Fatalf("upgrade migrate: %v", err)
	}

	var tables int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'provider_tokens'`,
	).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatal("provider_tokens table still exists after migration 056")
	}

	var settings int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM settings WHERE key = 'guest.store_plex_tokens'`,
	).Scan(&settings); err != nil {
		t.Fatal(err)
	}
	if settings != 0 {
		t.Fatalf("retired guest.store_plex_tokens setting survived the upgrade (%d rows)", settings)
	}

	// DROP TABLE alone leaves the token bytes readable in the freelist.
	if _, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal"} {
		raw, err := os.ReadFile(dbPath + suffix)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if n := bytes.Count(raw, []byte(marker)); n != 0 {
			t.Fatalf("%d token copies still readable in %s after migration", n, filepath.Base(dbPath+suffix))
		}
	}
}
