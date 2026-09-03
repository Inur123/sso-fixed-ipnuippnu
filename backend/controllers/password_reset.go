package controllers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sso-backend/database"
	"sso-backend/internal/apptime"
	"sso-backend/models"
	"sso-backend/passwordmailqueue"
	"sso-backend/utils"
)

const (
	passwordResetTokenLifetime  = 15 * time.Minute
	passwordResetCooldown       = time.Minute
	passwordResetResponseFloor  = 350 * time.Millisecond
	passwordResetRequestMessage = "Jika akun terdaftar, aktif, dan emailnya telah diverifikasi, tautan pengaturan ulang akan dikirim."
)

var (
	errInvalidPasswordResetToken = errors.New("invalid password reset token")
	errPasswordReuse             = errors.New("new password matches current password")
)

type PasswordResetRequest struct {
	Email string `json:"email" binding:"required,email,max=254"`
}

// RequestPasswordReset selalu memakai respons publik yang sama untuk email
// valid. Lookup, status akun, dan cooldown tidak pernah dibedakan di response.
func RequestPasswordReset(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req PasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Email yang valid wajib diisi.")
		return
	}
	startedAt := time.Now()
	queued := false
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("LOWER(email) = ?", normalizeEmail(req.Email)).Limit(1).Find(&user)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if !user.IsActive || user.EmailVerifiedAt == nil {
			return nil
		}
		now := apptime.Now()

		var latest models.PasswordResetToken
		latestResult := tx.Where("user_id = ?", user.ID).Order("created_at DESC").Limit(1).Find(&latest)
		if latestResult.Error != nil {
			return latestResult.Error
		}
		if latestResult.RowsAffected > 0 && now.Before(latest.CreatedAt.Add(passwordResetCooldown)) {
			return nil
		}
		var recentRequests int64
		if err := tx.Model(&models.PasswordResetToken{}).Where("user_id = ? AND created_at > ?", user.ID, now.Add(-time.Hour)).Count(&recentRequests).Error; err != nil {
			return err
		}
		if recentRequests >= 5 {
			return nil
		}
		if err := passwordmailqueue.RevokeUserResetAccess(tx, user.ID, now); err != nil {
			return err
		}
		rawToken, err := utils.RandomToken(32)
		if err != nil {
			return err
		}
		token := models.PasswordResetToken{
			UserID: user.ID, TokenHash: utils.HashToken(rawToken),
			RequestedIPAddress: c.ClientIP(), ExpiresAt: now.Add(passwordResetTokenLifetime),
		}
		if err := tx.Create(&token).Error; err != nil {
			return err
		}
		if err := passwordmailqueue.EnqueueReset(tx, user, token, rawToken); err != nil {
			return err
		}
		c.Set("userID", user.ID)
		if err := persistAuditFromContext(tx, c, AuditUserPasswordResetRequest, "user", user.ID, "Tautan pengaturan ulang kata sandi diminta."); err != nil {
			return err
		}
		queued = true
		return nil
	})
	if err != nil {
		// Jangan sertakan email atau token dalam log. Respons tetap generik agar
		// kegagalan internal pada akun tertentu tidak menjadi oracle enumerasi.
		log.Printf("password reset request could not be queued: %v", err)
	}
	if err == nil && queued {
		passwordmailqueue.Notify()
	}
	waitForPasswordResetResponseFloor(startedAt)
	c.JSON(http.StatusAccepted, gin.H{"message": passwordResetRequestMessage})
}

func waitForPasswordResetResponseFloor(startedAt time.Time) {
	if remaining := passwordResetResponseFloor - time.Since(startedAt); remaining > 0 {
		time.Sleep(remaining)
	}
}

type ConfirmPasswordResetRequest struct {
	Token       string `json:"token" binding:"required,min=43,max=512"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

func ConfirmPasswordReset(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req ConfirmPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Tautan reset dan kata sandi baru minimal 8 karakter wajib diisi.")
		return
	}
	if len(req.NewPassword) > 72 {
		respondError(c, http.StatusBadRequest, "invalid_request", "Kata sandi maksimal 72 byte. Kurangi jumlah karakter atau simbol.")
		return
	}
	var user models.User
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Seluruh perubahan kredensial mengunci user sebelum token/sesi.
		// Lookup pertama hanya menentukan user; validitas diperiksa ulang
		// setelah lock diperoleh agar replay dan request bersamaan aman.
		var token models.PasswordResetToken
		if err := tx.Where("token_hash = ?", utils.HashToken(strings.TrimSpace(req.Token))).First(&token).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errInvalidPasswordResetToken
			}
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", token.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errInvalidPasswordResetToken
			}
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&token, "id = ?", token.ID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errInvalidPasswordResetToken
			}
			return err
		}
		now := apptime.Now()
		if token.UsedAt != nil || token.RevokedAt != nil || !token.ExpiresAt.After(now) {
			return errInvalidPasswordResetToken
		}
		if !user.IsActive || user.EmailVerifiedAt == nil {
			return errInvalidPasswordResetToken
		}
		if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.NewPassword)) == nil {
			return errPasswordReuse
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := tx.Model(&user).Update("password", string(hash)).Error; err != nil {
			return err
		}
		if err := tx.Model(&token).Update("used_at", now).Error; err != nil {
			return err
		}
		if err := passwordmailqueue.RevokeUserResetAccess(tx, user.ID, now); err != nil {
			return err
		}
		if err := tx.Model(&models.Session{}).
			Where("user_id = ? AND revoked_at IS NULL", user.ID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.OAuthToken{}).
			Where("user_id = ? AND revoked_at IS NULL", user.ID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.OAuthAuthCode{}).Error; err != nil {
			return err
		}
		if err := passwordmailqueue.EnqueuePasswordChanged(tx, user, now, c.ClientIP(), false); err != nil {
			return err
		}
		c.Set("userID", user.ID)
		return persistAuditFromContext(tx, c, AuditUserPasswordResetComplete, "user", user.ID, "Kata sandi diatur ulang; seluruh sesi dan token aplikasi lama dicabut.")
	})
	switch {
	case errors.Is(err, errInvalidPasswordResetToken):
		respondError(c, http.StatusBadRequest, "invalid_reset_token", "Tautan pengaturan ulang tidak valid, sudah digunakan, atau kedaluwarsa. Minta tautan baru.")
		return
	case errors.Is(err, errPasswordReuse):
		respondError(c, http.StatusBadRequest, "password_reuse", "Gunakan kata sandi baru yang berbeda dari kata sandi sebelumnya.")
		return
	case err != nil:
		log.Printf("password reset confirmation failed: %v", err)
		respondError(c, http.StatusInternalServerError, "server_error", "Kata sandi belum dapat diperbarui.")
		return
	}
	setSessionCookie(c, "", -1)
	passwordmailqueue.Notify()
	c.JSON(http.StatusOK, gin.H{"message": "Kata sandi berhasil diperbarui. Seluruh sesi lama telah dikeluarkan; silakan masuk kembali."})
}
