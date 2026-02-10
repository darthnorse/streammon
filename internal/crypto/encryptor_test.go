package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func generateTestKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestNewEncryptor_ValidKey(t *testing.T) {
	keyStr := generateTestKey(t)
	enc, err := NewEncryptor(keyStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enc == nil {
		t.Fatal("expected non-nil encryptor")
	}
}

func TestNewEncryptor_InvalidBase64(t *testing.T) {
	_, err := NewEncryptor("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestNewEncryptor_WrongKeyLength(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("tooshort"))
	_, err := NewEncryptor(short)
	if err == nil {
		t.Fatal("expected error for wrong key length")
	}
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	enc, err := NewEncryptor(generateTestKey(t))
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "this-is-a-secret-plex-token-abc123"
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if ciphertext == plaintext {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncrypt_ProducesDifferentCiphertexts(t *testing.T) {
	enc, err := NewEncryptor(generateTestKey(t))
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "same-token"
	c1, _ := enc.Encrypt(plaintext)
	c2, _ := enc.Encrypt(plaintext)

	if c1 == c2 {
		t.Fatal("two encryptions of the same plaintext should produce different ciphertexts (unique nonce)")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	enc1, _ := NewEncryptor(generateTestKey(t))
	enc2, _ := NewEncryptor(generateTestKey(t))

	ciphertext, _ := enc1.Encrypt("secret")
	_, err := enc2.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	enc, _ := NewEncryptor(generateTestKey(t))
	ciphertext, _ := enc.Encrypt("secret")

	// Tamper with the ciphertext
	raw, _ := base64.StdEncoding.DecodeString(ciphertext)
	raw[len(raw)-1] ^= 0xff
	tampered := base64.StdEncoding.EncodeToString(raw)

	_, err := enc.Decrypt(tampered)
	if err == nil {
		t.Fatal("expected error when decrypting tampered ciphertext")
	}
}

func TestDecrypt_EmptyString(t *testing.T) {
	enc, _ := NewEncryptor(generateTestKey(t))
	_, err := enc.Decrypt("")
	if err == nil {
		t.Fatal("expected error for empty ciphertext")
	}
}
