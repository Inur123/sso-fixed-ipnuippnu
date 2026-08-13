package utils

import (
	"regexp"
	"testing"
)

func TestGenerateEmailOTPHasExactlySixDigits(t *testing.T) {
	for range 100 {
		code, err := GenerateEmailOTP()
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^[0-9]{6}$`).MatchString(code) {
			t.Fatalf("unexpected OTP format %q", code)
		}
	}
}

func TestEmailOTPUsesKeyedUserBoundHash(t *testing.T) {
	t.Setenv("OTP_HASH_SECRET", "0123456789abcdef0123456789abcdef")
	hash, err := HashEmailOTP("user-a", "012345")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "012345" || !VerifyEmailOTP("user-a", "012345", hash) {
		t.Fatal("expected OTP hash to verify")
	}
	if VerifyEmailOTP("user-b", "012345", hash) || VerifyEmailOTP("user-a", "012346", hash) {
		t.Fatal("OTP hash must be bound to both user and code")
	}
}
