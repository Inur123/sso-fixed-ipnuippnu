package controllers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"sso-backend/database"
	"sso-backend/models"
)

const (
	AuditUserRegister                = "user.register"
	AuditEmailVerify                 = "email.verify"
	AuditAuthLogin                   = "auth.login"
	AuditAuthLoginFailed             = "auth.login_failed"
	AuditAuthLogout                  = "auth.logout"
	AuditUserStatusUpdate            = "user.status_update"
	AuditUserRoleUpdate              = "user.role_update"
	AuditUserDelete                  = "user.delete"
	AuditUserProfileUpdate           = "user.profile_update"
	AuditUserPasswordUpdate          = "user.password_update"
	AuditOAuthClientCreate           = "oauth.client_create"
	AuditOAuthClientUpdate           = "oauth.client_update"
	AuditOAuthClientDelete           = "oauth.client_delete"
	AuditOAuthClientAssignmentUpdate = "oauth.client_assignment_update"
	AuditOAuthClientAssignmentDelete = "oauth.client_assignment_delete"
	AuditOAuthClientSecretView       = "oauth.client_secret_view"
	AuditOAuthClientSecretRegenerate = "oauth.client_secret_regenerate"
	AuditOAuthConsent                = "oauth.consent"
	AuditOAuthGrant                  = "oauth.grant"
	AuditOAuthTokenRevoke            = "oauth.token_revoke"
	AuditOAuthConnectionRevoke       = "oauth.connection_revoke"

	auditQueueCapacity  = 256
	maxAuditSearchRunes = 100
)

var knownAuditActions = map[string]struct{}{
	AuditUserRegister: {}, AuditEmailVerify: {}, AuditAuthLogin: {}, AuditAuthLoginFailed: {}, AuditAuthLogout: {},
	AuditUserStatusUpdate: {}, AuditUserRoleUpdate: {}, AuditUserDelete: {}, AuditUserProfileUpdate: {}, AuditUserPasswordUpdate: {},
	AuditOAuthClientCreate: {}, AuditOAuthClientUpdate: {}, AuditOAuthClientDelete: {}, AuditOAuthClientAssignmentUpdate: {}, AuditOAuthClientAssignmentDelete: {}, AuditOAuthClientSecretView: {}, AuditOAuthClientSecretRegenerate: {}, AuditOAuthConsent: {}, AuditOAuthGrant: {},
	AuditOAuthTokenRevoke: {}, AuditOAuthConnectionRevoke: {},
}

var (
	auditQueue      = make(chan models.AuditLog, auditQueueCapacity)
	auditWorkerOnce sync.Once
)

// auditFromContext memasukkan peristiwa ke antrean bounded tanpa menahan
// response utama. Hanya field eksplisit yang diterima; body/header otorisasi
// request tidak pernah disalin ke audit log.
func auditFromContext(c *gin.Context, action, targetType, targetID, description string) {
	actorID := auditActorFromContext(c)
	auditWithActor(c, actorID, action, targetType, targetID, description)
}

// persistAuditFromContext menyimpan event secara sinkron melalui handle DB yang
// diberikan. Handler untuk aksi kredensial memakai fungsi ini agar plaintext
// tidak pernah dikirim jika jejak audit gagal disimpan. Jika db adalah sebuah
// transaksi, event menjadi atomik dengan perubahan yang sedang dilakukan.
func persistAuditFromContext(db *gorm.DB, c *gin.Context, action, targetType, targetID, description string) error {
	if db == nil {
		return errors.New("audit storage unavailable")
	}
	event, ok := newAuditEvent(auditActorFromContext(c), action, targetType, targetID, description, auditIPAddressFromContext(c))
	if !ok {
		return errors.New("invalid audit event")
	}
	return db.Create(&event).Error
}

func auditActorFromContext(c *gin.Context) *string {
	if c == nil {
		return nil
	}
	if value := strings.TrimSpace(c.GetString("userID")); value != "" {
		return &value
	}
	return nil
}

func auditIPAddressFromContext(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return c.ClientIP()
}

func auditWithActor(c *gin.Context, actorID *string, action, targetType, targetID, description string) {
	event, ok := newAuditEvent(actorID, action, targetType, targetID, description, auditIPAddressFromContext(c))
	if !ok {
		return
	}
	startAuditWorker()
	select {
	case auditQueue <- event:
	default:
		// Jangan menulis description atau data request ke log proses.
		log.Printf("audit queue full; event dropped action=%q", event.Action)
	}
}

func startAuditWorker() {
	auditWorkerOnce.Do(func() {
		go func() {
			for event := range auditQueue {
				if database.DB == nil {
					log.Printf("audit storage unavailable; event dropped action=%q", event.Action)
					continue
				}
				if err := database.DB.Create(&event).Error; err != nil {
					log.Printf("audit write failed action=%q: %v", event.Action, err)
				}
			}
		}()
	})
}

