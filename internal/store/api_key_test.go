package store

import (
	"testing"
	"time"
)

func TestAPIKey_NotConfigured(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	hash, err := s.GetAPIKeyHash()
	if err != nil {
		t.Fatalf("GetAPIKeyHash: %v", err)
	}
	if hash != "" {
		t.Errorf("expected empty hash on fresh store, got %q", hash)
	}

	createdAt, err := s.GetAPIKeyCreatedAt()
	if err != nil {
		t.Fatalf("GetAPIKeyCreatedAt: %v", err)
	}
	if !createdAt.IsZero() {
		t.Errorf("expected zero time on fresh store, got %v", createdAt)
	}
}

func TestAPIKey_SetAndGet(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	now := time.Now().UTC().Truncate(time.Second)
	if err := s.SetAPIKey("hash-abc", now); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	hash, err := s.GetAPIKeyHash()
	if err != nil || hash != "hash-abc" {
		t.Fatalf("got hash=%q err=%v", hash, err)
	}

	createdAt, err := s.GetAPIKeyCreatedAt()
	if err != nil {
		t.Fatalf("GetAPIKeyCreatedAt: %v", err)
	}
	if !createdAt.Equal(now) {
		t.Errorf("got createdAt=%v want %v", createdAt, now)
	}
}

func TestAPIKey_Overwrite(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	t1 := time.Now().UTC().Truncate(time.Second)
	if err := s.SetAPIKey("hash-1", t1); err != nil {
		t.Fatalf("first SetAPIKey: %v", err)
	}

	t2 := t1.Add(time.Hour)
	if err := s.SetAPIKey("hash-2", t2); err != nil {
		t.Fatalf("second SetAPIKey: %v", err)
	}

	hash, _ := s.GetAPIKeyHash()
	if hash != "hash-2" {
		t.Errorf("expected hash-2, got %q", hash)
	}
	createdAt, _ := s.GetAPIKeyCreatedAt()
	if !createdAt.Equal(t2) {
		t.Errorf("expected createdAt=%v, got %v", t2, createdAt)
	}
}

func TestAPIKey_Clear(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetAPIKey("hash-x", time.Now().UTC()); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	if err := s.ClearAPIKey(); err != nil {
		t.Fatalf("ClearAPIKey: %v", err)
	}

	hash, _ := s.GetAPIKeyHash()
	if hash != "" {
		t.Errorf("expected empty hash after clear, got %q", hash)
	}
	createdAt, _ := s.GetAPIKeyCreatedAt()
	if !createdAt.IsZero() {
		t.Errorf("expected zero time after clear, got %v", createdAt)
	}
}
