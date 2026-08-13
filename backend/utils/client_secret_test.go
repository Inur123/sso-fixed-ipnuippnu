package utils

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGeneratedClientSecretsAreUniqueAndHaveFullEntropy(t *testing.T) {
	seen := make(map[string]struct{}, 128)
	for index := 0; index < 128; index++ {
		secret, err := RandomToken(32)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := base64.RawURLEncoding.DecodeString(secret)
		if err != nil || len(raw) != 32 {
			t.Fatalf("client secret must encode 32 random bytes: length=%d err=%v", len(raw), err)
		}
		if _, duplicate := seen[secret]; duplicate {
			t.Fatal("duplicate client secret generated")
		}
		seen[secret] = struct{}{}
	}
}

func TestClientSecretEncryptionRoundTripAndAAD(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("CLIENT_SECRET_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))

	ciphertext, err := EncryptClientSecret("client-a", "secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ciphertext, "secret-value") {
		t.Fatal("ciphertext contains plaintext")
	}
	secondCiphertext, err := EncryptClientSecret("client-a", "secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if ciphertext == secondCiphertext {
		t.Fatal("AES-GCM nonce must make repeated encryptions unique")
	}
	plaintext, err := DecryptClientSecret("client-a", ciphertext)
	if err != nil || plaintext != "secret-value" {
		t.Fatalf("round trip failed: plaintext=%q err=%v", plaintext, err)
	}
	if _, err := DecryptClientSecret("client-b", ciphertext); err == nil {
		t.Fatal("ciphertext must be bound to its client ID")
	}
}

func TestClientSecretEncryptionRejectsTampering(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("CLIENT_SECRET_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	ciphertext, err := EncryptClientSecret("client-a", "secret-value")
	if err != nil {
		t.Fatal(err)
	}
	value, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ciphertext, "v1:"))
	if err != nil {
		t.Fatal(err)
	}
	value[len(value)-1] ^= 0x01
	tampered := "v1:" + base64.RawURLEncoding.EncodeToString(value)
	if _, err := DecryptClientSecret("client-a", tampered); err == nil {
		t.Fatal("tampered ciphertext was accepted")
	}
}

func TestClientSecretEncryptionConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("CLIENT_SECRET_ENCRYPTION_KEY", "")
	if err := ValidateClientSecretEncryptionConfiguration(); err == nil {
		t.Fatal("production must require a dedicated encryption key")
	}

	t.Setenv("CLIENT_SECRET_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString([]byte("too-short")))
	if err := ValidateClientSecretEncryptionConfiguration(); err == nil {
		t.Fatal("short decoded key was accepted")
	}
	t.Setenv("CLIENT_SECRET_ENCRYPTION_KEY", "not-base64")
	if err := ValidateClientSecretEncryptionConfiguration(); err == nil {
		t.Fatal("malformed base64 key was accepted")
	}

	t.Setenv("APP_ENV", "development")
	t.Setenv("CLIENT_SECRET_ENCRYPTION_KEY", "")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	if err := ValidateClientSecretEncryptionConfiguration(); err != nil {
		t.Fatalf("development fallback failed: %v", err)
	}
}
