package store

import (
	"testing"

	"streammon/internal/models"
)

func TestGetOrCreateUser_New(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	user, err := s.GetOrCreateUser("alice")
	if err != nil {
		t.Fatalf("GetOrCreateUser: %v", err)
	}
	if user.Name != "alice" {
		t.Fatalf("expected alice, got %s", user.Name)
	}
	if user.Role != models.RoleViewer {
		t.Fatalf("expected viewer role, got %s", user.Role)
	}
	if user.ID == 0 {
		t.Fatal("expected ID to be set")
	}
}

func TestGetOrCreateUser_Existing(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	u1, _ := s.GetOrCreateUser("alice")
	u2, _ := s.GetOrCreateUser("alice")
	if u1.ID != u2.ID {
		t.Fatalf("expected same ID, got %d and %d", u1.ID, u2.ID)
	}
}

func TestListUsers(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.GetOrCreateUser("alice")
	s.GetOrCreateUser("bob")

	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestGetUser(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.GetOrCreateUser("alice")

	user, err := s.GetUser("alice")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Name != "alice" {
		t.Fatalf("expected alice, got %s", user.Name)
	}
}

func TestGetUserNotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	_, err := s.GetUser("nobody")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestUpdateUserRole(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.GetOrCreateUser("alice")

	err := s.UpdateUserRole("alice", models.RoleAdmin)
	if err != nil {
		t.Fatalf("UpdateUserRole: %v", err)
	}

	user, _ := s.GetUser("alice")
	if user.Role != models.RoleAdmin {
		t.Fatalf("expected admin, got %s", user.Role)
	}
}
