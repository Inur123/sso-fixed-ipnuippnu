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

func TestPromptConsentDoesNotInvalidateDurableGrant(t *testing.T) {
	prompts, err := parseOAuthPrompt("consent")
	if err != nil || !prompts[promptConsent] {
		t.Fatalf("expected consent prompt to remain accepted for compatibility, got %#v, %v", prompts, err)
	}

	consent := models.OAuthConsent{ID: "consent", Scope: "email openid profile"}
	if !consentCovers(consent, "email openid profile") {
		t.Fatal("prompt=consent must not invalidate an active durable grant")
	}
}

func TestRequiredConsentMustBeExplicitlyApproved(t *testing.T) {
	tests := []struct {
		name     string
		required bool
		approved bool
		missing  bool
	}{
		{name: "account selection with existing consent", required: false, approved: false, missing: false},
		{name: "explicit submit with existing consent", required: false, approved: true, missing: false},
		{name: "new consent without approval", required: true, approved: false, missing: true},
		{name: "new consent explicitly approved", required: true, approved: true, missing: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := missingRequiredConsentApproval(tt.required, tt.approved); got != tt.missing {
				t.Fatalf("missingRequiredConsentApproval(%t, %t) = %t, want %t", tt.required, tt.approved, got, tt.missing)
			}
		})
	}
}
