package store

import (
	"encoding/json"
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

func TestListAdminUsers(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create users with provider info
	if _, err := s.CreateLocalUser("alice", "alice@example.com", "hash123", models.RoleAdmin); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO users (name, email, role, provider, provider_id) VALUES (?, ?, ?, ?, ?)`,
		"bob", "bob@example.com", models.RoleViewer, "plex", "12345"); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	users, err := s.ListAdminUsers()
	if err != nil {
		t.Fatalf("ListAdminUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	// Find bob and check provider info
	var bob *AdminUser
	for i := range users {
		if users[i].Name == "bob" {
			bob = &users[i]
			break
		}
	}
	if bob == nil {
		t.Fatal("bob not found")
	}
	if bob.Provider != "plex" {
		t.Errorf("expected provider plex, got %s", bob.Provider)
	}
	if bob.ProviderID != "12345" {
		t.Errorf("expected provider_id 12345, got %s", bob.ProviderID)
	}
}

func TestGetAdminUserByID(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if _, err := s.db.Exec(`INSERT INTO users (name, email, role, provider, provider_id) VALUES (?, ?, ?, ?, ?)`,
		"alice", "alice@example.com", models.RoleAdmin, "oidc", "sub-123"); err != nil {
		t.Fatalf("create alice: %v", err)
	}

	// Get alice by name first to find her ID
	alice, err := s.GetUser("alice")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}

	user, err := s.GetAdminUserByID(alice.ID)
	if err != nil {
		t.Fatalf("GetAdminUserByID: %v", err)
	}
	if user.Name != "alice" {
		t.Errorf("expected alice, got %s", user.Name)
	}
	if user.Provider != "oidc" {
		t.Errorf("expected provider oidc, got %s", user.Provider)
	}
	if user.ProviderID != "sub-123" {
		t.Errorf("expected provider_id sub-123, got %s", user.ProviderID)
	}
}

func TestGetAdminUserByID_NotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	_, err := s.GetAdminUserByID(999)
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestDeleteUser(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create two admins (so we can delete one)
	s.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleAdmin)
	s.CreateLocalUser("bob", "bob@example.com", "hash", models.RoleAdmin)

	// Get alice's ID
	alice, _ := s.GetUser("alice")

	err := s.DeleteUser(alice.ID)
	if err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	// Verify alice is deleted
	_, err = s.GetUser("alice")
	if err == nil {
		t.Fatal("alice should be deleted")
	}
}

func TestDeleteUser_LastAdmin(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create only one admin
	s.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleAdmin)
	alice, _ := s.GetUser("alice")

	err := s.DeleteUser(alice.ID)
	if err == nil {
		t.Fatal("expected error when deleting last admin")
	}
	if err != ErrLastAdmin {
		t.Errorf("expected ErrLastAdmin, got %v", err)
	}
}

func TestUpdateUserRoleByIDSafe(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	alice, _ := s.GetUser("alice")

	err := s.UpdateUserRoleByIDSafe(alice.ID, models.RoleAdmin)
	if err != nil {
		t.Fatalf("UpdateUserRoleByIDSafe: %v", err)
	}

	updated, _ := s.GetUser("alice")
	if updated.Role != models.RoleAdmin {
		t.Errorf("expected admin, got %s", updated.Role)
	}
}

func TestUpdateUserRoleByIDSafe_LastAdmin(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleAdmin)
	alice, _ := s.GetUser("alice")

	err := s.UpdateUserRoleByIDSafe(alice.ID, models.RoleViewer)
	if err == nil {
		t.Fatal("expected error when demoting last admin")
	}
	if err != ErrLastAdmin {
		t.Errorf("expected ErrLastAdmin, got %v", err)
	}
}

func TestUnlinkUserProvider(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create user with password and provider
	s.CreateLocalUser("alice", "alice@example.com", "hash123", models.RoleViewer)
	alice, _ := s.GetUser("alice")
	s.LinkProviderAccount(alice.ID, "plex", "12345")

	err := s.UnlinkUserProvider(alice.ID)
	if err != nil {
		t.Fatalf("UnlinkUserProvider: %v", err)
	}

	// Verify provider is cleared
	user, _ := s.GetAdminUserByID(alice.ID)
	if user.Provider != "" {
		t.Errorf("expected empty provider, got %s", user.Provider)
	}
	if user.ProviderID != "" {
		t.Errorf("expected empty provider_id, got %s", user.ProviderID)
	}
}

func TestUnlinkUserProvider_NoPassword(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create OAuth-only user (no password)
	if _, err := s.db.Exec(`INSERT INTO users (name, email, role, provider, provider_id) VALUES (?, ?, ?, ?, ?)`,
		"alice", "alice@example.com", models.RoleViewer, "plex", "12345"); err != nil {
		t.Fatalf("create alice: %v", err)
	}

	alice, err := s.GetUser("alice")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}

	err = s.UnlinkUserProvider(alice.ID)
	if err == nil {
		t.Fatal("expected error when unlinking user without password")
	}
	if err != ErrNoPassword {
		t.Errorf("expected ErrNoPassword, got %v", err)
	}
}

func TestMergeUsers(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create a server first (needed for watch history foreign key)
	s.CreateServer(&models.Server{Name: "Test Server", Type: models.ServerTypePlex, URL: "http://test"})

	// Create two users with admins
	s.CreateLocalUser("admin", "admin@example.com", "hash", models.RoleAdmin)
	s.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	s.CreateLocalUser("bob", "bob@example.com", "hash", models.RoleViewer)

	alice, _ := s.GetUser("alice")
	bob, _ := s.GetUser("bob")

	// Add watch history for bob
	_, err := s.db.Exec(`INSERT INTO watch_history (server_id, user_name, title, media_type, started_at, stopped_at)
		VALUES (1, 'bob', 'Test Movie', 'movie', '2024-01-01T00:00:00Z', '2024-01-01T02:00:00Z')`)
	if err != nil {
		t.Fatalf("insert watch history: %v", err)
	}

	result, err := s.MergeUsers(alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("MergeUsers: %v", err)
	}
	if result.WatchHistoryMoved != 1 {
		t.Errorf("expected 1 watch history moved, got %d", result.WatchHistoryMoved)
	}

	// Verify bob is deleted
	_, err = s.GetUser("bob")
	if err == nil {
		t.Fatal("bob should be deleted")
	}

	// Verify alice has bob's watch history
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM watch_history WHERE user_name = 'alice'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 watch history for alice, got %d", count)
	}
}

func TestMergeUsers_SameUser(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	alice, _ := s.GetUser("alice")

	_, err := s.MergeUsers(alice.ID, alice.ID)
	if err == nil {
		t.Fatal("expected error when merging user with itself")
	}
}

func TestMergeUsers_LastAdmin(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create one admin and one viewer
	s.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleAdmin)
	s.CreateLocalUser("bob", "bob@example.com", "hash", models.RoleViewer)

	alice, _ := s.GetUser("alice")
	bob, _ := s.GetUser("bob")

	// Try to merge alice (admin) into bob (viewer) - should fail
	_, err := s.MergeUsers(bob.ID, alice.ID)
	if err == nil {
		t.Fatal("expected error when merging last admin")
	}
	if err != ErrLastAdmin {
		t.Errorf("expected ErrLastAdmin, got %v", err)
	}
}

func TestGetOrLinkUser_NewUser(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	user, err := s.GetOrLinkUser("alice@example.com", []string{"alice"}, "Alice", "plex", "12345", "https://thumb.url")
	if err != nil {
		t.Fatalf("GetOrLinkUser: %v", err)
	}
	if user.Name != "Alice" {
		t.Errorf("expected name Alice, got %s", user.Name)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", user.Email)
	}
}

func TestGetOrLinkUser_ExistingByProvider(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create user with provider
	if _, err := s.db.Exec(`INSERT INTO users (name, email, role, provider, provider_id) VALUES (?, ?, ?, ?, ?)`,
		"Alice", "alice@example.com", models.RoleViewer, "plex", "12345"); err != nil {
		t.Fatalf("create alice: %v", err)
	}

	user, err := s.GetOrLinkUser("newemail@example.com", []string{"newname"}, "NewName", "plex", "12345", "")
	if err != nil {
		t.Fatalf("GetOrLinkUser: %v", err)
	}
	if user.Name != "Alice" {
		t.Errorf("expected Alice (existing user), got %s", user.Name)
	}
}

func TestGetOrLinkUser_LinkByUsername(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create unlinked user (from Tautulli import)
	s.GetOrCreateUser("alice")

	user, err := s.GetOrLinkUser("alice@example.com", []string{"alice"}, "Alice", "plex", "12345", "")
	if err != nil {
		t.Fatalf("GetOrLinkUser: %v", err)
	}

	// Should link to existing user
	adminUser, _ := s.GetAdminUserByID(user.ID)
	if adminUser.Provider != "plex" {
		t.Errorf("expected provider plex, got %s", adminUser.Provider)
	}
	if adminUser.ProviderID != "12345" {
		t.Errorf("expected provider_id 12345, got %s", adminUser.ProviderID)
	}
}

func TestGetOrLinkUser_UsernameConflict(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create linked user with name "alice"
	if _, err := s.db.Exec(`INSERT INTO users (name, email, role, provider, provider_id) VALUES (?, ?, ?, ?, ?)`,
		"alice", "alice@example.com", models.RoleViewer, "plex", "existing"); err != nil {
		t.Fatalf("create alice: %v", err)
	}

	// Try to create new user also named "alice"
	user, err := s.GetOrLinkUser("bob@example.com", []string{"alice"}, "alice", "oidc", "sub-123", "")
	if err != nil {
		t.Fatalf("GetOrLinkUser: %v", err)
	}
	if user.Name != "alice_2" {
		t.Errorf("expected alice_2 (conflict resolution), got %s", user.Name)
	}
}

func TestGetOrLinkUser_NoOverwriteProvider(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create user linked to plex
	if _, err := s.db.Exec(`INSERT INTO users (name, email, role, provider, provider_id) VALUES (?, ?, ?, ?, ?)`,
		"alice", "alice@example.com", models.RoleViewer, "plex", "12345"); err != nil {
		t.Fatalf("create alice: %v", err)
	}

	// Try to login via OIDC with same email - should create new user
	user, err := s.GetOrLinkUser("alice@example.com", []string{"Alice"}, "Alice", "oidc", "sub-123", "")
	if err != nil {
		t.Fatalf("GetOrLinkUser: %v", err)
	}
	// Should create new user since existing user has different provider
	if user.Name == "alice" {
		t.Errorf("should not overwrite existing user's provider")
	}
}

func TestGetUnlinkedUserByName(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create unlinked user
	s.GetOrCreateUser("alice")

	user, err := s.GetUnlinkedUserByName("alice")
	if err != nil {
		t.Fatalf("GetUnlinkedUserByName: %v", err)
	}
	if user.Name != "alice" {
		t.Errorf("expected alice, got %s", user.Name)
	}
}

func TestGetUnlinkedUserByName_CaseInsensitive(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.GetOrCreateUser("Alice")

	user, err := s.GetUnlinkedUserByName("alice")
	if err != nil {
		t.Fatalf("GetUnlinkedUserByName: %v", err)
	}
	if user.Name != "Alice" {
		t.Errorf("expected Alice, got %s", user.Name)
	}
}

func TestGetUnlinkedUserByName_LinkedUserNotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create linked user
	if _, err := s.db.Exec(`INSERT INTO users (name, role, provider, provider_id) VALUES (?, ?, ?, ?)`,
		"alice", models.RoleViewer, "plex", "12345"); err != nil {
		t.Fatalf("create alice: %v", err)
	}

	_, err := s.GetUnlinkedUserByName("alice")
	if err == nil {
		t.Fatal("expected error for linked user")
	}
}

// Test that username linking is skipped if unlinked user has email set (security fix)
func TestGetOrLinkUser_SkipsUsernameWithEmail(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create unlinked user WITH email (not a pure streaming user)
	if _, err := s.db.Exec(`INSERT INTO users (name, email, role) VALUES (?, ?, ?)`,
		"alice", "alice@existing.com", models.RoleViewer); err != nil {
		t.Fatalf("create alice: %v", err)
	}

	// Try to link via username with different email - should create new user, not link
	user, err := s.GetOrLinkUser("attacker@evil.com", []string{"alice"}, "alice", "oidc", "attacker-sub", "")
	if err != nil {
		t.Fatalf("GetOrLinkUser: %v", err)
	}

	// Should create new user (alice_2), not link to existing alice
	if user.Name == "alice" {
		t.Error("should not link to user with existing email - potential hijack")
	}
	if user.Email != "attacker@evil.com" {
		t.Errorf("new user should have attacker email, got %s", user.Email)
	}
}

// Test that username linking works when unlinked user has no email (pure streaming user)
func TestGetOrLinkUser_LinksUsernameWithoutEmail(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create unlinked user WITHOUT email (pure streaming user from watch history)
	s.GetOrCreateUser("alice")

	// Should link since alice has no email
	user, err := s.GetOrLinkUser("alice@example.com", []string{"alice"}, "Alice", "plex", "12345", "")
	if err != nil {
		t.Fatalf("GetOrLinkUser: %v", err)
	}

	// Should link to existing user
	if user.Name != "alice" {
		t.Errorf("should link to existing alice, got %s", user.Name)
	}

	// Verify provider was linked
	adminUser, _ := s.GetAdminUserByID(user.ID)
	if adminUser.Provider != "plex" {
		t.Errorf("expected provider plex, got %s", adminUser.Provider)
	}
}

// Test that MergeUsers transfers rule violations
func TestMergeUsers_TransfersViolations(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create admin and two users
	s.CreateLocalUser("admin", "admin@example.com", "hash", models.RoleAdmin)
	s.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	s.CreateLocalUser("bob", "bob@example.com", "hash", models.RoleViewer)

	alice, _ := s.GetUser("alice")
	bob, _ := s.GetUser("bob")

	// Create a rule
	rule := &models.Rule{Name: "Test Rule", Type: models.RuleTypeConcurrentStreams, Enabled: true, Config: json.RawMessage(`{"max_streams": 2}`)}
	if err := s.CreateRule(rule); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}

	// Add violation for bob
	if _, err := s.db.Exec(`INSERT INTO rule_violations (rule_id, user_name, severity, message, confidence_score, occurred_at)
		VALUES (?, 'bob', 'warning', 'test violation', 0.8, '2024-01-01T00:00:00Z')`, rule.ID); err != nil {
		t.Fatalf("insert violation: %v", err)
	}

	// Merge bob into alice
	if _, err := s.MergeUsers(alice.ID, bob.ID); err != nil {
		t.Fatalf("MergeUsers: %v", err)
	}

	// Verify violation transferred to alice
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM rule_violations WHERE user_name = 'alice'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 violation for alice, got %d", count)
	}
}

// Test that MergeUsers preserves keep-user data when delete-user has none
func TestMergeUsers_PreservesKeepUserData(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create admin and two users
	s.CreateLocalUser("admin", "admin@example.com", "hash", models.RoleAdmin)
	s.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	s.CreateLocalUser("bob", "bob@example.com", "hash", models.RoleViewer)

	alice, _ := s.GetUser("alice")
	bob, _ := s.GetUser("bob")

	// Add trust score and household location for alice (keep user)
	if _, err := s.db.Exec(`INSERT INTO user_trust_scores (user_name, score) VALUES ('alice', 85)`); err != nil {
		t.Fatalf("insert trust score: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO household_locations (user_name, city, country, session_count, first_seen, last_seen)
		VALUES ('alice', 'NYC', 'US', 10, '2024-01-01', '2024-01-10')`); err != nil {
		t.Fatalf("insert household location: %v", err)
	}

	// bob has NO trust score or household locations

	// Merge bob into alice - alice's data should be preserved
	_, err := s.MergeUsers(alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("MergeUsers: %v", err)
	}

	// Verify alice's trust score is preserved
	var score int
	err = s.db.QueryRow(`SELECT score FROM user_trust_scores WHERE user_name = 'alice'`).Scan(&score)
	if err != nil {
		t.Fatalf("alice's trust score should be preserved: %v", err)
	}
	if score != 85 {
		t.Errorf("expected score 85, got %d", score)
	}

	// Verify alice's household location is preserved
	var city string
	err = s.db.QueryRow(`SELECT city FROM household_locations WHERE user_name = 'alice'`).Scan(&city)
	if err != nil {
		t.Fatalf("alice's household location should be preserved: %v", err)
	}
	if city != "NYC" {
		t.Errorf("expected city NYC, got %s", city)
	}
}

