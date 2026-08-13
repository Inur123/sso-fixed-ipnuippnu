package controllers

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sso-backend/database"
	"sso-backend/models"
)

func GetClientAssignments(c *gin.Context) {
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
	var client models.OAuthClient
	if err := findClientForManagement(database.DB, c, c.Param("id"), &client); err != nil {
		respondClientManagementError(c, err)
		return
	}

	query := database.DB.Model(&models.OAuthClientAssignment{}).
		Where("client_id = ? AND is_active = ?", client.ID, true)
	assignmentTable := database.DB.NamingStrategy.TableName("OAuthClientAssignment")
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Joins("JOIN users ON users.id = "+assignmentTable+".user_id").
			Where("LOWER(users.name) LIKE ? OR LOWER(users.email) LIKE ? OR users.id::text = ?", like, like, search)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal menghitung pengguna aplikasi.")
		return
	}
	var assignments []models.OAuthClientAssignment
	if err := query.Preload("User").Order(assignmentTable + ".created_at ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&assignments).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengambil pengguna aplikasi.")
		return
	}
	items := make([]ClientAssignmentResponse, 0, len(assignments))
	for _, assignment := range assignments {
		items = append(items, clientAssignmentResponse(assignment))
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"client": clientResponseWithAssignmentCount(database.DB, client, ""), "assignments": items,
		"total": total, "page": page, "page_size": pageSize, "total_pages": totalPages,
	})
}

// LookupClientAssignmentUser menampilkan satu hasil exact tanpa membuka
// direktori pengguna atau melakukan perubahan assignment.
func LookupClientAssignmentUser(c *gin.Context) {
	var req AssignClientUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "ID pengguna atau email wajib diisi.")
		return
	}
	identifier := strings.TrimSpace(req.Identifier)
	if !validAssignmentIdentifier(identifier) {
		respondError(c, http.StatusBadRequest, "invalid_identifier", "Masukkan UUID lengkap atau format email lengkap.")
		return
	}

	var client models.OAuthClient
	if err := findClientForManagement(database.DB, c, c.Param("id"), &client); err != nil {
		respondClientManagementError(c, err)
		return
	}
	user, err := findAssignableUser(database.DB, identifier)
	if errors.Is(err, errAssignmentUserUnavailable) {
		respondError(c, http.StatusConflict, "user_unavailable", "Pengguna harus aktif dan emailnya terverifikasi.")
		return
	}
	if err != nil {
		respondClientManagementError(c, err)
		return
	}

	var assigned int64
	if err := database.DB.Model(&models.OAuthClientAssignment{}).
		Where("client_id = ? AND user_id = ? AND is_active = ?", client.ID, user.ID, true).
		Count(&assigned).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal memeriksa akses pengguna.")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"user":     assignableUserResponse(user),
		"assigned": assigned > 0,
	})
}

// AssignClientUser menerima pencocokan exact berdasarkan UUID atau email.
// Tidak ada endpoint pencarian fuzzy agar daftar akun organisasi tidak dapat
// dienumerasi oleh pemilik aplikasi.
func AssignClientUser(c *gin.Context) {
	var req AssignClientUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "ID pengguna atau email wajib diisi.")
		return
	}
	identifier := strings.TrimSpace(req.Identifier)
	if !validAssignmentIdentifier(identifier) {
		respondError(c, http.StatusBadRequest, "invalid_identifier", "Masukkan UUID lengkap atau format email lengkap.")
		return
	}

	var assignment models.OAuthClientAssignment
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var client models.OAuthClient
		if err := findClientForManagement(tx.Clauses(clause.Locking{Strength: "UPDATE"}), c, c.Param("id"), &client); err != nil {
			return err
		}
		user, err := findAssignableUser(tx, identifier)
		if err != nil {
			return err
		}
		result := tx.Unscoped().Where("client_id = ? AND user_id = ?", client.ID, user.ID).First(&assignment)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			assignment = models.OAuthClientAssignment{ClientID: client.ID, UserID: user.ID, IsActive: true}
		} else if result.Error != nil {
			return result.Error
		}
		assignment.IsActive = true
		assignment.DeletedAt = gorm.DeletedAt{}
		if assignment.ID == "" {
			if err := tx.Create(&assignment).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Unscoped().Model(&assignment).Updates(map[string]interface{}{
				"is_active": true, "deleted_at": nil,
			}).Error; err != nil {
				return err
			}
		}
		if err := revokeClientUserGrant(tx, client.ID, user.ID, time.Now().UTC()); err != nil {
			return err
		}
		assignment.User = user
		return nil
	})
	if errors.Is(err, errAssignmentUserUnavailable) {
		respondError(c, http.StatusConflict, "user_unavailable", "Pengguna harus aktif dan emailnya terverifikasi.")
		return
	}
	if err != nil {
		respondClientManagementError(c, err)
		return
	}
	auditFromContext(c, AuditOAuthClientAssignmentUpdate, "oauth_client", c.Param("id"), "Akses pengguna ke aplikasi ditambahkan.")
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"message": "Akses pengguna berhasil ditambahkan.", "assignment": clientAssignmentResponse(assignment)})
}

