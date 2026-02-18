package store

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

func TestMigrateMultiStatement(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	dir := t.TempDir()
	sql := `CREATE TABLE test_a (id INTEGER PRIMARY KEY);
ALTER TABLE test_a ADD COLUMN name TEXT DEFAULT '';
CREATE INDEX idx_test_a_name ON test_a(name);`
	if err := os.WriteFile(filepath.Join(dir, "001_multi.sql"), []byte(sql), 0644); err != nil {
		t.Fatal(err)
	}

	if err := s.Migrate(dir); err != nil {
		t.Fatalf("Migrate() multi-statement failed: %v", err)
	}

	// Verify all statements executed: column and index should exist
	var colCount int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('test_a') WHERE name = 'name'").Scan(&colCount); err != nil {
		t.Fatal(err)
	}
	if colCount != 1 {
		t.Fatal("expected 'name' column to exist after multi-statement migration")
	}

	var idxCount int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name = 'idx_test_a_name'").Scan(&idxCount); err != nil {
		t.Fatal(err)
	}
	if idxCount != 1 {
		t.Fatal("expected index idx_test_a_name to exist after multi-statement migration")
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

func TestSplitStatements(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"two statements", "SELECT 1; SELECT 2;", []string{"SELECT 1", "SELECT 2"}},
		{"trailing whitespace", "  SELECT 1  ;  ", []string{"SELECT 1"}},
		{"empty string", "", nil},
		{"only semicolons", ";;;", nil},
		{"no trailing semicolon", "CREATE TABLE foo (id INT)", []string{"CREATE TABLE foo (id INT)"}},
		{"multiline", "ALTER TABLE foo ADD COLUMN bar TEXT;\nCREATE INDEX idx ON foo(bar);",
			[]string{"ALTER TABLE foo ADD COLUMN bar TEXT", "CREATE INDEX idx ON foo(bar)"}},
		{"comments between", "-- comment\nSELECT 1;\n-- another\nSELECT 2;",
			[]string{"-- comment\nSELECT 1", "-- another\nSELECT 2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitStatements(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitStatements(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsIgnorableAlterError(t *testing.T) {
	tests := []struct {
		name string
		stmt string
		err  error
		want bool
	}{
		{
			"duplicate column on ALTER TABLE ADD COLUMN",
			"ALTER TABLE foo ADD COLUMN bar TEXT",
			fmt.Errorf("duplicate column name: bar"),
			true,
		},
		{
			"case insensitive",
			"alter table foo add column bar text",
			fmt.Errorf("Duplicate Column name: bar"),
			true,
		},
		{
			"non-duplicate error on ALTER TABLE",
			"ALTER TABLE foo ADD COLUMN bar TEXT",
			fmt.Errorf("no such table: foo"),
			false,
		},
		{
			"duplicate column on non-ALTER statement",
			"CREATE TABLE foo (id INT)",
			fmt.Errorf("duplicate column name: id"),
			false,
		},
		{
			"ALTER TABLE DROP COLUMN with duplicate error",
			"ALTER TABLE foo DROP COLUMN bar",
			fmt.Errorf("duplicate column"),
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isIgnorableAlterError(tt.stmt, tt.err)
			if got != tt.want {
				t.Errorf("isIgnorableAlterError(%q, %v) = %v, want %v", tt.stmt, tt.err, got, tt.want)
			}
		})
	}
}
