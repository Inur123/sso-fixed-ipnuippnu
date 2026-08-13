package controllers

import (
	"strings"
	"testing"
)

func TestNewAuditEventRejectsUnknownActionAndSanitizesSecrets(t *testing.T) {
	if _, ok := newAuditEvent(nil, "unknown.action", "user", "id", "safe", "127.0.0.1"); ok {
		t.Fatal("unknown audit action must be rejected")
	}
	event, ok := newAuditEvent(nil, AuditEmailVerify, "user", "id", "OTP: 123456", "127.0.0.1")
	if !ok {
		t.Fatal("known audit action should be accepted")
	}
	if strings.Contains(event.Description, "123456") || event.Description != "Detail sensitif disembunyikan." {
		t.Fatalf("sensitive description was not redacted: %q", event.Description)
	}
}

func TestNewAuditEventTruncatesUntrustedDisplayFields(t *testing.T) {
	long := strings.Repeat("x", 700)
	event, ok := newAuditEvent(nil, AuditAuthLoginFailed, long, long, long, long)
	if !ok {
		t.Fatal("known audit action should be accepted")
	}
	if len([]rune(event.TargetType)) != 80 || len([]rune(event.TargetID)) != 128 || len([]rune(event.Description)) != 500 || len([]rune(event.IPAddress)) != 64 {
		t.Fatalf("unexpected audit field lengths: %d %d %d %d", len([]rune(event.TargetType)), len([]rune(event.TargetID)), len([]rune(event.Description)), len([]rune(event.IPAddress)))
	}
}

func TestEscapeAuditLike(t *testing.T) {
	if got := escapeAuditLike(`a%b_c\d`); got != `a\%b\_c\\d` {
		t.Fatalf("unexpected escaped search: %q", got)
	}
}

func TestAuditLoginContextUsesStructuredFields(t *testing.T) {
	event, ok := newAuditEvent(nil, AuditAuthLogin, "session", "session-id", "Login berhasil.", "127.0.0.1")
	if !ok {
		t.Fatal("expected known login action")
	}
	latitude, longitude, accuracy := -7.6499, 111.3381, 25.0
	event.Device = "MacIntel; id-ID; Asia/Jakarta"
	event.Latitude = &latitude
	event.Longitude = &longitude
	event.Accuracy = &accuracy
	if event.Device == "" || event.Latitude == nil || event.Longitude == nil || event.Accuracy == nil {
		t.Fatal("expected device and location audit fields")
	}
}
