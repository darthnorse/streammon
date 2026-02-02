package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrate(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	dir := t.TempDir()
	sql := `CREATE TABLE IF NOT EXISTS test_items (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL
	);`
	if err := os.WriteFile(filepath.Join(dir, "001_test.sql"), []byte(sql), 0644); err != nil {
		t.Fatal(err)
	}

	if err := s.Migrate(dir); err != nil {
		t.Fatalf("Migrate() failed: %v", err)
	}

	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM test_items").Scan(&count); err != nil {
		t.Fatalf("querying test_items: %v", err)
	}

	if err := s.Migrate(dir); err != nil {
		t.Fatalf("second Migrate() failed: %v", err)
	}
}

func TestMigrateInvalidFilename(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad_name.sql"), []byte("SELECT 1"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := s.Migrate(dir); err == nil {
		t.Fatal("expected error for invalid migration filename")
	}
}