// Test that MergeUsers transfers delete-user data when it exists
func TestMergeUsers_TransfersDeleteUserData(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create admin and two users
	s.CreateLocalUser("admin", "admin@example.com", "hash", models.RoleAdmin)
	s.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	s.CreateLocalUser("bob", "bob@example.com", "hash", models.RoleViewer)

	alice, _ := s.GetUser("alice")
	bob, _ := s.GetUser("bob")

	// Add trust score for alice
	if _, err := s.db.Exec(`INSERT INTO user_trust_scores (user_name, score) VALUES ('alice', 85)`); err != nil {
		t.Fatalf("insert alice trust score: %v", err)
	}

	// Add trust score for bob (delete user) - should replace alice's
	if _, err := s.db.Exec(`INSERT INTO user_trust_scores (user_name, score) VALUES ('bob', 50)`); err != nil {
		t.Fatalf("insert bob trust score: %v", err)
	}

	// Merge bob into alice
	_, err := s.MergeUsers(alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("MergeUsers: %v", err)
	}

	// Verify bob's trust score was transferred (replaces alice's)
	var score int
	err = s.db.QueryRow(`SELECT score FROM user_trust_scores WHERE user_name = 'alice'`).Scan(&score)
	if err != nil {
		t.Fatalf("trust score query: %v", err)
	}
	if score != 50 {
		t.Errorf("expected bob's score (50) to replace alice's, got %d", score)
	}
}

