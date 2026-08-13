package utils

import (
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestVerificationMessageUsesIPNUIPPNUBrandAndDoesNotLeakHTML(t *testing.T) {
	t.Setenv("APP_NAME", "IPNU IPPNU ID")
	message := string(buildVerificationMessage(
		mail.Address{Name: "IPNU IPPNU ID", Address: "noreply@example.com"},
		mail.Address{Address: "member@example.com"},
		"<script>alert(1)</script>",
		"012345",
		10*time.Minute,
	))
	if !strings.Contains(message, "IPNU IPPNU ID") || !strings.Contains(message, "012345") {
		t.Fatal("message must include brand and OTP")
	}
	if strings.Contains(message, "<p>Halo <script>") {
		t.Fatal("recipient name must be escaped in HTML")
	}
}
