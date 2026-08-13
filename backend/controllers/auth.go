package controllers

import (
	"errors"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sso-backend/database"
	"sso-backend/models"
	"sso-backend/utils"
)

const (
	sessionLifetime         = 24 * time.Hour
	defaultEmailOTPLifetime = 10 * time.Minute
	emailOTPResendCooldown  = time.Minute
	emailOTPMaxAttempts     = 5
)

var errEmailAlreadyExists = errors.New("email already exists")

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Name     string `json:"name" binding:"required,min=2,max=120"`
}

func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Nama, email, dan kata sandi minimal 8 karakter wajib diisi.")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengamankan kata sandi.")
		return
	}

	user := models.User{
		Email:    normalizeEmail(req.Email),
		Password: string(hashedPassword),
		Name:     strings.TrimSpace(req.Name),
		Role:     models.RoleAnggota,
		IsActive: true,
	}
	otpLifetime := emailOTPLifetime()
	var otpCode, otpHash string
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.User{}).Where("LOWER(email) = ?", user.Email).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errEmailAlreadyExists
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		var prepared bool
		otpCode, otpHash, prepared, err = prepareEmailOTP(tx, &user, false, otpLifetime)
		if !prepared && err == nil {
			return errors.New("verification OTP was not prepared")
		}
		return err
	})
	if err != nil {
		switch {
		case errors.Is(err, errEmailAlreadyExists):
			respondError(c, http.StatusConflict, "email_exists", "Email sudah terdaftar.")
		default:
			// Unique constraint tetap menjadi perlindungan terakhir untuk request
			// registrasi bersamaan dengan email yang sama.
			var count int64
			if database.DB.Model(&models.User{}).Where("LOWER(email) = ?", user.Email).Count(&count).Error == nil && count > 0 {
				respondError(c, http.StatusConflict, "email_exists", "Email sudah terdaftar.")
				return
			}
			respondError(c, http.StatusInternalServerError, "server_error", "Akun belum dapat dibuat.")
		}
		return
	}

	emailSent := true
	if err := utils.SendVerificationEmail(user.Email, user.Name, otpCode, otpLifetime); err != nil {
		emailSent = false
		clearUndeliveredEmailOTP(user.ID, otpHash)
		log.Printf("registration verification email failed for %s: %v", user.Email, err)
	}
	user.RefreshComputedFields()
	message := "Akun IPNU IPPNU ID berhasil dibuat. Masukkan OTP yang dikirim ke email Anda."
	if !emailSent {
		message = "Akun berhasil dibuat, tetapi email OTP belum terkirim. Gunakan tombol kirim ulang OTP."
	}
	auditWithActor(c, &user.ID, AuditUserRegister, "user", user.ID, "Akun baru terdaftar.")
	c.JSON(http.StatusCreated, gin.H{
		"message":                 message,
		"verification_email_sent": emailSent,
		"user":                    user,
	})
}

type VerifyEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
	OTP   string `json:"otp" binding:"required,len=6,numeric"`
}

func VerifyEmail(c *gin.Context) {
	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Email dan OTP enam digit wajib diisi.")
		return
	}

	var user models.User
	var verificationError string
	var alreadyVerified bool
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("LOWER(email) = ?", normalizeEmail(req.Email)).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				verificationError = "invalid_otp"
				return nil
			}
			return err
		}
		if !user.IsActive {
			verificationError = "invalid_otp"
			return nil
		}
		if user.EmailVerifiedAt != nil {
			alreadyVerified = true
			return nil
		}

		var record models.EmailVerificationOTP
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", user.ID).First(&record).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			verificationError = "invalid_otp"
			return nil
		}
		now := time.Now().UTC()
		if !now.Before(record.ExpiresAt) {
			if err := tx.Delete(&record).Error; err != nil {
				return err
			}
			verificationError = "otp_expired"
			return nil
		}
		if record.Attempts >= emailOTPMaxAttempts || !utils.VerifyEmailOTP(user.ID, req.OTP, record.CodeHash) {
			record.Attempts++
			if record.Attempts >= emailOTPMaxAttempts {
				if err := tx.Delete(&record).Error; err != nil {
					return err
				}
			} else if err := tx.Model(&record).Update("attempts", record.Attempts).Error; err != nil {
				return err
			}
			verificationError = "invalid_otp"
			return nil
		}

		updates := map[string]interface{}{"email_verified_at": now}
		verifiedRole := roleAfterEmailVerification(user.Email, user.Role)
		if verifiedRole != user.Role {
			updates["role"] = verifiedRole
		}
		if err := tx.Model(&user).Updates(updates).Error; err != nil {
			return err
		}
		user.EmailVerifiedAt = &now
		user.Role = verifiedRole
		return tx.Delete(&record).Error
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Verifikasi email belum dapat diproses.")
		return
	}
	switch verificationError {
	case "otp_expired":
		respondError(c, http.StatusBadRequest, "otp_expired", "OTP sudah kedaluwarsa. Minta kode verifikasi baru.")
		return
	case "invalid_otp":
		respondError(c, http.StatusBadRequest, "invalid_otp", "Email atau OTP tidak valid. Periksa kode atau minta kode baru.")
		return
	}

	if alreadyVerified {
		c.JSON(http.StatusOK, gin.H{"message": "Jika email terdaftar, status verifikasinya sudah diproses."})
		return
	}
	user.RefreshComputedFields()
	auditWithActor(c, &user.ID, AuditEmailVerify, "user", user.ID, "Email akun berhasil diverifikasi.")
	if user.Role == models.RoleSuperAdmin && utils.IsConfiguredSuperAdminEmail(user.Email) {
		auditWithActor(c, &user.ID, AuditUserRoleUpdate, "user", user.ID, "Akun terverifikasi dipromosikan menjadi super admin sesuai konfigurasi server.")
	}
	c.JSON(http.StatusOK, gin.H{"message": "Email berhasil diverifikasi. Anda sekarang dapat masuk ke IPNU IPPNU ID.", "user": user})
}

