package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	storePlexTokensKey = "guest.store_plex_tokens"
	ProviderPlex       = "plex"
)

// StoreProviderToken encrypts and stores a provider token for a user.
func (s *Store) StoreProviderToken(userID int64, provider, token string) error {
	if s.encryptor == nil {
		return fmt.Errorf("encryption not configured")
	}

	encrypted, err := s.encryptor.Encrypt(token)
	if err != nil {
		return fmt.Errorf("encrypting token: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO provider_tokens (user_id, provider, token, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id, provider) DO UPDATE SET token = excluded.token, updated_at = excluded.updated_at`,
		userID, provider, encrypted, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("storing provider token: %w", err)
	}
	return nil
}

// GetProviderToken retrieves and decrypts a stored provider token.
// Returns empty string (no error) if not found or if no encryptor is configured.
func (s *Store) GetProviderToken(userID int64, provider string) (string, error) {
	if s.encryptor == nil {
		return "", nil
	}

	var encrypted string
	err := s.db.QueryRow(
		`SELECT token FROM provider_tokens WHERE user_id = ? AND provider = ?`,
		userID, provider,
	).Scan(&encrypted)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting provider token: %w", err)
	}

	decrypted, err := s.encryptor.Decrypt(encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypting token: %w", err)
	}
	return decrypted, nil
}

// DeleteProviderToken removes a specific provider token for a user.
func (s *Store) DeleteProviderToken(userID int64, provider string) error {
	_, err := s.db.Exec(
		`DELETE FROM provider_tokens WHERE user_id = ? AND provider = ?`,
		userID, provider,
	)
	if err != nil {
		return fmt.Errorf("deleting provider token: %w", err)
	}
	return nil
}

// DeleteProviderTokensByProvider removes all stored tokens for a given provider.
func (s *Store) DeleteProviderTokensByProvider(provider string) error {
	_, err := s.db.Exec(`DELETE FROM provider_tokens WHERE provider = ?`, provider)
	if err != nil {
		return fmt.Errorf("deleting %s provider tokens: %w", provider, err)
	}
	return nil
}

func (s *Store) GetStorePlexTokens() (bool, error) {
	val, err := s.GetSetting(storePlexTokensKey)
	if err != nil {
		return false, err
	}
	return val == "true", nil
}

// SetStorePlexTokens enables or disables Plex token storage.
// When disabled, all stored Plex tokens are deleted.
func (s *Store) SetStorePlexTokens(enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	if err := s.SetSetting(storePlexTokensKey, val); err != nil {
		return err
	}
	if !enabled {
		return s.DeleteProviderTokensByProvider(ProviderPlex)
	}
	return nil
}
