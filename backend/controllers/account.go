package controllers

import (
	"net/http"

	"sso-backend/models"
)

func accountAccessError(user *models.User) (int, string, string, bool) {
	if user == nil {
		return http.StatusUnauthorized, "unauthorized", "Login diperlukan.", true
	}
	if !user.IsActive {
		return http.StatusForbidden, "account_inactive", "Akun Anda dinonaktifkan. Hubungi super admin IPNU IPPNU ID.", true
	}
	if user.EmailVerifiedAt == nil {
		return http.StatusForbidden, "email_unverified", "Email belum diverifikasi. Masukkan OTP yang dikirim ke email Anda.", true
	}
	return 0, "", "", false
}