type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func ResendVerification(c *gin.Context) {
	var req ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Email yang valid wajib diisi.")
		return
	}

	// Respons selalu sama untuk email yang tidak dikenal, sudah terverifikasi,
	// nonaktif, maupun sedang cooldown agar endpoint tidak menjadi enumerator akun.
	message := "Jika akun terdaftar, aktif, dan belum terverifikasi, OTP baru akan dikirim ke email tersebut."
	var user models.User
	var otpCode, otpHash string
	var shouldSend bool
	otpLifetime := emailOTPLifetime()
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("LOWER(email) = ?", normalizeEmail(req.Email)).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if !user.IsActive || user.EmailVerifiedAt != nil {
			return nil
		}
		var err error
		otpCode, otpHash, shouldSend, err = prepareEmailOTP(tx, &user, true, otpLifetime)
		return err
	})
	if err != nil {
		log.Printf("prepare resend verification email failed for %s: %v", normalizeEmail(req.Email), err)
	} else if shouldSend {
		if err := utils.SendVerificationEmail(user.Email, user.Name, otpCode, otpLifetime); err != nil {
			clearUndeliveredEmailOTP(user.ID, otpHash)
			log.Printf("resend verification email failed for %s: %v", normalizeEmail(req.Email), err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": message})
}

func prepareEmailOTP(tx *gorm.DB, user *models.User, enforceCooldown bool, lifetime time.Duration) (string, string, bool, error) {
	var existing models.EmailVerificationOTP
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", user.ID).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", false, err
	}
	now := time.Now().UTC()
	if err == nil && enforceCooldown && now.Before(existing.LastSentAt.Add(emailOTPResendCooldown)) {
		return "", "", false, nil
	}
	code, err := utils.GenerateEmailOTP()
	if err != nil {
		return "", "", false, err
	}
	hash, err := utils.HashEmailOTP(user.ID, code)
	if err != nil {
		return "", "", false, err
	}
	record := models.EmailVerificationOTP{
		UserID:     user.ID,
		CodeHash:   hash,
		Attempts:   0,
		LastSentAt: now,
		ExpiresAt:  now.Add(lifetime),
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"code_hash": hash, "attempts": 0, "last_sent_at": now, "expires_at": record.ExpiresAt,
		}),
	}).Create(&record).Error; err != nil {
		return "", "", false, err
	}
	return code, hash, true, nil
}

func emailOTPLifetime() time.Duration {
	minutes, err := strconv.Atoi(strings.TrimSpace(os.Getenv("MAIL_OTP_TTL_MINUTES")))
	if err != nil || minutes < 5 || minutes > 30 {
		return defaultEmailOTPLifetime
	}
	return time.Duration(minutes) * time.Minute
}

func clearUndeliveredEmailOTP(userID, hash string) {
	if userID == "" || hash == "" {
		return
	}
	if err := database.DB.Where("user_id = ? AND code_hash = ?", userID, hash).Delete(&models.EmailVerificationOTP{}).Error; err != nil {
		log.Printf("failed to clear undelivered email OTP for user %s: %v", userID, err)
	}
}

type LoginRequest struct {
	Email    string        `json:"email" binding:"required,email"`
	Password string        `json:"password" binding:"required"`
	Location LoginLocation `json:"location" binding:"required"`
	Device   LoginDevice   `json:"device" binding:"required"`
}

type LoginLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Accuracy  float64 `json:"accuracy"`
}

type LoginDevice struct {
	Platform string `json:"platform"`
	Language string `json:"language"`
	Timezone string `json:"timezone"`
}

func validLoginContext(req LoginRequest) bool {
	return !math.IsNaN(req.Location.Latitude) && !math.IsInf(req.Location.Latitude, 0) &&
		!math.IsNaN(req.Location.Longitude) && !math.IsInf(req.Location.Longitude, 0) &&
		!math.IsNaN(req.Location.Accuracy) && !math.IsInf(req.Location.Accuracy, 0) &&
		req.Location.Latitude >= -90 && req.Location.Latitude <= 90 &&
		req.Location.Longitude >= -180 && req.Location.Longitude <= 180 &&
		req.Location.Accuracy > 0 && req.Location.Accuracy <= 100000 &&
		strings.TrimSpace(req.Device.Platform) != "" && len(req.Device.Platform) <= 120 &&
		len(req.Device.Language) <= 40 && len(req.Device.Timezone) <= 100
}

