package store

import (
	"testing"
	"time"

	"streammon/internal/models"
)

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

	if err := s.SetSetting("key", "v1"); err != nil {
		t.Fatalf("SetSetting v1: %v", err)
	}
	if err := s.SetSetting("key", "v2"); err != nil {
		t.Fatalf("SetSetting v2: %v", err)
	}

	val, err := s.GetSetting("key")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
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

	if err := s.SetOIDCConfig(OIDCConfig{
		Issuer:       "https://issuer.example.com",
		ClientID:     "my-client",
		ClientSecret: "my-secret",
		RedirectURL:  "https://app.example.com/callback",
	}); err != nil {
		t.Fatalf("SetOIDCConfig: %v", err)
	}

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

func TestIntegrationConfigs(t *testing.T) {
	type configOps struct {
		get    func(*Store) (IntegrationConfig, error)
		set    func(*Store, IntegrationConfig) error
		delete func(*Store) error
	}

	integrations := []struct {
		name string
		url  string
		ops  configOps
	}{
		{"tautulli", "http://localhost:8181", configOps{
			func(s *Store) (IntegrationConfig, error) { return s.GetTautulliConfig() },
			func(s *Store, c IntegrationConfig) error { return s.SetTautulliConfig(c) },
			func(s *Store) error { return s.DeleteTautulliConfig() },
		}},
		{"overseerr", "http://localhost:5055", configOps{
			func(s *Store) (IntegrationConfig, error) { return s.GetOverseerrConfig() },
			func(s *Store, c IntegrationConfig) error { return s.SetOverseerrConfig(c) },
			func(s *Store) error { return s.DeleteOverseerrConfig() },
		}},
		{"sonarr", "http://localhost:8989", configOps{
			func(s *Store) (IntegrationConfig, error) { return s.GetSonarrConfig() },
			func(s *Store, c IntegrationConfig) error { return s.SetSonarrConfig(c) },
			func(s *Store) error { return s.DeleteSonarrConfig() },
		}},
		{"radarr", "http://localhost:7878", configOps{
			func(s *Store) (IntegrationConfig, error) { return s.GetRadarrConfig() },
			func(s *Store, c IntegrationConfig) error { return s.SetRadarrConfig(c) },
			func(s *Store) error { return s.DeleteRadarrConfig() },
		}},
	}

	for _, ic := range integrations {
		t.Run(ic.name+"/round trip", func(t *testing.T) {
			s := newTestStoreWithMigrations(t)
			cfg := IntegrationConfig{URL: ic.url, APIKey: "my-" + ic.name + "-key", Enabled: true}
			if err := ic.ops.set(s, cfg); err != nil {
				t.Fatalf("Set: %v", err)
			}
			got, err := ic.ops.get(s)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got != cfg {
				t.Fatalf("expected %+v, got %+v", cfg, got)
			}
		})

		t.Run(ic.name+"/api key preservation", func(t *testing.T) {
			s := newTestStoreWithMigrations(t)
			cfg := IntegrationConfig{URL: ic.url, APIKey: "original-key", Enabled: true}
			if err := ic.ops.set(s, cfg); err != nil {
				t.Fatalf("Set: %v", err)
			}
			updated := IntegrationConfig{URL: "http://newhost:9999", APIKey: "", Enabled: true}
			if err := ic.ops.set(s, updated); err != nil {
				t.Fatalf("Set updated: %v", err)
			}
			got, err := ic.ops.get(s)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.URL != "http://newhost:9999" {
				t.Fatalf("expected updated URL, got %s", got.URL)
			}
			if got.APIKey != "original-key" {
				t.Fatalf("expected preserved API key, got %s", got.APIKey)
			}
		})

		t.Run(ic.name+"/delete", func(t *testing.T) {
			s := newTestStoreWithMigrations(t)
			if err := ic.ops.set(s, IntegrationConfig{URL: ic.url, APIKey: "my-key", Enabled: true}); err != nil {
				t.Fatalf("Set: %v", err)
			}
			if err := ic.ops.delete(s); err != nil {
				t.Fatalf("Delete: %v", err)
			}
			got, err := ic.ops.get(s)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.URL != "" || got.APIKey != "" {
				t.Fatalf("expected empty URL/APIKey after delete, got %+v", got)
			}
		})

		t.Run(ic.name+"/empty defaults", func(t *testing.T) {
			s := newTestStoreWithMigrations(t)
			got, err := ic.ops.get(s)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.URL != "" || got.APIKey != "" {
				t.Fatalf("expected empty URL/APIKey, got %+v", got)
			}
			if !got.Enabled {
				t.Fatal("expected Enabled=true (default) for unconfigured integration")
			}
		})
	}
}

