package controllers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"sso-backend/database"
	"sso-backend/models"
	"sso-backend/provisioning"
	"sso-backend/utils"
)

func GetMe(c *gin.Context) {
	userID := c.GetString("userID")
	user, ok := currentUser(c)
	if !ok || user.ID != userID {
		var loaded models.User
		if err := database.DB.First(&loaded, "id = ?", userID).Error; err != nil {
			respondError(c, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan.")
			return
		}
		user = &loaded
	}
	claims, _ := c.Get("accessClaims")
	accessClaims, _ := claims.(*utils.AccessClaims)
	scope := ""
	if accessClaims != nil {
		scope = accessClaims.Scope
	}
	response := gin.H{"sub": user.ID, "id": user.ID}
	if strings.Contains(" "+scope+" ", " profile ") {
		response["name"] = user.Name
		response["phone"] = user.Phone
		response["bio"] = user.Bio
		response["gender"] = user.Gender
		response["avatar"] = user.Avatar
	}
	if strings.Contains(" "+scope+" ", " email ") {
		response["email"] = user.Email
		response["email_verified"] = user.EmailVerifiedAt != nil
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, response)
}

type UpdateProfileRequest struct {
	Name   string `json:"name" binding:"required,min=2,max=120"`
	Phone  string `json:"phone" binding:"max=30"`
	Bio    string `json:"bio" binding:"max=500"`
	Gender string `json:"gender" binding:"omitempty,oneof=male female other"`
}

func UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Data profil tidak valid.")
		return
	}
	user, ok := currentUser(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Login diperlukan.")
		return
	}
	updates := map[string]interface{}{
		"name": strings.TrimSpace(req.Name), "phone": strings.TrimSpace(req.Phone),
		"bio": strings.TrimSpace(req.Bio), "gender": req.Gender,
	}
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(user).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.First(user, "id = ?", user.ID).Error; err != nil {
			return err
		}
		return provisioning.EnqueueForUser(tx, provisioning.EventUpdated, *user)
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal memperbarui profil.")
		return
	}
	provisioning.Notify()
	auditFromContext(c, AuditUserProfileUpdate, "user", user.ID, "Profil akun diperbarui.")
	c.JSON(http.StatusOK, gin.H{"message": "Profil berhasil diperbarui.", "user": user})
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=72"`
}

func ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Kata sandi baru minimal 8 karakter.")
		return
	}
	user, _ := currentUser(c)
	if user == nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)) != nil {
		respondError(c, http.StatusUnauthorized, "invalid_credentials", "Kata sandi saat ini salah.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengamankan kata sandi.")
		return
	}
	session, _ := c.Get("session")
	current, _ := session.(*models.Session)
	now := time.Now().UTC()
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(user).Update("password", string(hash)).Error; err != nil {
			return err
		}
		query := tx.Model(&models.Session{}).Where("user_id = ? AND revoked_at IS NULL", user.ID)
		if current != nil {
			query = query.Where("id <> ?", current.ID)
		}
		if err := query.Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.OAuthToken{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", user.ID).Delete(&models.OAuthAuthCode{}).Error
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengubah kata sandi.")
		return
	}
	auditFromContext(c, AuditUserPasswordUpdate, "user", user.ID, "Kredensial akun diperbarui; sesi lain dan akses aplikasi lama dicabut.")
	c.JSON(http.StatusOK, gin.H{"message": "Kata sandi diperbarui; sesi lain dan token aplikasi telah dicabut."})
}

var allowedAvatarTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

const maxAvatarSize = 2 << 20 // 2 MiB

func UploadAvatar(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Login diperlukan.")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAvatarSize+512)
	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "File avatar wajib dikirim (maksimal 2 MB).")
		return
	}
	defer file.Close()

	if header.Size > maxAvatarSize {
		respondError(c, http.StatusRequestEntityTooLarge, "file_too_large", "Ukuran file melebihi batas 2 MB.")
		return
	}

	contentType := header.Header.Get("Content-Type")
	if !allowedAvatarTypes[contentType] {
		respondError(c, http.StatusBadRequest, "invalid_file_type", "Format file harus JPEG, PNG, atau WebP.")
		return
	}

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		switch contentType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		}
	}
	key := fmt.Sprintf("avatars/%s/%d%s", user.ID, time.Now().UnixMilli(), ext)

	publicURL, err := utils.UploadToR2(c.Request.Context(), key, contentType, file)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "upload_failed", "Gagal mengunggah avatar.")
		return
	}

	// Hapus avatar lama dari R2 jika ada.
	if oldKey := utils.R2KeyFromPublicURL(user.Avatar); oldKey != "" {
		_ = utils.DeleteFromR2(c.Request.Context(), oldKey)
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(user).Update("avatar", publicURL).Error; err != nil {
			return err
		}
		if err := tx.First(user, "id = ?", user.ID).Error; err != nil {
			return err
		}
		return provisioning.EnqueueForUser(tx, provisioning.EventUpdated, *user)
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal menyimpan avatar.")
		return
	}
	provisioning.Notify()
	auditFromContext(c, AuditUserProfileUpdate, "user", user.ID, "Avatar akun diperbarui.")
	c.JSON(http.StatusOK, gin.H{"message": "Avatar berhasil diperbarui.", "user": user})
}

func DeleteAvatar(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Login diperlukan.")
		return
	}

	if user.Avatar == "" {
		respondError(c, http.StatusBadRequest, "no_avatar", "Belum ada avatar yang diunggah.")
		return
	}

	if oldKey := utils.R2KeyFromPublicURL(user.Avatar); oldKey != "" {
		_ = utils.DeleteFromR2(c.Request.Context(), oldKey)
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(user).Update("avatar", "").Error; err != nil {
			return err
		}
		if err := tx.First(user, "id = ?", user.ID).Error; err != nil {
			return err
		}
		return provisioning.EnqueueForUser(tx, provisioning.EventUpdated, *user)
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal menghapus avatar.")
		return
	}
	provisioning.Notify()
	auditFromContext(c, AuditUserProfileUpdate, "user", user.ID, "Avatar akun dihapus.")
	c.JSON(http.StatusOK, gin.H{"message": "Avatar berhasil dihapus.", "user": user})
}
