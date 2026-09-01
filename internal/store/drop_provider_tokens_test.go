package store

import "testing"

// Migration 056 retires Plex token storage. The table held long-lived Plex
// account tokens, so an upgrade must actually remove them rather than leave
// them readable in the database.
func TestMigration_DropsProviderTokens(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	var name string
	err := s.db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'provider_tokens'`,
	).Scan(&name)
	if err == nil {
		t.Fatal("provider_tokens table still exists after migrations")
	}

	var n int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM settings WHERE key = 'guest.store_plex_tokens'`,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("retired guest.store_plex_tokens setting still present (%d rows)", n)
	}
}
