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

func TestGetOrCreateUserByEmail(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	u1, err := s.GetOrCreateUserByEmail("alice@example.com", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if u1.Email != "alice@example.com" {
		t.Errorf("got email %q, want alice@example.com", u1.Email)
	}
	if u1.Name != "Alice" {
		t.Errorf("got name %q, want Alice", u1.Name)
	}

	u2, err := s.GetOrCreateUserByEmail("alice@example.com", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if u2.ID != u1.ID {
		t.Errorf("expected same user ID, got %d and %d", u1.ID, u2.ID)
	}
}

func TestUpdateUserAvatar_ExistingUser(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.GetOrCreateUser("alice")

	err := s.UpdateUserAvatar("alice", "https://plex.tv/users/abc/avatar")
	if err != nil {
		t.Fatalf("UpdateUserAvatar: %v", err)
	}

	user, _ := s.GetUser("alice")
	if user.ThumbURL != "https://plex.tv/users/abc/avatar" {
		t.Errorf("got thumb %q, want https://plex.tv/users/abc/avatar", user.ThumbURL)
	}
}

func TestUpdateUserAvatar_NewUser(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	err := s.UpdateUserAvatar("newuser", "https://plex.tv/users/new/avatar")
	if err != nil {
		t.Fatalf("UpdateUserAvatar: %v", err)
	}

	user, err := s.GetUser("newuser")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.ThumbURL != "https://plex.tv/users/new/avatar" {
		t.Errorf("got thumb %q, want https://plex.tv/users/new/avatar", user.ThumbURL)
	}
}

func TestUpdateUserAvatar_Overwrite(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.UpdateUserAvatar("alice", "https://old-url")
	s.UpdateUserAvatar("alice", "https://new-url")

	user, _ := s.GetUser("alice")
	if user.ThumbURL != "https://new-url" {
		t.Errorf("got thumb %q, want https://new-url", user.ThumbURL)
	}
}

func TestSyncUsersFromServer_PlexFullURLs(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	users := []models.MediaUser{
		{Name: "alice", ThumbURL: "https://plex.tv/users/abc/avatar"},
		{Name: "bob", ThumbURL: "https://plex.tv/users/def/avatar"},
	}

	result, err := s.SyncUsersFromServer(1, users)
	if err != nil {
		t.Fatalf("SyncUsersFromServer: %v", err)
	}
	if result.Synced != 2 {
		t.Errorf("got synced %d, want 2", result.Synced)
	}

	alice, _ := s.GetUser("alice")
	if alice.ThumbURL != "https://plex.tv/users/abc/avatar" {
		t.Errorf("alice thumb = %q, want full URL", alice.ThumbURL)
	}
}

func TestSyncUsersFromServer_EmbyProxyURLs(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	users := []models.MediaUser{
		{Name: "alice", ThumbURL: "user/abc123def456"},
	}

	result, err := s.SyncUsersFromServer(42, users)
	if err != nil {
		t.Fatalf("SyncUsersFromServer: %v", err)
	}
	if result.Synced != 1 {
		t.Errorf("got synced %d, want 1", result.Synced)
	}

	alice, _ := s.GetUser("alice")
	expected := "/api/servers/42/thumb/user/abc123def456"
	if alice.ThumbURL != expected {
		t.Errorf("alice thumb = %q, want %q", alice.ThumbURL, expected)
	}
}

func TestSyncUsersFromServer_SkipsEmptyThumb(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	users := []models.MediaUser{
		{Name: "alice", ThumbURL: ""},
		{Name: "bob", ThumbURL: "https://plex.tv/avatar"},
	}

	result, err := s.SyncUsersFromServer(1, users)
	if err != nil {
		t.Fatalf("SyncUsersFromServer: %v", err)
	}
	if result.Synced != 1 {
		t.Errorf("got synced %d, want 1 (alice should be skipped)", result.Synced)
	}

	_, err = s.GetUser("alice")
	if err == nil {
		t.Error("alice should not have been created")
	}
}

func TestSyncUsersFromServer_SkipsUnchanged(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.UpdateUserAvatar("alice", "https://plex.tv/avatar")

	users := []models.MediaUser{
		{Name: "alice", ThumbURL: "https://plex.tv/avatar"},
	}

	result, err := s.SyncUsersFromServer(1, users)
	if err != nil {
		t.Fatalf("SyncUsersFromServer: %v", err)
	}
	if result.Synced != 0 || result.Updated != 0 {
		t.Errorf("got synced=%d updated=%d, want both 0 (unchanged)", result.Synced, result.Updated)
	}
}

func TestSyncUsersFromServer_UpdatesExisting(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.UpdateUserAvatar("alice", "https://old-avatar")

	users := []models.MediaUser{
		{Name: "alice", ThumbURL: "https://new-avatar"},
	}

	result, err := s.SyncUsersFromServer(1, users)
	if err != nil {
		t.Fatalf("SyncUsersFromServer: %v", err)
	}
	if result.Updated != 1 {
		t.Errorf("got updated %d, want 1", result.Updated)
	}

	alice, _ := s.GetUser("alice")
	if alice.ThumbURL != "https://new-avatar" {
		t.Errorf("got thumb %q, want https://new-avatar", alice.ThumbURL)
	}
}

func TestIsFullURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://plex.tv/avatar", true},
		{"http://example.com/img", true},
		{"https://x", true},
		{"http://x", true},
		{"user/abc123", false},
		{"/api/servers/1/thumb/user/abc", false},
		{"", false},
		{"http://", true},
		{"https://", true},
		{"ftp://example.com", false},
		{"htt", false},
	}

	for _, tt := range tests {
		got := isFullURL(tt.url)
		if got != tt.want {
			t.Errorf("isFullURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}
