package utils

import (
	"testing"
	"time"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("BACKEND_PUBLIC_URL", "https://identity.example.com")
	token, err := GenerateAccessToken("user-id", "client-id", "profile email", "jti", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateAccessToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-id" || claims.ClientID != "client-id" || claims.TokenType != AccessTokenType || claims.Scope != "email profile" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestRejectsWeakJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")
	if _, err := GenerateAccessToken("u", "c", "profile", "j", time.Minute); err == nil {
		t.Fatal("expected weak secret to be rejected")
	}
}
