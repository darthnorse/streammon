package store

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func migrationsDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "migrations")
}

func newTestStoreWithMigrations(t *testing.T, opts ...Option) *Store {
	t.Helper()
	s, err := New(":memory:", opts...)
	if err != nil {
		t.Fatalf("New(:memory:) failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	dir := migrationsDir()
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("migrations dir not found: %v", err)
	}
	if err := s.Migrate(dir); err != nil {
		t.Fatalf("Migrate() failed: %v", err)
	}
	return s
}
