package server

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"streammon/internal/store"
)

func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, f, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(f), "..", "..", "migrations")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("migrations dir: %v", err)
	}
	if err := s.Migrate(dir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	srv := NewServer(s)
	return srv, s
}
