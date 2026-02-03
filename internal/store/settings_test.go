package store

import "testing"

func TestSetAndGetSetting(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	err := s.SetSetting("theme", "dark")
	if err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	val, err := s.GetSetting("theme")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "dark" {
		t.Fatalf("expected dark, got %s", val)
	}
}

func TestGetSettingNotFound(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	val, err := s.GetSetting("nonexistent")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty string, got %s", val)
	}
}

func TestSetSettingOverwrite(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.SetSetting("key", "v1")
	s.SetSetting("key", "v2")

	val, _ := s.GetSetting("key")
	if val != "v2" {
		t.Fatalf("expected v2, got %s", val)
	}
}

func TestOIDCConfigRoundTrip(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	cfg := OIDCConfig{
		Issuer:       "https://issuer.example.com",
		ClientID:     "my-client",
		ClientSecret: "my-secret",
		RedirectURL:  "https://app.example.com/callback",
	}
	if err := s.SetOIDCConfig(cfg); err != nil {
		t.Fatalf("SetOIDCConfig: %v", err)
	}

	got, err := s.GetOIDCConfig()
	if err != nil {
		t.Fatalf("GetOIDCConfig: %v", err)
	}
	if got != cfg {
		t.Fatalf("expected %+v, got %+v", cfg, got)
	}
}

func TestOIDCConfigEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	got, err := s.GetOIDCConfig()
	if err != nil {
		t.Fatalf("GetOIDCConfig: %v", err)
	}
	if got != (OIDCConfig{}) {
		t.Fatalf("expected zero value, got %+v", got)
	}
}

func TestOIDCConfigSecretPreservation(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	cfg := OIDCConfig{
		Issuer:       "https://issuer.example.com",
		ClientID:     "my-client",
		ClientSecret: "original-secret",
		RedirectURL:  "https://app.example.com/callback",
	}
	if err := s.SetOIDCConfig(cfg); err != nil {
		t.Fatalf("SetOIDCConfig: %v", err)
	}

	updated := OIDCConfig{
		Issuer:       "https://issuer2.example.com",
		ClientID:     "new-client",
		ClientSecret: "",
		RedirectURL:  "https://app2.example.com/callback",
	}
	if err := s.SetOIDCConfig(updated); err != nil {
		t.Fatalf("SetOIDCConfig: %v", err)
	}

	got, err := s.GetOIDCConfig()
	if err != nil {
		t.Fatalf("GetOIDCConfig: %v", err)
	}
	if got.Issuer != "https://issuer2.example.com" {
		t.Fatalf("expected updated issuer, got %s", got.Issuer)
	}
	if got.ClientID != "new-client" {
		t.Fatalf("expected updated client_id, got %s", got.ClientID)
	}
	if got.ClientSecret != "original-secret" {
		t.Fatalf("expected preserved secret, got %s", got.ClientSecret)
	}
	if got.RedirectURL != "https://app2.example.com/callback" {
		t.Fatalf("expected updated redirect_url, got %s", got.RedirectURL)
	}
}

func TestDeleteOIDCConfig(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.SetOIDCConfig(OIDCConfig{
		Issuer:       "https://issuer.example.com",
		ClientID:     "my-client",
		ClientSecret: "my-secret",
		RedirectURL:  "https://app.example.com/callback",
	})

	if err := s.DeleteOIDCConfig(); err != nil {
		t.Fatalf("DeleteOIDCConfig: %v", err)
	}

	got, err := s.GetOIDCConfig()
	if err != nil {
		t.Fatalf("GetOIDCConfig: %v", err)
	}
	if got != (OIDCConfig{}) {
		t.Fatalf("expected zero value after delete, got %+v", got)
	}
}
