package utils

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIDTokenUsesRS256KidAndPublishedJWKS(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("OIDC_PRIVATE_KEY_PATH", "")
	t.Setenv("BACKEND_PUBLIC_URL", "https://identity.example.com")
	authTime := time.Now().Add(-time.Minute)
	raw, err := GenerateIDToken("user", "Nama", "user@example.com", true, "client", "nonce-123", "openid profile email", authTime)
	if err != nil {
		t.Fatal(err)
	}
	set, err := OIDCJWKS()
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Keys) != 1 || set.Keys[0].Algorithm != "RS256" || set.Keys[0].Use != "sig" || set.Keys[0].KeyID == "" {
		t.Fatalf("unexpected JWKS: %#v", set)
	}
	publicKey, err := DecodeJWKPublicKey(set.Keys[0])
	if err != nil {
		t.Fatal(err)
	}
	claims := &IDTokenClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodRS256 || token.Header["kid"] != set.Keys[0].KeyID {
			t.Fatalf("unexpected token header: %#v", token.Header)
		}
		return publicKey, nil
	}, jwt.WithIssuer("https://identity.example.com"), jwt.WithAudience("client"), jwt.WithValidMethods([]string{"RS256"}))
	if err != nil || !token.Valid {
		t.Fatalf("invalid ID token: %v", err)
	}
	if claims.Nonce != "nonce-123" || claims.Name != "Nama" || claims.Email != "user@example.com" || !claims.EmailVerified {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestProductionRequiresPersistentOIDCPrivateKey(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("OIDC_PRIVATE_KEY_PATH", "")
	if err := ValidateOIDCConfiguration(); err == nil {
		t.Fatal("expected production without OIDC_PRIVATE_KEY_PATH to fail")
	}
}

func TestRejectsInvalidOIDCPrivateKeyPEM(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	path := t.TempDir() + "/oidc.pem"
	if err := os.WriteFile(path, []byte("not a PEM key"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OIDC_PRIVATE_KEY_PATH", path)
	if err := ValidateOIDCConfiguration(); err == nil {
		t.Fatal("expected invalid private key to fail")
	}
}
