package controllers

import (
	"errors"
	"testing"

	"sso-backend/models"
)

func TestClientResponseMarksLegacyHashOnlySecretUnavailable(t *testing.T) {
	legacy := models.OAuthClient{ID: "legacy", SecretHash: "bcrypt-hash"}
	if got := clientResponse(legacy, ""); got.SecretAvailable || got.SecretVersion != 1 {
		t.Fatal("legacy hash-only client must not advertise a revealable secret")
	}

	ciphertext := "v1:ciphertext"
	current := models.OAuthClient{ID: "current", SecretHash: "bcrypt-hash", SecretCiphertext: &ciphertext, SecretVersion: 7}
	if got := clientResponse(current, ""); !got.SecretAvailable || got.SecretVersion != 7 {
		t.Fatal("client with ciphertext must advertise an available secret")
	}
}

func TestNextClientSecretVersionRejectsStaleVersion(t *testing.T) {
	if _, err := nextClientSecretVersion(4, 3); !errors.Is(err, errSecretVersionConflict) {
		t.Fatalf("stale expected version must conflict: %v", err)
	}
	if next, err := nextClientSecretVersion(4, 4); err != nil || next != 5 {
		t.Fatalf("matching version must increment exactly once: next=%d err=%v", next, err)
	}
	if next, err := nextClientSecretVersion(0, 1); err != nil || next != 2 {
		t.Fatalf("legacy zero value must normalize to version one: next=%d err=%v", next, err)
	}
}

func TestNextClientSecretVersionRejectsOverflow(t *testing.T) {
	maximum := ^uint64(0)
	if _, err := nextClientSecretVersion(maximum, maximum); !errors.Is(err, errSecretVersionExhausted) {
		t.Fatalf("maximum version must not wrap to zero: %v", err)
	}
}

func TestNormalizeClientScopesDefaultsToDashboardScopes(t *testing.T) {
	scopes, err := normalizeClientScopes(nil)
	if err != nil {
		t.Fatalf("default scopes must be valid: %v", err)
	}
	want := []string{"email", "openid", "profile"}
	if len(scopes) != len(want) {
		t.Fatalf("got %v want %v", scopes, want)
	}
	for index := range want {
		if scopes[index] != want[index] {
			t.Fatalf("got %v want %v", scopes, want)
		}
	}
}

func TestAssignmentIdentifierRequiresCompleteEmailOrUUID(t *testing.T) {
	valid := []string{"anggota@example.com", "852fcc86-51a7-435b-9ee5-61d7df7c4e71"}
	for _, value := range valid {
		if !validAssignmentIdentifier(value) {
			t.Fatalf("complete identifier %q must be valid", value)
		}
	}
	invalid := []string{"", "anggota", "anggota@", "anggota@example", "852fcc86-51a7"}
	for _, value := range invalid {
		if validAssignmentIdentifier(value) {
			t.Fatalf("partial identifier %q must be invalid", value)
		}
	}
}

func TestValidateRedirectURIsRejectsUnsafeValues(t *testing.T) {
	for _, value := range []string{
		"https://example.org/callback#fragment",
		"http://example.org/callback",
		"javascript:alert(1)",
	} {
		if _, err := validateRedirectURIs([]string{value}); err == nil {
			t.Fatalf("redirect URI %q must be rejected", value)
		}
	}
	if values, err := validateRedirectURIs([]string{"http://localhost:3002/callback", "https://example.org/callback"}); err != nil || len(values) != 2 {
		t.Fatalf("safe development/production URIs must pass: values=%v err=%v", values, err)
	}
}
