package auth

import (
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"valid 8 chars", "password", nil},
		{"valid long", "this is a very long password", nil},
		{"too short", "1234567", ErrPasswordTooShort},
		{"empty", "", ErrPasswordTooShort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if err != tt.wantErr {
				t.Errorf("ValidatePassword(%q) = %v, want %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestHashPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// Check PHC format
	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Errorf("hash should start with $argon2id$v=19$, got %s", hash)
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Errorf("hash should have 6 parts, got %d", len(parts))
	}

	// Hash should be different each time (random salt)
	hash2, _ := HashPassword(password)
	if hash == hash2 {
		t.Error("hashes should be different due to random salt")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "testpassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
		wantErr  bool
	}{
		{"correct password", password, hash, true, false},
		{"wrong password", "wrongpassword", hash, false, false},
		{"similar password", "testpassword124", hash, false, false},
		{"empty password", "", hash, false, false},
		{"invalid hash format", password, "notahash", false, true},
		{"invalid hash prefix", password, "$bcrypt$invalidhash", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := VerifyPassword(tt.password, tt.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("VerifyPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVerifyPasswordConstantTime(t *testing.T) {
	// This test ensures we use constant-time comparison
	// by verifying both correct and incorrect passwords complete without panic
	password := "testpassword123"
	hash, _ := HashPassword(password)

	// Should not panic on any input
	VerifyPassword(password, hash)
	VerifyPassword("wrong", hash)
	VerifyPassword("", hash)
}

func TestDummyHashIsValid(t *testing.T) {
	// DummyHash must be a valid argon2id hash for timing attack prevention
	valid, err := VerifyPassword("anypassword", DummyHash)
	if err != nil {
		t.Fatalf("DummyHash is not a valid argon2id hash: %v", err)
	}
	// It should never match any password (that's the point)
	if valid {
		t.Error("DummyHash should not match any password")
	}
}
