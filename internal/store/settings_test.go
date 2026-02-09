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

func TestTautulliConfigRoundTrip(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	cfg := TautulliConfig{
		URL:    "http://localhost:8181",
		APIKey: "my-tautulli-api-key",
	}
	if err := s.SetTautulliConfig(cfg); err != nil {
		t.Fatalf("SetTautulliConfig: %v", err)
	}

	got, err := s.GetTautulliConfig()
	if err != nil {
		t.Fatalf("GetTautulliConfig: %v", err)
	}
	if got != cfg {
		t.Fatalf("expected %+v, got %+v", cfg, got)
	}
}

func TestTautulliConfigOverwrite(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	cfg1 := TautulliConfig{
		URL:    "http://localhost:8181",
		APIKey: "key1",
	}
	s.SetTautulliConfig(cfg1)

	cfg2 := TautulliConfig{
		URL:    "http://newhost:8181",
		APIKey: "key2",
	}
	s.SetTautulliConfig(cfg2)

	got, _ := s.GetTautulliConfig()
	if got != cfg2 {
		t.Fatalf("expected %+v, got %+v", cfg2, got)
	}
}

func TestTautulliConfigAPIKeyPreservation(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	cfg := TautulliConfig{
		URL:    "http://localhost:8181",
		APIKey: "original-key",
	}
	if err := s.SetTautulliConfig(cfg); err != nil {
		t.Fatalf("SetTautulliConfig: %v", err)
	}

	updated := TautulliConfig{
		URL:    "http://newhost:8181",
		APIKey: "",
	}
	if err := s.SetTautulliConfig(updated); err != nil {
		t.Fatalf("SetTautulliConfig: %v", err)
	}

	got, err := s.GetTautulliConfig()
	if err != nil {
		t.Fatalf("GetTautulliConfig: %v", err)
	}
	if got.URL != "http://newhost:8181" {
		t.Fatalf("expected updated URL, got %s", got.URL)
	}
	if got.APIKey != "original-key" {
		t.Fatalf("expected preserved API key, got %s", got.APIKey)
	}
}

func TestTautulliConfigDelete(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.SetTautulliConfig(TautulliConfig{
		URL:    "http://localhost:8181",
		APIKey: "my-key",
	})

	if err := s.DeleteTautulliConfig(); err != nil {
		t.Fatalf("DeleteTautulliConfig: %v", err)
	}

	got, err := s.GetTautulliConfig()
	if err != nil {
		t.Fatalf("GetTautulliConfig: %v", err)
	}
	if got != (TautulliConfig{}) {
		t.Fatalf("expected zero value after delete, got %+v", got)
	}
}

func TestTautulliConfigEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	got, err := s.GetTautulliConfig()
	if err != nil {
		t.Fatalf("GetTautulliConfig: %v", err)
	}
	if got != (TautulliConfig{}) {
		t.Fatalf("expected zero value, got %+v", got)
	}
}

func TestOverseerrConfigRoundTrip(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	cfg := OverseerrConfig{
		URL:    "http://localhost:5055",
		APIKey: "my-overseerr-api-key",
	}
	if err := s.SetOverseerrConfig(cfg); err != nil {
		t.Fatalf("SetOverseerrConfig: %v", err)
	}

	got, err := s.GetOverseerrConfig()
	if err != nil {
		t.Fatalf("GetOverseerrConfig: %v", err)
	}
	if got != cfg {
		t.Fatalf("expected %+v, got %+v", cfg, got)
	}
}

func TestOverseerrConfigAPIKeyPreservation(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	cfg := OverseerrConfig{
		URL:    "http://localhost:5055",
		APIKey: "original-key",
	}
	if err := s.SetOverseerrConfig(cfg); err != nil {
		t.Fatalf("SetOverseerrConfig: %v", err)
	}

	updated := OverseerrConfig{
		URL:    "http://newhost:5055",
		APIKey: "",
	}
	if err := s.SetOverseerrConfig(updated); err != nil {
		t.Fatalf("SetOverseerrConfig: %v", err)
	}

	got, err := s.GetOverseerrConfig()
	if err != nil {
		t.Fatalf("GetOverseerrConfig: %v", err)
	}
	if got.URL != "http://newhost:5055" {
		t.Fatalf("expected updated URL, got %s", got.URL)
	}
	if got.APIKey != "original-key" {
		t.Fatalf("expected preserved API key, got %s", got.APIKey)
	}
}

