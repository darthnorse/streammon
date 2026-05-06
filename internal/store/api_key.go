package store

import (
	"fmt"
	"time"
)

const (
	apiKeyHashSetting      = "api_key.hash"
	apiKeyCreatedAtSetting = "api_key.created_at"
)

func (s *Store) GetAPIKeyHash() (string, error) {
	return s.GetSetting(apiKeyHashSetting)
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

func (s *Store) SetAPIKey(hash string, createdAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(settingUpsert, apiKeyHashSetting, hash); err != nil {
		return fmt.Errorf("setting api key hash: %w", err)
	}
	if _, err := tx.Exec(settingUpsert, apiKeyCreatedAtSetting, createdAt.UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("setting api key created_at: %w", err)
	}
	return tx.Commit()
}

func (s *Store) ClearAPIKey() error {
	_, err := s.db.Exec(
		`DELETE FROM settings WHERE key IN (?, ?)`,
		apiKeyHashSetting, apiKeyCreatedAtSetting,
	)
	if err != nil {
		return fmt.Errorf("clearing api key: %w", err)
	}
	return nil
}