func TestTautulliConfigOverwrite(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetTautulliConfig(TautulliConfig{URL: "http://localhost:8181", APIKey: "key1", Enabled: true}); err != nil {
		t.Fatalf("SetTautulliConfig: %v", err)
	}
	if err := s.SetTautulliConfig(TautulliConfig{URL: "http://newhost:8181", APIKey: "key2", Enabled: true}); err != nil {
		t.Fatalf("SetTautulliConfig: %v", err)
	}

	got, err := s.GetTautulliConfig()
	if err != nil {
		t.Fatalf("GetTautulliConfig: %v", err)
	}
	if got.URL != "http://newhost:8181" || got.APIKey != "key2" {
		t.Fatalf("expected overwritten config, got %+v", got)
	}
}

func TestIntegrationConfigEncryptedRoundTrip(t *testing.T) {
	s := newTestStoreWithMigrations(t, WithEncryptor(testEncryptor(t)))

	cfg := SonarrConfig{URL: "http://localhost:8989", APIKey: "secret-api-key", Enabled: true}
	if err := s.SetSonarrConfig(cfg); err != nil {
		t.Fatalf("SetSonarrConfig: %v", err)
	}

	raw, err := s.GetSetting("sonarr.api_key")
	if err != nil {
		t.Fatal(err)
	}
	if raw == "secret-api-key" {
		t.Fatal("API key stored in plaintext despite encryptor being configured")
	}
	if raw[:4] != "enc:" {
		t.Fatalf("expected enc: prefix, got %q", raw[:10])
	}

	got, err := s.GetSonarrConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got != cfg {
		t.Fatalf("expected %+v, got %+v", cfg, got)
	}
}

func TestIntegrationConfigPlaintextUpgrade(t *testing.T) {
	s := newTestStoreWithMigrations(t, WithEncryptor(testEncryptor(t)))

	if err := s.SetSetting("overseerr.url", "http://localhost:5055"); err != nil {
		t.Fatalf("SetSetting url: %v", err)
	}
	if err := s.SetSetting("overseerr.api_key", "plaintext-key"); err != nil {
		t.Fatalf("SetSetting api_key: %v", err)
	}
	if err := s.SetSetting("overseerr.enabled", "1"); err != nil {
		t.Fatalf("SetSetting enabled: %v", err)
	}

	got, err := s.GetOverseerrConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "plaintext-key" {
		t.Fatalf("expected plaintext-key, got %s", got.APIKey)
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

	if err := s.SetWatchedThreshold(60); err != nil {
		t.Fatalf("SetWatchedThreshold(60): %v", err)
	}
	if err := s.SetWatchedThreshold(90); err != nil {
		t.Fatalf("SetWatchedThreshold(90): %v", err)
	}

	val, err := s.GetWatchedThreshold()
	if err != nil {
		t.Fatalf("GetWatchedThreshold: %v", err)
	}
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

func TestIdleTimeoutDefault(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	val, err := s.GetIdleTimeoutMinutes()
	if err != nil {
		t.Fatalf("GetIdleTimeoutMinutes: %v", err)
	}
	if val != 5 {
		t.Fatalf("expected default 5, got %d", val)
	}
}

func TestIdleTimeoutRoundTrip(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetIdleTimeoutMinutes(10); err != nil {
		t.Fatalf("SetIdleTimeoutMinutes: %v", err)
	}

	val, err := s.GetIdleTimeoutMinutes()
	if err != nil {
		t.Fatalf("GetIdleTimeoutMinutes: %v", err)
	}
	if val != 10 {
		t.Fatalf("expected 10, got %d", val)
	}
}

func TestIdleTimeoutDisabled(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetIdleTimeoutMinutes(0); err != nil {
		t.Fatalf("SetIdleTimeoutMinutes(0): %v", err)
	}

	val, err := s.GetIdleTimeoutMinutes()
	if err != nil {
		t.Fatalf("GetIdleTimeoutMinutes: %v", err)
	}
	if val != 0 {
		t.Fatalf("expected 0 (disabled), got %d", val)
	}
}

func TestIdleTimeoutInvalidValues(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	if err := s.SetIdleTimeoutMinutes(-1); err == nil {
		t.Fatal("expected error for -1")
	}
	if err := s.SetIdleTimeoutMinutes(1441); err == nil {
		t.Fatal("expected error for 1441 (exceeds max 1440)")
	}
	if err := s.SetIdleTimeoutMinutes(1440); err != nil {
		t.Fatalf("expected 1440 to be valid, got error: %v", err)
	}
}

func TestCleanupZombieSessions(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	srv := &models.Server{Name: "srv", Type: models.ServerTypePlex, URL: "http://x", APIKey: "k", Enabled: true}
	if err := s.CreateServer(srv); err != nil {
		t.Fatal(err)
	}

	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID:  srv.ID,
		UserName:  "alice",
		Title:     "Zombie Movie",
		MediaType: models.MediaTypeMovie,
		StartedAt: time.Now().UTC().Add(-48 * time.Hour),
		StoppedAt: time.Now().UTC(),
		WatchedMs: 1800000,
	}); err != nil {
		t.Fatalf("InsertHistory zombie: %v", err)
	}

	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID:  srv.ID,
		UserName:  "bob",
		Title:     "Normal Movie",
		MediaType: models.MediaTypeMovie,
		StartedAt: time.Now().UTC().Add(-2 * time.Hour),
		StoppedAt: time.Now().UTC().Add(-30 * time.Minute),
		WatchedMs: 5400000,
	}); err != nil {
		t.Fatalf("InsertHistory normal: %v", err)
	}

	if err := s.InsertHistory(&models.WatchHistoryEntry{
		ServerID:  srv.ID,
		UserName:  "charlie",
		Title:     "Zero Progress Zombie",
		MediaType: models.MediaTypeMovie,
		StartedAt: time.Now().UTC().Add(-72 * time.Hour),
		StoppedAt: time.Now().UTC(),
		WatchedMs: 0,
	}); err != nil {
		t.Fatalf("InsertHistory zero-progress: %v", err)
	}

	if err := s.CleanupZombieSessions(); err != nil {
		t.Fatalf("CleanupZombieSessions: %v", err)
	}

	result, err := s.ListHistory(1, 10, "", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 3 {
		t.Fatalf("expected 3 history entries, got %d", result.Total)
	}

	for _, item := range result.Items {
		wallTime := item.StoppedAt.Sub(item.StartedAt)
		switch item.Title {
		case "Zombie Movie":
			// Should be capped: started_at + watched_ms/1000 + 300 seconds = 30 min + 5 min = 35 min
			if wallTime > 36*time.Minute {
				t.Errorf("zombie wall time should be capped, got %v", wallTime)
			}
		case "Zero Progress Zombie":
			// Should be capped: stopped_at = started_at for zero-progress
			if wallTime > time.Second {
				t.Errorf("zero-progress zombie wall time should be ~0, got %v", wallTime)
			}
		}
	}

	if err := s.CleanupZombieSessions(); err != nil {
		t.Fatalf("second CleanupZombieSessions: %v", err)
	}
}

