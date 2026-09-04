package controllers

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sso-backend/backup"
	"sso-backend/database"
	"sso-backend/internal/apptime"
	"sso-backend/models"
)

type BackupController struct{ Service *backup.Service }

func (b BackupController) Status(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	status, err := b.Service.Status(c.Request.Context())
	if err != nil {
		respondError(c, 503, "backup_unavailable", "Status backup belum dapat dimuat.")
		return
	}
	c.JSON(http.StatusOK, status)
}

// Backups contain all identity data. Require the exact frontend origin, JSON,
// and a fresh password check in addition to session authentication and RBAC.
func reauthenticateBackup(c *gin.Context) (*models.User, bool) {
	c.Header("Cache-Control", "no-store")
	origin := strings.TrimRight(strings.TrimSpace(os.Getenv("FRONTEND_PUBLIC_URL")), "/")
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if origin == "" || c.GetHeader("Origin") != origin || err != nil || mediaType != "application/json" {
		respondError(c, 403, "invalid_origin", "Permintaan harus berasal dari portal identitas.")
		return nil, false
	}
	var input struct {
		Password string `json:"password"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2048)
	if c.ShouldBindJSON(&input) != nil || len(input.Password) == 0 || len(input.Password) > 72 {
		respondError(c, 400, "invalid_password", "Masukkan kata sandi akun Anda.")
		return nil, false
	}
	user, ok := currentUser(c)
	if !ok {
		respondError(c, 401, "unauthorized", "Login diperlukan.")
		return nil, false
	}
	var fresh models.User
	db := database.DB.WithContext(c.Request.Context())
	if err := db.First(&fresh, "id = ?", user.ID).Error; err != nil || fresh.Role != models.RoleSuperAdmin || !fresh.IsActive || fresh.EmailVerifiedAt == nil {
		respondError(c, 403, "forbidden", "Akses super admin aktif diperlukan.")
		return nil, false
	}
	if bcrypt.CompareHashAndPassword([]byte(fresh.Password), []byte(input.Password)) != nil {
		respondError(c, 403, "password_mismatch", "Kata sandi tidak sesuai.")
		return nil, false
	}
	// Recheck both session and current password after the expensive hash check.
	if !backupSessionValid(c, &fresh) {
		return nil, false
	}
	return &fresh, true
}

func backupSessionValid(c *gin.Context, fresh *models.User) bool {
	value, _ := c.Get("session")
	session, ok := value.(*models.Session)
	if !ok {
		respondError(c, 401, "invalid_session", "Silakan login kembali.")
		return false
	}
	var count int64
	// The password hash in this guard must not be expanded into SQL error logs.
	err := database.DB.WithContext(c.Request.Context()).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Model(&models.Session{}).Joins("JOIN users ON users.id = sessions.user_id").
		Where("sessions.id = ? AND sessions.user_id = ? AND sessions.revoked_at IS NULL AND sessions.expires_at > ?", session.ID, fresh.ID, apptime.Now()).
		Where("users.password = ? AND users.role = ? AND users.is_active = true AND users.email_verified_at IS NOT NULL AND users.deleted_at IS NULL", fresh.Password, models.RoleSuperAdmin).Count(&count).Error
	if err != nil || count != 1 {
		respondError(c, 401, "invalid_session", "Sesi atau akun telah berubah. Silakan login kembali.")
		return false
	}
	return true
}

func (b BackupController) Request(c *gin.Context) {
	user, ok := reauthenticateBackup(c)
	if !ok {
		return
	}
	job, err := b.Service.Request(c.Request.Context(), user.ID, c.ClientIP())
	if errors.Is(err, backup.ErrBusy) {
		respondError(c, 409, "backup_busy", "Backup lain masih menunggu atau berjalan.")
		return
	}
	if err != nil {
		respondError(c, 503, "backup_unavailable", "Backup belum dapat dimulai. Periksa status konfigurasi di halaman ini.")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"backup": job, "message": "Backup masuk antrean."})
}

var backupIDPattern = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)

func (b BackupController) Download(c *gin.Context) {
	user, ok := reauthenticateBackup(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if !backupIDPattern.MatchString(id) {
		respondError(c, 400, "invalid_backup", "ID backup tidak valid.")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
	defer cancel()
	// Conversion can exceed the ordinary API deadline; extend before preparing SQL.
	_ = http.NewResponseController(c.Writer).SetWriteDeadline(apptime.Now().Add(10 * time.Minute))
	_, object, err := b.Service.DownloadSQL(ctx, id)
	if errors.Is(err, backup.ErrBusy) {
		respondError(c, 409, "backup_download_busy", "Unduhan SQL lain masih diproses. Coba lagi setelah selesai.")
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, 404, "backup_not_found", "Backup tidak tersedia atau telah dihapus oleh retensi.")
		return
	}
	if err != nil {
		respondError(c, 503, "backup_unavailable", "File SQL belum dapat disiapkan. Periksa konfigurasi unduhan, integritas backup, batas ukuran, dan pg_restore.")
		return
	}
	defer object.Body.Close()
	if object.Size <= 0 {
		respondError(c, 503, "invalid_backup", "Ukuran file backup tidak cocok dengan catatan integritas.")
		return
	}
	if !backupSessionValid(c, user) {
		return
	}
	if err := persistAuditFromContext(database.DB.WithContext(ctx), c, backup.ActionDownloaded, "database_backup", id, "Unduhan SQL PostgreSQL diotorisasi setelah verifikasi kata sandi; file unduhan tidak terenkripsi."); err != nil {
		respondError(c, 503, "audit_unavailable", "Unduhan ditunda karena pencatatan audit belum tersedia.")
		return
	}
	c.Header("Content-Type", "application/sql; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="backup-`+id+`.sql"`)
	c.Header("Content-Length", strconv.FormatInt(object.Size, 10))
	// A proxy may compress transport bytes. Expose the original SQL size for
	// validating the browser's decoded Blob independently of Content-Length.
	c.Header("X-Backup-SQL-Size", strconv.FormatInt(object.Size, 10))
	c.Header("Access-Control-Expose-Headers", "X-Backup-SQL-Size, Content-Disposition")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(http.StatusOK)
	if n, err := io.CopyN(c.Writer, object.Body, object.Size); err != nil || n != object.Size {
		// Close a partial download instead of reporting a successful short file.
		c.Abort()
		return
	}
}