// Test that provider conflicts are not treated as name conflicts
func TestGetOrLinkUser_ProviderConflictNotRetried(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create user with specific provider
	if _, err := s.db.Exec(`INSERT INTO users (name, email, role, provider, provider_id) VALUES (?, ?, ?, ?, ?)`,
		"alice", "alice@example.com", models.RoleViewer, "plex", "12345"); err != nil {
		t.Fatalf("create alice: %v", err)
	}

	// Try to create user with SAME provider_id but different name
	// This should fail with provider conflict, not create "bob_2"
	_, err := s.GetOrLinkUser("bob@example.com", []string{}, "bob", "plex", "12345", "")

	// Should return existing alice (found by provider), not create new user
	if err != nil {
		t.Fatalf("GetOrLinkUser: %v", err)
	}
}

// Test that MergeUsers handles conflicting rule violations (same rule_id + session_key)
func TestMergeUsers_ViolationConflict(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create admin and two users
	s.CreateLocalUser("admin", "admin@example.com", "hash", models.RoleAdmin)
	s.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	s.CreateLocalUser("bob", "bob@example.com", "hash", models.RoleViewer)

	alice, _ := s.GetUser("alice")
	bob, _ := s.GetUser("bob")

	// Create a rule
	rule := &models.Rule{Name: "Test Rule", Type: models.RuleTypeConcurrentStreams, Enabled: true, Config: json.RawMessage(`{"max_streams": 2}`)}
	if err := s.CreateRule(rule); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}

	// Add violation for both users with SAME rule_id and session_key (conflict scenario)
	if _, err := s.db.Exec(`INSERT INTO rule_violations (rule_id, user_name, severity, message, confidence_score, occurred_at, session_key)
		VALUES (?, 'alice', 'warning', 'alice violation', 0.8, '2024-01-01T00:00:00Z', 'session-123')`, rule.ID); err != nil {
		t.Fatalf("insert alice violation: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO rule_violations (rule_id, user_name, severity, message, confidence_score, occurred_at, session_key)
		VALUES (?, 'bob', 'critical', 'bob violation', 0.9, '2024-01-02T00:00:00Z', 'session-123')`, rule.ID); err != nil {
		t.Fatalf("insert bob violation: %v", err)
	}

	// Merge bob into alice - should handle conflict by keeping bob's violation
	_, err := s.MergeUsers(alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("MergeUsers: %v", err)
	}

	// Verify only one violation remains (bob's, transferred to alice)
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM rule_violations WHERE user_name = 'alice'`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 violation for alice after merge, got %d", count)
	}

	// Verify it's bob's violation (severity=critical)
	var severity string
	s.db.QueryRow(`SELECT severity FROM rule_violations WHERE user_name = 'alice'`).Scan(&severity)
	if severity != "critical" {
		t.Errorf("expected bob's critical violation to be kept, got severity %s", severity)
	}
}