func TestGuestSettingsDefaults(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	gs, err := s.GetGuestSettings()
	if err != nil {
		t.Fatalf("GetGuestSettings: %v", err)
	}

	optInKeys := map[string]bool{
		"access_enabled":    true,
		"store_plex_tokens": true,
	}

	for key, val := range gs {
		if optInKeys[key] {
			if val {
				t.Errorf("expected %s to default false (opt-in), got true", key)
			}
		} else {
			if !val {
				t.Errorf("expected %s to default true, got false", key)
			}
		}
	}
	expected := []string{
		"access_enabled", "store_plex_tokens", "show_discover",
		"visible_trust_score", "visible_violations", "visible_watch_history",
		"visible_household", "visible_devices", "visible_isps",
	}
	for _, k := range expected {
		if _, ok := gs[k]; !ok {
			t.Errorf("missing key %s", k)
		}
	}
}

func TestGuestSettingsRoundTrip(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	updates := map[string]bool{
		"visible_devices": false,
		"visible_isps":    false,
	}
	if err := s.SetGuestSettings(updates); err != nil {
		t.Fatalf("SetGuestSettings: %v", err)
	}

	gs, err := s.GetGuestSettings()
	if err != nil {
		t.Fatalf("GetGuestSettings: %v", err)
	}
	if gs["visible_devices"] {
		t.Fatal("expected visible_devices=false")
	}
	if gs["visible_isps"] {
		t.Fatal("expected visible_isps=false")
	}
	if !gs["visible_trust_score"] {
		t.Fatal("expected visible_trust_score=true (unchanged)")
	}
}

func TestGuestSettingSingle(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	val, err := s.GetGuestSetting("visible_devices")
	if err != nil {
		t.Fatal(err)
	}
	if !val {
		t.Fatal("expected default true")
	}

	if err = s.SetGuestSettings(map[string]bool{"visible_devices": false}); err != nil {
		t.Fatalf("SetGuestSettings: %v", err)
	}
	val, err = s.GetGuestSetting("visible_devices")
	if err != nil {
		t.Fatalf("GetGuestSetting: %v", err)
	}
	if val {
		t.Fatal("expected false after update")
	}
}

func TestGuestSettingSingleFalseDefaults(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	for _, key := range []string{"access_enabled", "store_plex_tokens"} {
		val, err := s.GetGuestSetting(key)
		if err != nil {
			t.Fatalf("GetGuestSetting(%s): %v", key, err)
		}
		if val {
			t.Errorf("expected %s to default false, got true", key)
		}
	}
}

func TestGuestSettingsRejectUnknownKey(t *testing.T) {
	s := newTestStoreWithMigrations(t)

	err := s.SetGuestSettings(map[string]bool{"bogus_key": true})
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}
