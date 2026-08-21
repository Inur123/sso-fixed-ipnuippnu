package controllers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sso-backend/database"
	"sso-backend/models"
	"sso-backend/utils"
)

const (
	promptNone          = "none"
	promptConsent       = "consent"
	promptSelectAccount = "select_account"
)

func parseOAuthPrompt(value string) (map[string]bool, error) {
	result := make(map[string]bool)
	for _, item := range strings.Fields(value) {
		switch item {
		case promptNone, promptConsent, promptSelectAccount:
			result[item] = true
		default:
			return nil, errors.New("unsupported prompt")
		}
	}
	if result[promptNone] && len(result) > 1 {
		return nil, errors.New("prompt none cannot be combined")
	}
	return result, nil
}

func consentCovers(consent models.OAuthConsent, requestedScope string) bool {
	return consent.ID != "" && consent.RevokedAt == nil && utils.ScopeAllowed(requestedScope, consent.Scope)
}

// Pemilihan akun hanya boleh menerbitkan code dari consent yang masih aktif.
// Consent baru atau scope tambahan harus dikonfirmasi secara eksplisit oleh UI.
func missingRequiredConsentApproval(required, approved bool) bool {
	return required && !approved
}

// Consent adalah grant persisten, bukan bagian dari sesi login SSO. Karena itu,
// prompt=consent dari RP tidak boleh membatalkan grant aktif yang masih mencakup
// seluruh scope. Pengguna akan diminta menyetujui lagi hanya jika grant belum ada,
// telah dicabut, atau scope yang diminta bertambah.
func consentRequired(db *gorm.DB, clientID, userID, requestedScope string) (bool, error) {
	var consent models.OAuthConsent
	err := db.Where("client_id = ? AND user_id = ?", clientID, userID).First(&consent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !consentCovers(consent, requestedScope), nil
}

func persistOAuthConsent(tx *gorm.DB, clientID, userID, requestedScope string, now time.Time) error {
	var consent models.OAuthConsent
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("client_id = ? AND user_id = ?", clientID, userID).
		First(&consent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		consent = models.OAuthConsent{ClientID: clientID, UserID: userID, Scope: requestedScope, GrantedAt: now}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "client_id"}, {Name: "user_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"scope": requestedScope, "granted_at": now, "revoked_at": nil, "updated_at": now,
			}),
		}).Create(&consent).Error
	}
	if err != nil {
		return err
	}
	mergedScope := utils.NormalizeScope(consent.Scope + " " + requestedScope)
	return tx.Model(&consent).Updates(map[string]any{
		"scope":      mergedScope,
		"granted_at": now,
		"revoked_at": nil,
	}).Error
}

func revokeOAuthConsent(tx *gorm.DB, clientID, userID string, now time.Time) error {
	return tx.Model(&models.OAuthConsent{}).
		Where("client_id = ? AND user_id = ? AND revoked_at IS NULL", clientID, userID).
		Update("revoked_at", now).Error
}

// GetOAuthAuthorizationContext menggabungkan identitas aplikasi, admission,
// dan status consent untuk UI authorization yang sudah memiliki sesi SSO.
func GetOAuthAuthorizationContext(c *gin.Context) {
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	scope := utils.NormalizeScope(c.Query("scope"))
	prompts, err := parseOAuthPrompt(c.Query("prompt"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Nilai prompt OAuth tidak didukung.")
		return
	}

	var client models.OAuthClient
	if err := database.DB.First(&client, "id = ? AND status = ?", clientID, models.ClientStatusActive).Error; err != nil || !exactRedirectMatch(client, redirectURI) {
		respondError(c, http.StatusBadRequest, "invalid_request", "Client atau redirect URI tidak valid.")
		return
	}
	if !utils.ScopeAllowed(scope, client.AllowedScopes) {
		respondError(c, http.StatusBadRequest, "invalid_scope", "Scope tidak diizinkan untuk aplikasi ini.")
		return
	}
	if _, err := applicationAccess(database.DB, client, c.GetString("userID"), time.Now().UTC()); err != nil {
		if errors.Is(err, errApplicationAccessDenied) {
			respondError(c, http.StatusForbidden, "access_denied", "Akun Anda belum diberi akses ke aplikasi ini.")
			return
		}
		respondError(c, http.StatusInternalServerError, "server_error", "Kebijakan akses aplikasi belum dapat diperiksa.")
		return
	}
	required, err := consentRequired(database.DB, client.ID, c.GetString("userID"), scope)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Status persetujuan aplikasi belum dapat diperiksa.")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"client_id": client.ID, "name": client.Name, "description": client.Description,
		"allowed_scopes":   strings.Fields(client.AllowedScopes),
		"consent_required": required,
		"select_account":   prompts[promptSelectAccount],
	})
}
