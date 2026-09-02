package utils

import "testing"

func TestEmailOTPCipherRoundTripAndBindsOutboxID(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("JWT_SECRET", "development-test-secret-with-at-least-32-characters")
	t.Setenv("CLIENT_SECRET_ENCRYPTION_KEY", "")

	encoded, err := EncryptEmailOTP("11111111-1111-4111-8111-111111111111", "012345")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecryptEmailOTP("11111111-1111-4111-8111-111111111111", encoded)
	if err != nil || decoded != "012345" {
		t.Fatalf("decoded=%q err=%v", decoded, err)
	}
	if _, err := DecryptEmailOTP("22222222-2222-4222-8222-222222222222", encoded); err == nil {
		t.Fatal("ciphertext must be bound to its outbox ID")
	}
}
