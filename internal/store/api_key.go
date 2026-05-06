package store

import (
	"fmt"
	"time"
)

const (
	apiKeyValueSetting     = "api_key.value"
	apiKeyCreatedAtSetting = "api_key.created_at"

	// Legacy key from the hash-only design — cleared on next SetAPIKey/ClearAPIKey.
	apiKeyLegacyHashSetting = "api_key.hash"
)

// GetAPIKey returns the decrypted API key plaintext, or "" when no key is
// configured. Used both for X-API-Key auth comparison and to display the key
// in the admin UI.
func (s *Store) GetAPIKey() (string, error) {
	raw, err := s.GetSetting(apiKeyValueSetting)
	if err != nil {
		return "", err
	}
	if raw == "" {
		return "", nil
	}
	return s.decryptValue(raw)
}

func (s *Store) GetAPIKeyCreatedAt() (time.Time, error) {
	raw, err := s.GetSetting(apiKeyCreatedAtSetting)
	if err != nil {
		return time.Time{}, err
	}
	if raw == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing api key created_at: %w", err)
	}
	return t.UTC(), nil
}

// SetAPIKey persists the plaintext (encrypted at rest if a TOKEN_ENCRYPTION_KEY
// is configured) and the creation timestamp atomically. Sweeps the legacy hash
// setting from the previous design.
func (s *Store) SetAPIKey(plain string, createdAt time.Time) error {
	encVal, err := s.encryptValue(plain)
	if err != nil {
		return fmt.Errorf("encrypting api key: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(settingUpsert, apiKeyValueSetting, encVal); err != nil {
		return fmt.Errorf("setting api key value: %w", err)
	}
	if _, err := tx.Exec(settingUpsert, apiKeyCreatedAtSetting, createdAt.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("setting api key created_at: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM settings WHERE key = ?`, apiKeyLegacyHashSetting); err != nil {
		return fmt.Errorf("clearing legacy api key hash: %w", err)
	}
	return tx.Commit()
}

func (s *Store) ClearAPIKey() error {
	_, err := s.db.Exec(
		`DELETE FROM settings WHERE key IN (?, ?, ?)`,
		apiKeyValueSetting, apiKeyCreatedAtSetting, apiKeyLegacyHashSetting,
	)
	if err != nil {
		return fmt.Errorf("clearing api key: %w", err)
	}
	return nil
}
