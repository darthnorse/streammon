package store

import (
	"testing"
	"time"
)

func TestAPIKey_NotConfigured(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	val, err := s.GetAPIKey()
	if err != nil {
		t.Fatalf("GetAPIKey: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty value on fresh store, got %q", val)
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
	if err := s.SetAPIKey("sm_secret", now); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	val, err := s.GetAPIKey()
	if err != nil || val != "sm_secret" {
		t.Fatalf("got value=%q err=%v", val, err)
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
	if err := s.SetAPIKey("sm_first", t1); err != nil {
		t.Fatalf("first SetAPIKey: %v", err)
	}

	t2 := t1.Add(time.Hour)
	if err := s.SetAPIKey("sm_second", t2); err != nil {
		t.Fatalf("second SetAPIKey: %v", err)
	}

	val, _ := s.GetAPIKey()
	if val != "sm_second" {
		t.Errorf("expected sm_second, got %q", val)
	}
	createdAt, _ := s.GetAPIKeyCreatedAt()
	if !createdAt.Equal(t2) {
		t.Errorf("expected createdAt=%v, got %v", t2, createdAt)
	}
}

func TestAPIKey_Clear(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetAPIKey("sm_x", time.Now().UTC()); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	if err := s.ClearAPIKey(); err != nil {
		t.Fatalf("ClearAPIKey: %v", err)
	}

	val, _ := s.GetAPIKey()
	if val != "" {
		t.Errorf("expected empty value after clear, got %q", val)
	}
	createdAt, _ := s.GetAPIKeyCreatedAt()
	if !createdAt.IsZero() {
		t.Errorf("expected zero time after clear, got %v", createdAt)
	}
}

func TestAPIKey_LegacyHashSweep(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Simulate a deployment that ran the previous (hash-only) version.
	if err := s.SetSetting(apiKeyLegacyHashSetting, "old-hash-data"); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}

	if err := s.SetAPIKey("sm_new", time.Now().UTC()); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	leftover, _ := s.GetSetting(apiKeyLegacyHashSetting)
	if leftover != "" {
		t.Errorf("legacy hash setting should be cleared, got %q", leftover)
	}
}
