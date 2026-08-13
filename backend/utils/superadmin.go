package utils

import (
	"os"
	"strings"
)

// IsConfiguredSuperAdminEmail membandingkan email secara case-insensitive.
// Nilai kosong tidak pernah dianggap sebagai akun super admin.
func IsConfiguredSuperAdminEmail(email string) bool {
	configured := strings.TrimSpace(os.Getenv("SUPER_ADMIN_EMAIL"))
	candidate := strings.TrimSpace(email)
	return configured != "" && candidate != "" && strings.EqualFold(configured, candidate)
}