func loginDeviceDescription(c *gin.Context, device LoginDevice) string {
	return "Platform: " + strings.TrimSpace(device.Platform) +
		"; Bahasa: " + strings.TrimSpace(device.Language) +
		"; Zona waktu: " + strings.TrimSpace(device.Timezone) +
		"; User-Agent: " + c.Request.UserAgent()
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Email, kata sandi, lokasi, dan informasi perangkat wajib diisi.")
		return
	}
	if !validLoginContext(req) {
		respondError(c, http.StatusBadRequest, "location_required", "Izin lokasi dan informasi perangkat yang valid diperlukan untuk login.")
		return
	}
	device := loginDeviceDescription(c, req.Device)

	var user models.User
	lookupErr := database.DB.Where("LOWER(email) = ?", normalizeEmail(req.Email)).First(&user).Error
	if lookupErr != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		var actorID *string
		targetID := ""
		if lookupErr == nil {
			actorID = &user.ID
			targetID = user.ID
		}
		auditLoginWithActor(c, actorID, AuditAuthLoginFailed, "user", targetID, "Percobaan login gagal.", device, req.Location.Latitude, req.Location.Longitude, req.Location.Accuracy)
		respondError(c, http.StatusUnauthorized, "invalid_credentials", "Email atau kata sandi salah.")
		return
	}
	if status, code, message, denied := accountAccessError(&user); denied {
		auditLoginWithActor(c, &user.ID, AuditAuthLoginFailed, "user", user.ID, "Login ditolak oleh kebijakan keamanan akun.", device, req.Location.Latitude, req.Location.Longitude, req.Location.Accuracy)
		respondError(c, status, code, message)
		return
	}

	rawToken, err := utils.RandomToken(32)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal membuat sesi.")
		return
	}
	now := time.Now().UTC()
	session := models.Session{
		TokenHash:  utils.HashToken(rawToken),
		UserID:     user.ID,
		IPAddress:  c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		LastSeenAt: now,
		ExpiresAt:  now.Add(sessionLifetime),
	}
	if err := database.DB.Create(&session).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal menyimpan sesi.")
		return
	}

	setSessionCookie(c, rawToken, int(sessionLifetime.Seconds()))
	auditLoginWithActor(c, &user.ID, AuditAuthLogin, "session", session.ID, "Login berhasil dan sesi baru dibuat.", device, req.Location.Latitude, req.Location.Longitude, req.Location.Accuracy)
	c.JSON(http.StatusOK, gin.H{"message": "Login IPNU IPPNU ID berhasil.", "user": user})
}

func GetSession(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	rawToken, err := c.Cookie(sessionCookieName())
	if err != nil || rawToken == "" {
		c.JSON(http.StatusOK, gin.H{"user": nil})
		return
	}

	now := time.Now().UTC()
	var session models.Session
	if err := database.DB.Preload("User").
		Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", utils.HashToken(rawToken), now).
		First(&session).Error; err != nil {
		setSessionCookie(c, "", -1)
		c.JSON(http.StatusOK, gin.H{"user": nil})
		return
	}
	if _, _, _, denied := accountAccessError(&session.User); denied {
		database.DB.Model(&session).Update("revoked_at", now)
		setSessionCookie(c, "", -1)
		c.JSON(http.StatusOK, gin.H{"user": nil})
		return
	}

	database.DB.Model(&session).Update("last_seen_at", now)
	user := &session.User
	user.RefreshComputedFields()
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func Logout(c *gin.Context) {
	var actorID *string
	if rawToken, err := c.Cookie(sessionCookieName()); err == nil {
		var session models.Session
		if err := database.DB.Select("user_id").Where("token_hash = ?", utils.HashToken(rawToken)).First(&session).Error; err == nil && session.UserID != "" {
			actorID = &session.UserID
		}
		now := time.Now().UTC()
		database.DB.Model(&models.Session{}).
			Where("token_hash = ? AND revoked_at IS NULL", utils.HashToken(rawToken)).
			Update("revoked_at", now)
	}
	setSessionCookie(c, "", -1)
	if actorID != nil {
		auditWithActor(c, actorID, AuditAuthLogout, "user", *actorID, "Logout berhasil dan sesi dicabut.")
	}
	c.JSON(http.StatusOK, gin.H{"message": "Logout berhasil."})
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func roleAfterEmailVerification(email, currentRole string) string {
	if utils.IsConfiguredSuperAdminEmail(email) {
		return models.RoleSuperAdmin
	}
	return currentRole
}

func setSessionCookie(c *gin.Context, value string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	secure := strings.EqualFold(os.Getenv("APP_ENV"), "production")
	c.SetCookie(sessionCookieName(), value, maxAge, "/", strings.TrimSpace(os.Getenv("SESSION_COOKIE_DOMAIN")), secure, true)
}

func sessionCookieName() string {
	return strings.TrimSpace(os.Getenv("SESSION_COOKIE_NAME"))
}
