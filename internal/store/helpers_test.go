package store

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"streammon/internal/crypto"
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

// testEncryptor builds a throwaway AES key for tests that exercise
// encryption-at-rest.
func testEncryptor(t *testing.T) *crypto.Encryptor {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	enc, err := crypto.NewEncryptor(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	return enc
}
