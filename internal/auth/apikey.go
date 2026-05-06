package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
)

const (
	APIKeyPrefix    = "sm_"
	apiKeyByteCount = 32
)

func GenerateAPIKey() (string, error) {
	b := make([]byte, apiKeyByteCount)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return APIKeyPrefix + hex.EncodeToString(b), nil
}

// CompareAPIKey constant-time-compares the stored plaintext against an
// incoming candidate. An empty stored value never matches.
func CompareAPIKey(stored, candidate string) bool {
	if stored == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(candidate)) == 1
}
