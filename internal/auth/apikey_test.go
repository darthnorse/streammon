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

func TestCompareAPIKey(t *testing.T) {
	plain := "sm_secret"
	if !CompareAPIKey(plain, plain) {
		t.Error("expected match for equal values")
	}
	if CompareAPIKey(plain, "sm_other") {
		t.Error("expected mismatch for different values")
	}
	if CompareAPIKey("", plain) {
		t.Error("empty stored value must never match")
	}
}
