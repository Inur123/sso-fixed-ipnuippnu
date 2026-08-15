package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sso-backend/database"
	"sso-backend/models"
	"sso-backend/provisioning"
)

const (
	defaultAdminPageSize = 20
	maxAdminPageSize     = 100
)

func AdminGetUsers(c *gin.Context) {
	page, err := parsePositiveQueryInt(c.Query("page"), 1, 0)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_pagination", "page harus berupa angka positif.")
		return
	}
	pageSize, err := parsePositiveQueryInt(c.Query("page_size"), defaultAdminPageSize, maxAdminPageSize)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_pagination", "page_size harus antara 1 dan 100.")
		return
	}

	query := database.DB.Model(&models.User{})
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(email) LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal menghitung daftar pengguna.")
		return
	}
	var users []models.User
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengambil daftar pengguna.")
		return
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	c.JSON(http.StatusOK, gin.H{
		"users":       users,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	})
}

func parsePositiveQueryInt(raw string, fallback, maximum int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || maximum > 0 && value > maximum {
		return 0, errors.New("value must be a positive integer within range")
	}
	return value, nil
}

type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=super_admin anggota"`
}

func AdminUpdateRole(c *gin.Context) {
	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_role", "Role harus super_admin atau anggota.")
		return
	}
	if c.Param("id") == c.GetString("userID") && req.Role != models.RoleSuperAdmin {
		respondError(c, http.StatusBadRequest, "self_demotion", "Super admin tidak dapat menurunkan role akun sendiri.")
		return
	}

	var user models.User
	var previousRole string
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", c.Param("id")).Error; err != nil {
			return err
		}
		previousRole = user.Role
		if err := tx.Model(&user).Update("role", req.Role).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&models.OAuthToken{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", user.ID).Delete(&models.OAuthAuthCode{}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan.")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal memperbarui role.")
		return
	}
	user.Role = req.Role
	user.RefreshComputedFields()
	auditFromContext(c, AuditUserRoleUpdate, "user", user.ID, "Role akun diubah dari "+previousRole+" menjadi "+req.Role+".")
	c.JSON(http.StatusOK, gin.H{"message": "Role pengguna berhasil diperbarui dan token aplikasi lama telah dicabut.", "user": user})
}

type UpdateStatusRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

func AdminUpdateStatus(c *gin.Context) {
	var req UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.IsActive == nil {
		respondError(c, http.StatusBadRequest, "invalid_status", "is_active wajib berupa boolean.")
		return
	}
	if c.Param("id") == c.GetString("userID") && !*req.IsActive {
		respondError(c, http.StatusBadRequest, "self_deactivation", "Super admin tidak dapat menonaktifkan akun sendiri.")
		return
	}

	var user models.User
	var previousStatus string
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", c.Param("id")).Error; err != nil {
			return err
		}
		if user.IsActive {
			previousStatus = models.StatusActive
		} else {
			previousStatus = models.StatusInactive
		}
		if err := tx.Model(&user).Update("is_active", *req.IsActive).Error; err != nil {
			return err
		}
		eventType := provisioning.EventUnassigned
		if *req.IsActive {
			eventType = provisioning.EventAssigned
		}
		if err := provisioning.EnqueueForUser(tx, eventType, user); err != nil {
			return err
		}
		if *req.IsActive {
			return nil
		}
		now := time.Now().UTC()
		if err := tx.Model(&models.Session{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.OAuthToken{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.OAuthAuthCode{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", user.ID).Delete(&models.EmailVerificationOTP{}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan.")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal memperbarui status akun.")
		return
	}
	user.IsActive = *req.IsActive
	user.RefreshComputedFields()
	provisioning.Notify()
	message := "Akun pengguna berhasil diaktifkan."
	if !*req.IsActive {
		message = "Akun dinonaktifkan; seluruh sesi, authorization code, dan token aplikasi telah dicabut."
	}
	auditFromContext(c, AuditUserStatusUpdate, "user", user.ID, "Status akun diubah dari "+previousStatus+" menjadi "+user.Status+".")
	c.JSON(http.StatusOK, gin.H{"message": message, "user": user})
}

// AdminDeleteUser menghapus akun dan seluruh data operasional yang dimilikinya.
// Audit historis tetap dipertahankan untuk integritas keamanan, tetapi relasi
// actor menjadi anonim melalui actor_id NULL setelah akun dihapus.
func AdminDeleteUser(c *gin.Context) {
	targetID := strings.TrimSpace(c.Param("id"))
	if targetID == c.GetString("userID") {
		respondError(c, http.StatusBadRequest, "self_deletion", "Super admin tidak dapat menghapus akun sendiri.")
		return
	}

	var user models.User
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", targetID).Error; err != nil {
			return err
		}
		if err := provisioning.EnqueueForUser(tx, provisioning.EventUnassigned, user); err != nil {
			return err
		}

		var ownedClientIDs []string
		if err := tx.Unscoped().Model(&models.OAuthClient{}).
			Where("owner_id = ?", user.ID).
			Pluck("id", &ownedClientIDs).Error; err != nil {
			return err
		}

		if len(ownedClientIDs) > 0 {
			var ownedAssignments []models.OAuthClientAssignment
			if err := tx.Preload("Client").Preload("User").
				Where("client_id IN ? AND is_active = ?", ownedClientIDs, true).Find(&ownedAssignments).Error; err != nil {
				return err
			}
			for _, assignment := range ownedAssignments {
				if err := provisioning.Enqueue(tx, provisioning.EventUnassigned, assignment.Client, assignment.User); err != nil {
					return err
				}
			}
			if err := tx.Unscoped().Where("client_id IN ?", ownedClientIDs).Delete(&models.OAuthClientAssignment{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Where("client_id IN ?", ownedClientIDs).Delete(&models.OAuthAuthCode{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Where("client_id IN ?", ownedClientIDs).Delete(&models.OAuthConsent{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Where("client_id IN ?", ownedClientIDs).Delete(&models.OAuthToken{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Unscoped().Where("user_id = ?", user.ID).Delete(&models.OAuthClientAssignment{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", user.ID).Delete(&models.OAuthAuthCode{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", user.ID).Delete(&models.OAuthConsent{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", user.ID).Delete(&models.OAuthToken{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", user.ID).Delete(&models.Session{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", user.ID).Delete(&models.EmailVerificationOTP{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("owner_id = ?", user.ID).Delete(&models.OAuthClient{}).Error; err != nil {
			return err
		}

		// Pertahankan event audit tanpa identitas pribadi actor yang dihapus.
		if err := tx.Model(&models.AuditLog{}).Where("actor_id = ?", user.ID).Update("actor_id", nil).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&user).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan.")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal menghapus data pengguna.")
		return
	}

	provisioning.Notify()
	auditFromContext(c, AuditUserDelete, "user", user.ID, "Akun pengguna dan seluruh data operasional miliknya dihapus permanen.")
	c.JSON(http.StatusOK, gin.H{"message": "Pengguna, sesi, aplikasi, dan seluruh grant terkait berhasil dihapus permanen."})
}