func TestOverseerrConfigDelete(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.SetOverseerrConfig(OverseerrConfig{
		URL:    "http://localhost:5055",
		APIKey: "my-key",
	})

	if err := s.DeleteOverseerrConfig(); err != nil {
		t.Fatalf("DeleteOverseerrConfig: %v", err)
	}

	got, err := s.GetOverseerrConfig()
	if err != nil {
		t.Fatalf("GetOverseerrConfig: %v", err)
	}
	if got != (OverseerrConfig{}) {
		t.Fatalf("expected zero value after delete, got %+v", got)
	}
}

func TestOverseerrConfigEmpty(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	got, err := s.GetOverseerrConfig()
	if err != nil {
		t.Fatalf("GetOverseerrConfig: %v", err)
	}
	if got != (OverseerrConfig{}) {
		t.Fatalf("expected zero value, got %+v", got)
	}
}

func TestUnitSystemRoundTrip(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetUnitSystem("imperial"); err != nil {
		t.Fatalf("SetUnitSystem: %v", err)
	}

	val, err := s.GetUnitSystem()
	if err != nil {
		t.Fatalf("GetUnitSystem: %v", err)
	}
	if val != "imperial" {
		t.Fatalf("expected imperial, got %s", val)
	}
}

func TestUnitSystemDefault(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	val, err := s.GetUnitSystem()
	if err != nil {
		t.Fatalf("GetUnitSystem: %v", err)
	}
	if val != "metric" {
		t.Fatalf("expected metric (default), got %s", val)
	}
}

func TestUnitSystemInvalidValue(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	err := s.SetUnitSystem("invalid")
	if err == nil {
		t.Fatal("expected error for invalid unit system")
	}
}

func TestUnitSystemOverwrite(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetUnitSystem("imperial"); err != nil {
		t.Fatalf("SetUnitSystem imperial: %v", err)
	}
	if err := s.SetUnitSystem("metric"); err != nil {
		t.Fatalf("SetUnitSystem metric: %v", err)
	}

	val, err := s.GetUnitSystem()
	if err != nil {
		t.Fatalf("GetUnitSystem: %v", err)
	}
	if val != "metric" {
		t.Fatalf("expected metric, got %s", val)
	}
}

func TestWatchedThresholdDefault(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	val, err := s.GetWatchedThreshold()
	if err != nil {
		t.Fatalf("GetWatchedThreshold: %v", err)
	}
	if val != 85 {
		t.Fatalf("expected default 85, got %d", val)
	}
}

func TestWatchedThresholdRoundTrip(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetWatchedThreshold(50); err != nil {
		t.Fatalf("SetWatchedThreshold: %v", err)
	}

	val, err := s.GetWatchedThreshold()
	if err != nil {
		t.Fatalf("GetWatchedThreshold: %v", err)
	}
	if val != 50 {
		t.Fatalf("expected 50, got %d", val)
	}
}

func TestWatchedThresholdOverwrite(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	s.SetWatchedThreshold(60)
	s.SetWatchedThreshold(90)

	val, _ := s.GetWatchedThreshold()
	if val != 90 {
		t.Fatalf("expected 90, got %d", val)
	}
}

func TestWatchedThresholdInvalidValues(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetWatchedThreshold(0); err == nil {
		t.Fatal("expected error for 0")
	}
	if err := s.SetWatchedThreshold(101); err == nil {
		t.Fatal("expected error for 101")
	}
	if err := s.SetWatchedThreshold(-1); err == nil {
		t.Fatal("expected error for -1")
	}
}
