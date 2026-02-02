package store

import (
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("New(:memory:) failed: %v", err)
	}
	return s
}

func TestNew(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
}

func TestPing(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	if err := s.Ping(); err != nil {
		t.Fatalf("Ping() failed: %v", err)
	}
}

func TestPingAfterClose(t *testing.T) {
	s := newTestStore(t)
	s.Close()

	if err := s.Ping(); err == nil {
		t.Fatal("expected Ping() to fail after Close()")
	}
}
