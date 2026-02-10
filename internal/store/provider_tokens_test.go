package store

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"streammon/internal/crypto"
	"streammon/internal/models"
)

func testEncryptor(t *testing.T) *crypto.Encryptor {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	enc, err := crypto.NewEncryptor(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	return enc
}

func testStoreWithEncryptor(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:", WithEncryptor(testEncryptor(t)))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(migrationsDir()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func testStoreNoEncryptor(t *testing.T) *Store {
	t.Helper()
	return newTestStoreWithMigrations(t)
}

func TestStoreProviderToken_Roundtrip(t *testing.T) {
	s := testStoreWithEncryptor(t)
	user := createTestUser(t, s, "alice-pt", "alice-pt@test.local")

	if err := s.StoreProviderToken(user.ID, "plex", "plex-token-abc"); err != nil {
		t.Fatalf("store: %v", err)
	}

	token, err := s.GetProviderToken(user.ID, "plex")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if token != "plex-token-abc" {
		t.Fatalf("expected %q, got %q", "plex-token-abc", token)
	}
}

func TestStoreProviderToken_Upsert(t *testing.T) {
	s := testStoreWithEncryptor(t)
	user := createTestUser(t, s, "bob-pt", "bob-pt@test.local")

	s.StoreProviderToken(user.ID, "plex", "old-token")
	s.StoreProviderToken(user.ID, "plex", "new-token")

	token, _ := s.GetProviderToken(user.ID, "plex")
	if token != "new-token" {
		t.Fatalf("expected %q, got %q", "new-token", token)
	}
}

func TestStoreProviderToken_StoredAsCiphertext(t *testing.T) {
	s := testStoreWithEncryptor(t)
	user := createTestUser(t, s, "cipher-pt", "cipher-pt@test.local")

	plaintext := "my-secret-plex-token"
	if err := s.StoreProviderToken(user.ID, "plex", plaintext); err != nil {
		t.Fatal(err)
	}

	var raw string
	s.db.QueryRow(`SELECT token FROM provider_tokens WHERE user_id = ? AND provider = ?`, user.ID, "plex").Scan(&raw)
	if raw == "" {
		t.Fatal("expected non-empty stored token")
	}
	if raw == plaintext {
		t.Fatal("token stored as plaintext, expected ciphertext")
	}
}

func TestStoreProviderToken_NoEncryptor(t *testing.T) {
	s := testStoreNoEncryptor(t)
	user := createTestUser(t, s, "carol-pt", "carol-pt@test.local")

	err := s.StoreProviderToken(user.ID, "plex", "some-token")
	if err == nil {
		t.Fatal("expected error when no encryptor")
	}
}

func TestGetProviderToken_NotFound(t *testing.T) {
	s := testStoreWithEncryptor(t)

	token, err := s.GetProviderToken(9999, "plex")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "" {
		t.Fatalf("expected empty, got %q", token)
	}
}

func TestDeleteProviderToken(t *testing.T) {
	s := testStoreWithEncryptor(t)
	user := createTestUser(t, s, "dave-pt", "dave-pt@test.local")

	s.StoreProviderToken(user.ID, "plex", "my-token")
	if err := s.DeleteProviderToken(user.ID, "plex"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	token, _ := s.GetProviderToken(user.ID, "plex")
	if token != "" {
		t.Fatalf("expected empty after delete, got %q", token)
	}
}

func TestDeleteProviderTokensByProvider(t *testing.T) {
	s := testStoreWithEncryptor(t)
	u1 := createTestUser(t, s, "eve-pt", "eve-pt@test.local")
	u2 := createTestUser(t, s, "frank-pt", "frank-pt@test.local")

	s.StoreProviderToken(u1.ID, "plex", "token1")
	s.StoreProviderToken(u2.ID, "plex", "token2")

	if err := s.DeleteProviderTokensByProvider("plex"); err != nil {
		t.Fatalf("delete by provider: %v", err)
	}

	t1, _ := s.GetProviderToken(u1.ID, "plex")
	t2, _ := s.GetProviderToken(u2.ID, "plex")
	if t1 != "" || t2 != "" {
		t.Fatal("expected all plex tokens deleted")
	}
}

func TestDeleteProviderTokensByProvider_ScopedToProvider(t *testing.T) {
	s := testStoreWithEncryptor(t)
	u := createTestUser(t, s, "scoped-pt", "scoped-pt@test.local")

	s.StoreProviderToken(u.ID, "plex", "plex-token")
	s.StoreProviderToken(u.ID, "emby", "emby-token")

	s.DeleteProviderTokensByProvider("plex")

	plex, _ := s.GetProviderToken(u.ID, "plex")
	emby, _ := s.GetProviderToken(u.ID, "emby")
	if plex != "" {
		t.Fatal("expected plex token deleted")
	}
	if emby != "emby-token" {
		t.Fatalf("expected emby token preserved, got %q", emby)
	}
}

func TestStorePlexTokensSetting(t *testing.T) {
	s := testStoreWithEncryptor(t)

	enabled, err := s.GetStorePlexTokens()
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("expected disabled by default")
	}

	if err := s.SetStorePlexTokens(true); err != nil {
		t.Fatal(err)
	}
	enabled, _ = s.GetStorePlexTokens()
	if !enabled {
		t.Fatal("expected enabled")
	}

	if err := s.SetStorePlexTokens(false); err != nil {
		t.Fatal(err)
	}
	enabled, _ = s.GetStorePlexTokens()
	if enabled {
		t.Fatal("expected disabled")
	}
}

func TestSetStorePlexTokens_DisableDeletesTokens(t *testing.T) {
	s := testStoreWithEncryptor(t)
	user := createTestUser(t, s, "grace-pt", "grace-pt@test.local")

	s.StoreProviderToken(user.ID, "plex", "my-token")
	s.SetStorePlexTokens(true)

	if err := s.SetStorePlexTokens(false); err != nil {
		t.Fatal(err)
	}

	token, _ := s.GetProviderToken(user.ID, "plex")
	if token != "" {
		t.Fatal("expected tokens deleted when feature disabled")
	}
}

func TestUnlinkUserProvider_DeletesToken(t *testing.T) {
	s := testStoreWithEncryptor(t)

	user, err := s.CreateLocalUser("unlink-pt", "unlink-pt@test.local", "password", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	s.LinkProviderAccount(user.ID, "plex", "12345")
	s.StoreProviderToken(user.ID, "plex", "my-plex-token")

	if err := s.UnlinkUserProvider(user.ID); err != nil {
		t.Fatal(err)
	}

	token, _ := s.GetProviderToken(user.ID, "plex")
	if token != "" {
		t.Fatal("expected token deleted after unlink")
	}
}

func TestUnlinkUserProvider_FailsOnTokenDeletionError(t *testing.T) {
	s := testStoreWithEncryptor(t)

	user, err := s.CreateLocalUser("unlink-fail", "unlink-fail@test.local", "password", models.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	s.LinkProviderAccount(user.ID, "plex", "99999")
	s.StoreProviderToken(user.ID, "plex", "my-plex-token")

	// Close the DB to force DeleteProviderToken to fail
	s.Close()

	err = s.UnlinkUserProvider(user.ID)
	if err == nil {
		t.Fatal("expected error when DB is closed, got nil")
	}
}

func TestGetProviderToken_NoEncryptor(t *testing.T) {
	s := testStoreNoEncryptor(t)

	token, err := s.GetProviderToken(9999, "plex")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "" {
		t.Fatalf("expected empty, got %q", token)
	}
}