func newAuditEvent(actorID *string, action, targetType, targetID, description, ipAddress string) (models.AuditLog, bool) {
	action = strings.TrimSpace(action)
	if _, ok := knownAuditActions[action]; !ok {
		return models.AuditLog{}, false
	}
	if actorID != nil {
		trimmed := truncateRunes(strings.TrimSpace(*actorID), 128)
		if trimmed == "" {
			actorID = nil
		} else {
			actorID = &trimmed
		}
	}
	return models.AuditLog{
		ActorID:     actorID,
		Action:      action,
		TargetType:  truncateRunes(strings.TrimSpace(targetType), 80),
		TargetID:    truncateRunes(strings.TrimSpace(targetID), 128),
		Description: safeAuditDescription(description),
		IPAddress:   truncateRunes(strings.TrimSpace(ipAddress), 64),
	}, true
}

func safeAuditDescription(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	lower := strings.ToLower(value)
	// Defense-in-depth jika kelak ada caller baru yang tanpa sengaja mencoba
	// menulis nilai kredensial ke description.
	for _, marker := range []string{
		"password=", "password:", "kata_sandi=", "kata sandi:",
		"otp=", "otp:", "token=", "token:", "secret=", "secret:",
		"client_secret", "code_verifier", "authorization: bearer",
	} {
		if strings.Contains(lower, marker) {
			return "Detail sensitif disembunyikan."
		}
	}
	if value == "" {
		return "Peristiwa keamanan dicatat."
	}
	return truncateRunes(value, 500)
}

func truncateRunes(value string, maximum int) string {
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	runes := []rune(value)
	return string(runes[:maximum])
}

type AuditLogResponse struct {
	ID          string    `json:"id"`
	ActorID     *string   `json:"actor_id"`
	ActorName   string    `json:"actor_name"`
	ActorEmail  string    `json:"actor_email"`
	Action      string    `json:"action"`
	TargetType  string    `json:"target_type"`
	TargetID    string    `json:"target_id"`
	Description string    `json:"description"`
	IPAddress   string    `json:"ip_address"`
	CreatedAt   time.Time `json:"created_at"`
}

func AdminGetAuditLogs(c *gin.Context) {
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
	search := strings.TrimSpace(c.Query("search"))
	if utf8.RuneCountInString(search) > maxAuditSearchRunes {
		respondError(c, http.StatusBadRequest, "invalid_search", "Pencarian maksimal 100 karakter.")
		return
	}
	action := strings.TrimSpace(c.Query("action"))
	if action != "" {
		if _, ok := knownAuditActions[action]; !ok {
			respondError(c, http.StatusBadRequest, "invalid_action", "Filter action audit tidak dikenal.")
			return
		}
	}

	query := database.DB.Model(&models.AuditLog{}).
		Joins("LEFT JOIN users AS audit_actors ON audit_actors.id = audit_logs.actor_id").
		Where("audit_logs.action <> ?", AuditOAuthClientSecretView)
	if action != "" {
		query = query.Where("audit_logs.action = ?", action)
	}
	if search != "" {
		like := "%" + escapeAuditLike(strings.ToLower(search)) + "%"
		query = query.Where(`(
			LOWER(COALESCE(audit_logs.action, '')) LIKE ? ESCAPE '\' OR
			LOWER(COALESCE(audit_logs.description, '')) LIKE ? ESCAPE '\' OR
			LOWER(COALESCE(audit_logs.target_type, '')) LIKE ? ESCAPE '\' OR
			LOWER(COALESCE(audit_logs.target_id, '')) LIKE ? ESCAPE '\' OR
			LOWER(COALESCE(audit_logs.ip_address, '')) LIKE ? ESCAPE '\' OR
			LOWER(COALESCE(audit_actors.name, '')) LIKE ? ESCAPE '\' OR
			LOWER(COALESCE(audit_actors.email, '')) LIKE ? ESCAPE '\'
		)`, like, like, like, like, like, like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal menghitung audit log.")
		return
	}
	var logs []models.AuditLog
	if err := query.Preload("Actor").
		Order("audit_logs.created_at DESC, audit_logs.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengambil audit log.")
		return
	}

	items := make([]AuditLogResponse, 0, len(logs))
	for _, item := range logs {
		response := AuditLogResponse{
			ID: item.ID, ActorID: item.ActorID, Action: item.Action,
			TargetType: item.TargetType, TargetID: item.TargetID,
			Description: item.Description, IPAddress: item.IPAddress, CreatedAt: item.CreatedAt,
		}
		if item.Actor != nil {
			response.ActorName = item.Actor.Name
			response.ActorEmail = item.Actor.Email
		}
		items = append(items, response)
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"logs": items, "total": total, "page": page,
		"page_size": pageSize, "total_pages": totalPages,
	})
}

func escapeAuditLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
