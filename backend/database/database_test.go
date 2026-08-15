package database

import (
	"testing"
	"time"

	"sso-backend/models"
)

func TestValidProductionSSLMode(t *testing.T) {
	tests := []struct {
		name string
		host string
		mode string
		want bool
	}{
		{name: "verified remote", host: "db.example.com", mode: "verify-full", want: true},
		{name: "verified ca remote", host: "db.example.com", mode: "verify-ca", want: true},
		{name: "loopback ipv4", host: "127.0.0.1", mode: "disable", want: true},
		{name: "loopback hostname", host: "localhost", mode: "disable", want: true},
		{name: "loopback ipv6", host: "::1", mode: "disable", want: true},
		{name: "remote disable rejected", host: "10.0.0.4", mode: "disable", want: false},
		{name: "weak remote mode rejected", host: "db.example.com", mode: "require", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validProductionSSLMode(tt.host, tt.mode); got != tt.want {
				t.Fatalf("validProductionSSLMode(%q, %q) = %t, want %t", tt.host, tt.mode, got, tt.want)
			}
		})
	}
}

func TestOAuthConsentBackfillCandidatesGroupsGrantByApplicationAndUser(t *testing.T) {
	earlier := time.Date(2026, time.August, 13, 8, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Hour)
	tokens := []models.OAuthToken{
		{ClientID: "client-a", UserID: "user-a", Scope: "openid profile", CreatedAt: later},
		{ClientID: "client-a", UserID: "user-a", Scope: "email openid", CreatedAt: earlier},
		{ClientID: "client-b", UserID: "user-a", Scope: "openid", CreatedAt: later},
	}

	candidates := oauthConsentBackfillCandidates(tokens)
	if len(candidates) != 2 {
		t.Fatalf("got %d candidates, want 2", len(candidates))
	}
	if got := candidates[0]; got.ClientID != "client-a" || got.UserID != "user-a" || got.Scope != "email openid profile" || !got.GrantedAt.Equal(earlier) {
		t.Fatalf("unexpected merged candidate: %#v", got)
	}
	if got := candidates[1]; got.ClientID != "client-b" || got.Scope != "openid" {
		t.Fatalf("unexpected second candidate: %#v", got)
	}
}