// Test that MergeUsers preserves empty session_key violations (no unique constraint for those)
func TestMergeUsers_ViolationEmptySessionKey(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	// Create admin and two users
	s.CreateLocalUser("admin", "admin@example.com", "hash", models.RoleAdmin)
	s.CreateLocalUser("alice", "alice@example.com", "hash", models.RoleViewer)
	s.CreateLocalUser("bob", "bob@example.com", "hash", models.RoleViewer)

	alice, _ := s.GetUser("alice")
	bob, _ := s.GetUser("bob")

	// Create a rule
	rule := &models.Rule{Name: "Test Rule", Type: models.RuleTypeConcurrentStreams, Enabled: true, Config: json.RawMessage(`{"max_streams": 2}`)}
	if err := s.CreateRule(rule); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}

	// Add violations with EMPTY session_key for both users (no unique conflict)
	if _, err := s.db.Exec(`INSERT INTO rule_violations (rule_id, user_name, severity, message, confidence_score, occurred_at, session_key)
		VALUES (?, 'alice', 'warning', 'alice violation 1', 0.8, '2024-01-01T00:00:00Z', '')`, rule.ID); err != nil {
		t.Fatalf("insert alice violation: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO rule_violations (rule_id, user_name, severity, message, confidence_score, occurred_at, session_key)
		VALUES (?, 'bob', 'critical', 'bob violation', 0.9, '2024-01-02T00:00:00Z', '')`, rule.ID); err != nil {
		t.Fatalf("insert bob violation: %v", err)
	}

	// Merge bob into alice - both violations should be preserved (no unique constraint on empty session_key)
	_, err := s.MergeUsers(alice.ID, bob.ID)
	if err != nil {
		t.Fatalf("MergeUsers: %v", err)
	}

	// Verify BOTH violations are preserved for alice
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM rule_violations WHERE user_name = 'alice'`).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 violations for alice after merge (both preserved), got %d", count)
	}
}
