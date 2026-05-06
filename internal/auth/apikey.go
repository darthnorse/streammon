package auth

import (
	"crypto/rand"
	"crypto/sha256"
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

func HashAPIKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func CompareAPIKeyHash(storedHash, plain string) bool {
	if storedHash == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(HashAPIKey(plain))) == 1
}
