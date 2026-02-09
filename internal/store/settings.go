package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"streammon/internal/units"
)

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

type TautulliConfig struct {
	URL    string
	APIKey string
}

func (s *Store) GetTautulliConfig() (TautulliConfig, error) {
	var cfg TautulliConfig
	var err error
	if cfg.URL, err = s.GetSetting("tautulli.url"); err != nil {
		return cfg, err
	}
	if cfg.APIKey, err = s.GetSetting("tautulli.api_key"); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (s *Store) SetTautulliConfig(cfg TautulliConfig) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(settingUpsert, "tautulli.url", cfg.URL); err != nil {
		return fmt.Errorf("setting %q: %w", "tautulli.url", err)
	}
	if cfg.APIKey != "" {
		if _, err := tx.Exec(settingUpsert, "tautulli.api_key", cfg.APIKey); err != nil {
			return fmt.Errorf("setting %q: %w", "tautulli.api_key", err)
		}
	}

	return tx.Commit()
}

func (s *Store) DeleteTautulliConfig() error {
	_, err := s.db.Exec(`DELETE FROM settings WHERE key IN ('tautulli.url', 'tautulli.api_key')`)
	if err != nil {
		return fmt.Errorf("deleting Tautulli config: %w", err)
	}
	return nil
}

type OverseerrConfig struct {
	URL    string
	APIKey string
}

func (s *Store) GetOverseerrConfig() (OverseerrConfig, error) {
	var cfg OverseerrConfig
	var err error
	if cfg.URL, err = s.GetSetting("overseerr.url"); err != nil {
		return cfg, err
	}
	if cfg.APIKey, err = s.GetSetting("overseerr.api_key"); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (s *Store) SetOverseerrConfig(cfg OverseerrConfig) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(settingUpsert, "overseerr.url", cfg.URL); err != nil {
		return fmt.Errorf("setting %q: %w", "overseerr.url", err)
	}
	if cfg.APIKey != "" {
		if _, err := tx.Exec(settingUpsert, "overseerr.api_key", cfg.APIKey); err != nil {
			return fmt.Errorf("setting %q: %w", "overseerr.api_key", err)
		}
	}

	return tx.Commit()
}

func (s *Store) DeleteOverseerrConfig() error {
	_, err := s.db.Exec(`DELETE FROM settings WHERE key IN ('overseerr.url', 'overseerr.api_key')`)
	if err != nil {
		return fmt.Errorf("deleting Overseerr config: %w", err)
	}
	return nil
}

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
