package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey_Format(t *testing.T) {
	k, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if !strings.HasPrefix(k, APIKeyPrefix) {
		t.Errorf("expected prefix %q, got %q", APIKeyPrefix, k)
	}
	body := strings.TrimPrefix(k, APIKeyPrefix)
	if len(body) != 64 { // 32 bytes hex-encoded
		t.Errorf("expected 64 hex chars after prefix, got %d", len(body))
	}
}

func TestGenerateAPIKey_Unique(t *testing.T) {
	a, _ := GenerateAPIKey()
	b, _ := GenerateAPIKey()
	if a == b {
		t.Error("two consecutive keys collided")
	}
}

func TestHashAPIKey_Deterministic(t *testing.T) {
	in := "sm_abc123"
	if HashAPIKey(in) != HashAPIKey(in) {
		t.Error("HashAPIKey is not deterministic")
	}
}

func TestHashAPIKey_DiffersByInput(t *testing.T) {
	if HashAPIKey("sm_a") == HashAPIKey("sm_b") {
		t.Error("different inputs produced same hash")
	}
}

func TestCompareAPIKeyHash(t *testing.T) {
	plain := "sm_secret"
	hash := HashAPIKey(plain)
	if !CompareAPIKeyHash(hash, plain) {
		t.Error("expected match for correct key")
	}
	if CompareAPIKeyHash(hash, "sm_wrong") {
		t.Error("expected mismatch for wrong key")
	}
	if CompareAPIKeyHash("", plain) {
		t.Error("empty stored hash must never match")
	}
}
