package store

import (
	"database/sql"
	"errors"
	"fmt"
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
