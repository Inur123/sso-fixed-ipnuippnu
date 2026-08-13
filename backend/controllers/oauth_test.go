package controllers

import (
	"errors"
	"net/url"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"sso-backend/models"
)

func TestOAuthRedirectURLPreservesExistingQuery(t *testing.T) {
	result, err := oauthRedirectURL("https://client.example/callback?tenant=nu", map[string]string{"code": "abc", "state": "xyz"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(result)
	if parsed.Query().Get("tenant") != "nu" || parsed.Query().Get("code") != "abc" || parsed.Query().Get("state") != "xyz" {
		t.Fatalf("unexpected redirect URL: %s", result)
	}
}

func TestVerifyOAuthClientSecretHashRejectsStaleSecretWithTypedError(t *testing.T) {
	currentHash, err := bcrypt.GenerateFromPassword([]byte("current-secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyOAuthClientSecretHash(string(currentHash), "current-secret"); err != nil {
		t.Fatalf("current secret was rejected: %v", err)
	}
	if err := verifyOAuthClientSecretHash(string(currentHash), "rotated-out-secret"); !errors.Is(err, errInvalidOAuthClientCredentials) {
		t.Fatalf("stale secret must return the typed invalid-client error: %v", err)
	}
}

func TestExactRedirectMatch(t *testing.T) {
	client := models.OAuthClient{RedirectURIs: "https://client.example/callback\nhttp://localhost:4000/callback"}
	if !exactRedirectMatch(client, "https://client.example/callback") {
		t.Fatal("expected registered URI to match")
	}
	if exactRedirectMatch(client, "https://client.example/callback/extra") {
		t.Fatal("redirect URI must use exact matching")
	}
}
