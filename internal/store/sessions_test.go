package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"streammon/internal/models"
)

func TestCreateAndGetSession(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	user, err := s.GetOrCreateUser("alice")
	if err != nil {
		t.Fatal(err)
	}

	token, err := s.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("expected non-empty session token")
	}

	got, err := s.GetSessionUser(token)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != user.ID {
		t.Errorf("got user ID %d, want %d", got.ID, user.ID)
	}
	if got.Name != "alice" {
		t.Errorf("got name %q, want %q", got.Name, "alice")
	}
}

func TestGetSessionUser_Expired(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	user, _ := s.GetOrCreateUser("bob")
	token, _ := s.CreateSession(user.ID, time.Now().UTC().Add(-1*time.Hour))

	_, err := s.GetSessionUser(token)
	if !errors.Is(err, models.ErrNotFound) {
		t.Errorf("expected ErrNotFound for expired session, got %v", err)
	}
}

func TestGetSessionUser_NotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	_, err := s.GetSessionUser("nonexistent")
	if !errors.Is(err, models.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteSession(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	user, _ := s.GetOrCreateUser("carol")
	token, _ := s.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))

	if err := s.DeleteSession(token); err != nil {
		t.Fatal(err)
	}

	_, err := s.GetSessionUser(token)
	if !errors.Is(err, models.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	user, _ := s.GetOrCreateUser("dave")
	s.CreateSession(user.ID, time.Now().UTC().Add(-2*time.Hour))
	s.CreateSession(user.ID, time.Now().UTC().Add(-1*time.Hour))
	activeToken, _ := s.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))

	n, err := s.DeleteExpiredSessions()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 expired deleted, got %d", n)
	}

	_, err = s.GetSessionUser(activeToken)
	if err != nil {
		t.Fatal("active session should still work")
	}
}

func TestSessionTokenIsHashedInDB(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	user, _ := s.GetOrCreateUser("eve")
	token, err := s.CreateSession(user.ID, time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	// The raw token should NOT appear in the database
	var count int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, token).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("raw token found in DB — should be hashed")
	}

	// The SHA-256 hash of the token SHOULD appear
	h := sha256.Sum256([]byte(token))
	hashed := hex.EncodeToString(h[:])
	err = s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, hashed).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected hashed token in DB, found %d rows", count)
	}
}
