package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"streammon/internal/units"
)

const encryptedPrefix = "enc:"

func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting setting: %w", err)
	}
	return value, nil
}

type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}


func (s *Store) GetOIDCConfig() (OIDCConfig, error) {
	var cfg OIDCConfig
	var err error
	if cfg.Issuer, err = s.GetSetting("oidc.issuer"); err != nil {
		return cfg, err
	}
	if cfg.ClientID, err = s.GetSetting("oidc.client_id"); err != nil {
		return cfg, err
	}
	if cfg.ClientSecret, err = s.GetSetting("oidc.client_secret"); err != nil {
		return cfg, err
	}
	if cfg.RedirectURL, err = s.GetSetting("oidc.redirect_url"); err != nil {
		return cfg, err
	}
	return cfg, nil
}

const settingUpsert = `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`

func (s *Store) SetOIDCConfig(cfg OIDCConfig) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, kv := range []struct{ k, v string }{
		{"oidc.issuer", cfg.Issuer},
		{"oidc.client_id", cfg.ClientID},
		{"oidc.redirect_url", cfg.RedirectURL},
	} {
		if _, err := tx.Exec(settingUpsert, kv.k, kv.v); err != nil {
			return fmt.Errorf("setting %q: %w", kv.k, err)
		}
	}
	if cfg.ClientSecret != "" {
		if _, err := tx.Exec(settingUpsert, "oidc.client_secret", cfg.ClientSecret); err != nil {
			return fmt.Errorf("setting %q: %w", "oidc.client_secret", err)
		}
	}

	return tx.Commit()
}

func (s *Store) DeleteOIDCConfig() error {
	_, err := s.db.Exec(`DELETE FROM settings WHERE key IN ('oidc.issuer', 'oidc.client_id', 'oidc.client_secret', 'oidc.redirect_url')`)
	if err != nil {
		return fmt.Errorf("deleting OIDC config: %w", err)
	}
	return nil
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(settingUpsert, key, value)
	if err != nil {
		return fmt.Errorf("setting %q: %w", key, err)
	}
	return nil
}

type TautulliConfig = IntegrationConfig

func (s *Store) GetTautulliConfig() (TautulliConfig, error)  { return s.getIntegrationConfig("tautulli") }
func (s *Store) SetTautulliConfig(cfg TautulliConfig) error   { return s.setIntegrationConfig("tautulli", cfg) }
func (s *Store) DeleteTautulliConfig() error                  { return s.deleteIntegrationConfig("tautulli") }

type IntegrationConfig struct {
	URL     string
	APIKey  string
	Enabled bool
}

type OverseerrConfig = IntegrationConfig
type SonarrConfig = IntegrationConfig

func (s *Store) getIntegrationConfig(prefix string) (IntegrationConfig, error) {
	var cfg IntegrationConfig
	var err error
	if cfg.URL, err = s.GetSetting(prefix + ".url"); err != nil {
		return cfg, err
	}
	raw, err := s.GetSetting(prefix + ".api_key")
	if err != nil {
		return cfg, err
	}
	if strings.HasPrefix(raw, encryptedPrefix) {
		if s.encryptor == nil {
			return cfg, fmt.Errorf("api key is encrypted but no encryption key configured")
		}
		cfg.APIKey, err = s.encryptor.Decrypt(strings.TrimPrefix(raw, encryptedPrefix))
		if err != nil {
			return cfg, fmt.Errorf("decrypting %s api key: %w", prefix, err)
		}
	} else {
		cfg.APIKey = raw
	}
	enabled, err := s.GetSetting(prefix + ".enabled")
	if err != nil {
		return cfg, err
	}
	cfg.Enabled = enabled != "0"
	return cfg, nil
}

func (s *Store) setIntegrationConfig(prefix string, cfg IntegrationConfig) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(settingUpsert, prefix+".url", cfg.URL); err != nil {
		return fmt.Errorf("setting %q: %w", prefix+".url", err)
	}
	if cfg.APIKey != "" {
		apiKeyVal := cfg.APIKey
		if s.encryptor != nil {
			encrypted, err := s.encryptor.Encrypt(cfg.APIKey)
			if err != nil {
				return fmt.Errorf("encrypting %s api key: %w", prefix, err)
			}
			apiKeyVal = encryptedPrefix + encrypted
		}
		if _, err := tx.Exec(settingUpsert, prefix+".api_key", apiKeyVal); err != nil {
			return fmt.Errorf("setting %q: %w", prefix+".api_key", err)
		}
	}
	enabledVal := "1"
	if !cfg.Enabled {
		enabledVal = "0"
	}
	if _, err := tx.Exec(settingUpsert, prefix+".enabled", enabledVal); err != nil {
		return fmt.Errorf("setting %q: %w", prefix+".enabled", err)
	}

	return tx.Commit()
}

