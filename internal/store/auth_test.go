package store

import (
	"errors"
	"testing"
	"time"

	"streammon/internal/models"
)

func createTestUser(t *testing.T, s *Store, name, email string) *models.User {
	t.Helper()
	u, err := s.CreateLocalUser(name, email, "hash", models.RoleViewer)
	if err != nil {
		t.Fatalf("CreateLocalUser(%q): %v", name, err)
	}
	return u
}

func TestUpdateUserEmail(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	user := createTestUser(t, s, "alice", "alice@example.com")

	t.Run("success", func(t *testing.T) {
		if err := s.UpdateUserEmail(user.ID, "new@example.com"); err != nil {
			t.Fatalf("UpdateUserEmail: %v", err)
		}
		u, err := s.GetUserByID(user.ID)
		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}
		if u.Email != "new@example.com" {
			t.Errorf("got email %q, want %q", u.Email, "new@example.com")
		}
	})

	t.Run("clear email", func(t *testing.T) {
		if err := s.UpdateUserEmail(user.ID, ""); err != nil {
			t.Fatalf("UpdateUserEmail: %v", err)
		}
		u, err := s.GetUserByID(user.ID)
		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}
		if u.Email != "" {
			t.Errorf("got email %q, want empty", u.Email)
		}
	})

	t.Run("uniqueness conflict", func(t *testing.T) {
		createTestUser(t, s, "bob", "bob@example.com")
		// alice tries to take bob's email
		err := s.UpdateUserEmail(user.ID, "bob@example.com")
		if !errors.Is(err, ErrEmailInUse) {
			t.Fatalf("got err %v, want ErrEmailInUse", err)
		}
	})

	t.Run("multiple users can have empty email", func(t *testing.T) {
		user2 := createTestUser(t, s, "charlie", "")
		if err := s.UpdateUserEmail(user.ID, ""); err != nil {
			t.Fatalf("UpdateUserEmail alice to empty: %v", err)
		}
		if err := s.UpdateUserEmail(user2.ID, ""); err != nil {
			t.Fatalf("UpdateUserEmail charlie to empty: %v", err)
		}
	})

	t.Run("nonexistent user", func(t *testing.T) {
		err := s.UpdateUserEmail(99999, "x@example.com")
		if !errors.Is(err, models.ErrNotFound) {
			t.Fatalf("got err %v, want ErrNotFound", err)
		}
	})
}

func TestGetPasswordHashByUserID(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	t.Run("user with password", func(t *testing.T) {
		u := createTestUser(t, s, "alice", "alice@example.com")
		hash, err := s.GetPasswordHashByUserID(u.ID)
		if err != nil {
			t.Fatalf("GetPasswordHashByUserID: %v", err)
		}
		if hash != "hash" {
			t.Errorf("got %q, want %q", hash, "hash")
		}
	})

	t.Run("user without password", func(t *testing.T) {
		u := createTestUser(t, s, "bob", "bob@example.com")
		// Clear the password hash
		_, err := s.db.Exec(`UPDATE users SET password_hash = '' WHERE id = ?`, u.ID)
		if err != nil {
			t.Fatalf("clearing password: %v", err)
		}
		hash, err := s.GetPasswordHashByUserID(u.ID)
		if err != nil {
			t.Fatalf("GetPasswordHashByUserID: %v", err)
		}
		if hash != "" {
			t.Errorf("got %q, want empty", hash)
		}
	})

	t.Run("nonexistent user", func(t *testing.T) {
		_, err := s.GetPasswordHashByUserID(99999)
		if !errors.Is(err, models.ErrNotFound) {
			t.Fatalf("got err %v, want ErrNotFound", err)
		}
	})
}

func TestDeleteUserSessionsExcept(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	user := createTestUser(t, s, "alice", "alice@example.com")
	expires := time.Now().UTC().Add(24 * time.Hour)

	token1, err := s.CreateSession(user.ID, expires)
	if err != nil {
		t.Fatalf("CreateSession 1: %v", err)
	}
	token2, err := s.CreateSession(user.ID, expires)
	if err != nil {
		t.Fatalf("CreateSession 2: %v", err)
	}
	token3, err := s.CreateSession(user.ID, expires)
	if err != nil {
		t.Fatalf("CreateSession 3: %v", err)
	}

	if err := s.DeleteUserSessionsExcept(user.ID, token1); err != nil {
		t.Fatalf("DeleteUserSessionsExcept: %v", err)
	}

	// token1 should still work
	if _, err := s.GetSessionUser(token1); err != nil {
		t.Errorf("token1 should still be valid: %v", err)
	}
	// token2 and token3 should be deleted
	if _, err := s.GetSessionUser(token2); err == nil {
		t.Error("token2 should have been deleted")
	}
	if _, err := s.GetSessionUser(token3); err == nil {
		t.Error("token3 should have been deleted")
	}
}