func DeleteClientAssignment(c *gin.Context) {
	var deleted models.OAuthClientAssignment
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var client models.OAuthClient
		if err := findClientForManagement(tx.Clauses(clause.Locking{Strength: "UPDATE"}), c, c.Param("id"), &client); err != nil {
			return err
		}
		if client.AccessPolicy == models.AccessPolicyAssignedOnly && client.OwnerID == c.Param("userId") {
			return errOwnerAssignmentRequired
		}
		if err := tx.Where("client_id = ? AND user_id = ?", client.ID, c.Param("userId")).First(&deleted).Error; err != nil {
			return err
		}
		if err := tx.Delete(&deleted).Error; err != nil {
			return err
		}
		return revokeClientUserGrant(tx, client.ID, deleted.UserID, time.Now().UTC())
	})
	if errors.Is(err, errOwnerAssignmentRequired) {
		respondError(c, http.StatusConflict, "owner_assignment_required", "Pemilik harus tetap memiliki akses saat policy assigned_only digunakan.")
		return
	}
	if err != nil {
		respondClientManagementError(c, err)
		return
	}
	auditFromContext(c, AuditOAuthClientAssignmentDelete, "oauth_client", c.Param("id"), "Assignment pengguna aplikasi dihapus dan grant dicabut.")
	c.JSON(http.StatusOK, gin.H{"message": "Akses pengguna dan seluruh grant aplikasi berhasil dicabut."})
}

var (
	errAssignmentUserUnavailable = errors.New("assignment user unavailable")
	errOwnerAssignmentRequired   = errors.New("owner assignment required")
	assignmentUUIDPattern        = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	assignmentEmailPattern       = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]{2,}$`)
)

func validAssignmentIdentifier(identifier string) bool {
	return identifier != "" && len(identifier) <= 254 &&
		(assignmentUUIDPattern.MatchString(identifier) || assignmentEmailPattern.MatchString(identifier))
}

func clientAssignmentResponse(assignment models.OAuthClientAssignment) ClientAssignmentResponse {
	return ClientAssignmentResponse{ID: assignment.ID, UserID: assignment.UserID, Name: assignment.User.Name,
		Email: assignment.User.Email, Avatar: assignment.User.Avatar}
}

func assignableUserResponse(user models.User) AssignableUserResponse {
	return AssignableUserResponse{UserID: user.ID, Name: user.Name, Email: user.Email, Avatar: user.Avatar}
}

func findAssignableUser(db *gorm.DB, identifier string) (models.User, error) {
	var user models.User
	if err := db.Where("id::text = ? OR LOWER(email) = ?", identifier, strings.ToLower(identifier)).First(&user).Error; err != nil {
		return models.User{}, err
	}
	if !user.IsActive || user.EmailVerifiedAt == nil {
		return models.User{}, errAssignmentUserUnavailable
	}
	return user, nil
}

func respondClientManagementError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, http.StatusNotFound, "not_found", "Aplikasi atau pengguna tidak ditemukan.")
		return
	}
	respondError(c, http.StatusInternalServerError, "server_error", "Permintaan pengelolaan akses aplikasi gagal diproses.")
}
