package store

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTMDBCacheRoundTrip(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	data := json.RawMessage(`{"id":12345,"title":"Test Movie"}`)
	if err := s.SetCachedTMDB("movie:12345", data); err != nil {
		t.Fatalf("SetCachedTMDB: %v", err)
	}

	got, err := s.GetCachedTMDB("movie:12345")
	if err != nil {
		t.Fatalf("GetCachedTMDB: %v", err)
	}
	if got == nil {
		t.Fatal("expected cached data, got nil")
	}
	if string(got) != string(data) {
		t.Fatalf("got %s, want %s", got, data)
	}
}

func TestTMDBCacheMiss(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	got, err := s.GetCachedTMDB("movie:99999")
	if err != nil {
		t.Fatalf("GetCachedTMDB: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil on cache miss, got %s", got)
	}
}

func TestTMDBCacheUpsert(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	v1 := json.RawMessage(`{"version":1}`)
	v2 := json.RawMessage(`{"version":2}`)

	if err := s.SetCachedTMDB("key", v1); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCachedTMDB("key", v2); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetCachedTMDB("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(v2) {
		t.Fatalf("got %s, want %s", got, v2)
	}
}

func TestTMDBCacheExpiry(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	data := json.RawMessage(`{"id":1}`)
	if err := s.SetCachedTMDB("key", data); err != nil {
		t.Fatal(err)
	}

	if err := s.BackdateTMDBCache("key", time.Now().UTC().Add(-25*time.Hour)); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetCachedTMDB("key")
	if err != nil {
		t.Fatalf("GetCachedTMDB: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil for expired entry")
	}
}

func TestPruneTMDBCacheDeletesExpired(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetCachedTMDB("expired", json.RawMessage(`{"id":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.BackdateTMDBCache("expired", time.Now().UTC().Add(-25*time.Hour)); err != nil {
		t.Fatal(err)
	}

	n, err := s.PruneTMDBCache()
	if err != nil {
		t.Fatalf("PruneTMDBCache: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row deleted, got %d", n)
	}

	got, err := s.GetCachedTMDB("expired")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected expired row to be gone")
	}
}

func TestPruneTMDBCacheKeepsFresh(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	data := json.RawMessage(`{"id":2}`)
	if err := s.SetCachedTMDB("fresh", data); err != nil {
		t.Fatal(err)
	}
	if err := s.BackdateTMDBCache("fresh", time.Now().UTC().Add(-23*time.Hour)); err != nil {
		t.Fatal(err)
	}

	n, err := s.PruneTMDBCache()
	if err != nil {
		t.Fatalf("PruneTMDBCache: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 rows deleted, got %d", n)
	}

	got, err := s.GetCachedTMDB("fresh")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("expected fresh row still readable, got %s", got)
	}
}

// TestPruneTMDBCacheBoundaryMatchesGetCachedTMDB pins the delete threshold to
// the same cutoff GetCachedTMDB reads against. A row backdated to exactly
// tmdbCacheTTL ago is already unreadable via GetCachedTMDB (cached_at > cutoff
// is false), so PruneTMDBCache must agree it's eligible for deletion. If the
// two comparisons ever drift apart (e.g. one uses < and the other <=), a row
// can end up permanently unreadable yet never pruned.
func TestPruneTMDBCacheBoundaryMatchesGetCachedTMDB(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetCachedTMDB("boundary", json.RawMessage(`{"id":3}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.BackdateTMDBCache("boundary", time.Now().UTC().Add(-tmdbCacheTTL)); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetCachedTMDB("boundary")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected boundary row to already be unreadable via GetCachedTMDB")
	}

	n, err := s.PruneTMDBCache()
	if err != nil {
		t.Fatalf("PruneTMDBCache: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected boundary row to be pruned, got %d rows deleted", n)
	}
}

func TestPruneTMDBCacheEmptyTable(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	n, err := s.PruneTMDBCache()
	if err != nil {
		t.Fatalf("PruneTMDBCache: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 rows deleted on empty table, got %d", n)
	}
}
