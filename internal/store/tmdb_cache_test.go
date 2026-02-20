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
