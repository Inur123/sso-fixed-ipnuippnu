package utils

import "testing"

func TestIsConfiguredSuperAdminEmail(t *testing.T) {
	t.Setenv("SUPER_ADMIN_EMAIL", " Configured.Admin@Example.test ")
	if !IsConfiguredSuperAdminEmail("configured.admin@example.test") {
		t.Fatal("expected case-insensitive configured email to match")
	}
	if IsConfiguredSuperAdminEmail("anggota@example.com") {
		t.Fatal("unexpected match for a different email")
	}
	t.Setenv("SUPER_ADMIN_EMAIL", "")
	if IsConfiguredSuperAdminEmail("") {
		t.Fatal("empty configuration must never match")
	}
}
