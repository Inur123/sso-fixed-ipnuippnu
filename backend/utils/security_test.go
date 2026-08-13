package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestVerifyPKCES256(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if !VerifyPKCES256(verifier, challenge) {
		t.Fatal("expected valid PKCE verifier")
	}
	if VerifyPKCES256(verifier+"x", challenge) {
		t.Fatal("expected invalid PKCE verifier")
	}
}

func TestScopeAllowed(t *testing.T) {
	if !ScopeAllowed("email profile", "profile email") {
		t.Fatal("expected normalized scopes to be allowed")
	}
	if ScopeAllowed("admin profile", "profile email") {
		t.Fatal("unexpected privilege escalation")
	}
}
