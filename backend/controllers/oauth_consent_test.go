package controllers

import (
	"testing"
	"time"

	"sso-backend/models"
)

func TestParseOAuthPrompt(t *testing.T) {
	prompts, err := parseOAuthPrompt("select_account consent")
	if err != nil || !prompts[promptSelectAccount] || !prompts[promptConsent] {
		t.Fatalf("expected supported prompts, got %#v, %v", prompts, err)
	}
	if _, err := parseOAuthPrompt("none select_account"); err == nil {
		t.Fatal("none must not be combined with another prompt")
	}
	if _, err := parseOAuthPrompt("unknown"); err == nil {
		t.Fatal("unsupported prompt must be rejected")
	}
}

func TestConsentCoversRequestedScope(t *testing.T) {
	consent := models.OAuthConsent{ID: "consent", Scope: "email openid profile"}
	if !consentCovers(consent, "openid email") {
		t.Fatal("stored consent should cover a requested subset")
	}
	if consentCovers(consent, "openid roles") {
		t.Fatal("stored consent must not cover an ungranted scope")
	}
	now := time.Now()
	consent.RevokedAt = &now
	if consentCovers(consent, "openid") {
		t.Fatal("revoked consent must not be accepted")
	}
}