func (s *Store) deleteIntegrationConfig(prefix string) error {
	_, err := s.db.Exec(`DELETE FROM settings WHERE key IN (?, ?, ?)`,
		prefix+".url", prefix+".api_key", prefix+".enabled")
	if err != nil {
		return fmt.Errorf("deleting %s config: %w", prefix, err)
	}
	return nil
}

func (s *Store) GetOverseerrConfig() (OverseerrConfig, error) { return s.getIntegrationConfig("overseerr") }
func (s *Store) SetOverseerrConfig(cfg OverseerrConfig) error  { return s.setIntegrationConfig("overseerr", cfg) }
func (s *Store) DeleteOverseerrConfig() error                  { return s.deleteIntegrationConfig("overseerr") }

func (s *Store) GetSonarrConfig() (SonarrConfig, error) { return s.getIntegrationConfig("sonarr") }
func (s *Store) SetSonarrConfig(cfg SonarrConfig) error  { return s.setIntegrationConfig("sonarr", cfg) }
func (s *Store) DeleteSonarrConfig() error               { return s.deleteIntegrationConfig("sonarr") }

type RadarrConfig = IntegrationConfig

func (s *Store) GetRadarrConfig() (RadarrConfig, error) { return s.getIntegrationConfig("radarr") }
func (s *Store) SetRadarrConfig(cfg RadarrConfig) error  { return s.setIntegrationConfig("radarr", cfg) }
func (s *Store) DeleteRadarrConfig() error               { return s.deleteIntegrationConfig("radarr") }

const unitSystemKey = "display.units"

func (s *Store) GetUnitSystem() (string, error) {
	val, err := s.GetSetting(unitSystemKey)
	if err != nil {
		return "", err
	}
	if val == "" {
		return "metric", nil
	}
	return val, nil
}

func (s *Store) SetUnitSystem(system string) error {
	if !units.IsValid(system) {
		return fmt.Errorf("invalid unit system: %s", system)
	}
	return s.SetSetting(unitSystemKey, system)
}

const watchedThresholdKey = "session.watched_threshold"
const defaultWatchedThreshold = 85

func (s *Store) GetWatchedThreshold() (int, error) {
	val, err := s.GetSetting(watchedThresholdKey)
	if err != nil {
		return 0, err
	}
	if val == "" {
		return defaultWatchedThreshold, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultWatchedThreshold, nil
	}
	return n, nil
}

func (s *Store) SetWatchedThreshold(pct int) error {
	if pct < 1 || pct > 100 {
		return fmt.Errorf("watched threshold must be between 1 and 100, got %d", pct)
	}
	return s.SetSetting(watchedThresholdKey, strconv.Itoa(pct))
}

var guestSettingKeys = []string{
	"access_enabled", "store_plex_tokens", "show_discover",
	"visible_trust_score", "visible_violations", "visible_watch_history",
	"visible_household", "visible_devices", "visible_isps",
}

func (s *Store) GetGuestSettings() (map[string]bool, error) {
	result := make(map[string]bool, len(guestSettingKeys))
	for _, k := range guestSettingKeys {
		result[k] = true // default all true
	}

	rows, err := s.db.Query(`SELECT key, value FROM settings WHERE key LIKE 'guest.%'`)
	if err != nil {
		return nil, fmt.Errorf("querying guest settings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scanning guest setting: %w", err)
		}
		short := strings.TrimPrefix(key, "guest.")
		result[short] = value == "true"
	}
	return result, rows.Err()
}

func (s *Store) SetGuestSettings(updates map[string]bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for k, v := range updates {
		val := "false"
		if v {
			val = "true"
		}
		if _, err := tx.Exec(settingUpsert, "guest."+k, val); err != nil {
			return fmt.Errorf("setting guest.%s: %w", k, err)
		}
	}
	return tx.Commit()
}

// GetGuestSetting returns a single guest setting value (defaults to true).
func (s *Store) GetGuestSetting(key string) (bool, error) {
	val, err := s.GetSetting("guest." + key)
	if err != nil {
		return true, err
	}
	if val == "" {
		return true, nil
	}
	return val == "true", nil
}

const idleTimeoutKey = "session.idle_timeout_minutes"
const DefaultIdleTimeoutMinutes = 5

func (s *Store) GetIdleTimeoutMinutes() (int, error) {
	val, err := s.GetSetting(idleTimeoutKey)
	if err != nil {
		return 0, err
	}
	if val == "" {
		return DefaultIdleTimeoutMinutes, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return DefaultIdleTimeoutMinutes, nil
	}
	return n, nil
}

const MaxIdleTimeoutMinutes = 1440 // 24 hours

func (s *Store) SetIdleTimeoutMinutes(min int) error {
	if min < 0 || min > MaxIdleTimeoutMinutes {
		return fmt.Errorf("idle timeout must be between 0 and %d, got %d", MaxIdleTimeoutMinutes, min)
	}
	return s.SetSetting(idleTimeoutKey, strconv.Itoa(min))
}

