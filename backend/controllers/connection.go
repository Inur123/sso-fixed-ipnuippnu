package controllers

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"sso-backend/database"
	"sso-backend/models"
)

// ApplicationConnection adalah satu grant aplikasi yang dimiliki pengguna.
// Token mentah sengaja tidak pernah dikirim ke portal.
type ApplicationConnection struct {
	ID          string    `json:"id"`
	ClientID    string    `json:"client_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Scopes      []string  `json:"scopes"`
	ConnectedAt time.Time `json:"connected_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// GetApplicationConnections menampilkan satu koneksi per aplikasi yang saat ini
// masih memiliki grant aktif. Login berulang atau beberapa token family tidak
// boleh membuat baris aplikasi yang sama muncul berkali-kali.
func GetApplicationConnections(c *gin.Context) {
	var tokens []models.OAuthToken
	if err := database.DB.
		Where("user_id = ?", c.GetString("userID")).
		Order("created_at DESC").
		Find(&tokens).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengambil aplikasi terhubung.")
		return
	}

	clientIDs := make([]string, 0)
	clientSeen := make(map[string]bool)
	for _, token := range tokens {
		if !clientSeen[token.ClientID] {
			clientSeen[token.ClientID] = true
			clientIDs = append(clientIDs, token.ClientID)
		}
	}
	var clients []models.OAuthClient
	if len(clientIDs) > 0 {
		if err := database.DB.Where("id IN ?", clientIDs).Find(&clients).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengambil detail aplikasi.")
			return
		}
	}
	clientByID := make(map[string]models.OAuthClient, len(clients))
	for _, client := range clients {
		clientByID[client.ID] = client
	}

	now := time.Now().UTC()
	type clientState struct {
		connection ApplicationConnection
		active     bool
		scopes     map[string]struct{}
	}
	applications := make(map[string]*clientState)
	for _, token := range tokens {
		client, exists := clientByID[token.ClientID]
		if !exists {
			continue
		}
		state, exists := applications[token.ClientID]
		if !exists {
			state = &clientState{connection: ApplicationConnection{
				ID: client.ID, ClientID: client.ID, Name: client.Name,
				Description: client.Description, Scopes: strings.Fields(token.Scope),
				ConnectedAt: token.CreatedAt, LastUsedAt: token.CreatedAt,
				ExpiresAt: token.RefreshExpiresAt,
			}, scopes: make(map[string]struct{})}
			applications[token.ClientID] = state
		}
		if token.CreatedAt.Before(state.connection.ConnectedAt) {
			state.connection.ConnectedAt = token.CreatedAt
		}
		lastUsed := token.CreatedAt
		if token.LastUsedAt != nil {
			lastUsed = *token.LastUsedAt
		}
		if lastUsed.After(state.connection.LastUsedAt) {
			state.connection.LastUsedAt = lastUsed
		}
		if token.RefreshExpiresAt.After(state.connection.ExpiresAt) {
			state.connection.ExpiresAt = token.RefreshExpiresAt
		}
		if token.RevokedAt == nil && (token.ExpiresAt.After(now) || token.RefreshExpiresAt.After(now)) {
			state.active = true
			for _, scope := range strings.Fields(token.Scope) {
				state.scopes[scope] = struct{}{}
			}
		}
	}

	connections := make([]ApplicationConnection, 0, len(applications))
	for _, state := range applications {
		if state.active {
			state.connection.Scopes = state.connection.Scopes[:0]
			for scope := range state.scopes {
				state.connection.Scopes = append(state.connection.Scopes, scope)
			}
			sort.Strings(state.connection.Scopes)
			connections = append(connections, state.connection)
		}
	}
	sort.Slice(connections, func(i, j int) bool {
		return connections[i].LastUsedAt.After(connections[j].LastUsedAt)
	})
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"connections": connections})
}

func RevokeApplicationConnection(c *gin.Context) {
	now := time.Now().UTC()
	userID := c.GetString("userID")
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var token models.OAuthToken
		if err := tx.Where("client_id = ? AND user_id = ? AND revoked_at IS NULL", c.Param("id"), userID).First(&token).Error; err != nil {
			return err
		}
		return revokeClientUserGrant(tx, c.Param("id"), userID, now)
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, http.StatusNotFound, "not_found", "Koneksi aplikasi tidak ditemukan atau sudah dicabut.")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mencabut akses aplikasi.")
		return
	}
	auditFromContext(c, AuditOAuthConnectionRevoke, "oauth_connection", c.Param("id"), "Koneksi aplikasi SSO dicabut oleh pemilik akun.")
	c.JSON(http.StatusOK, gin.H{"message": "Akses aplikasi berhasil dicabut."})
}
