package controllers

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

// GetApplicationConnections menampilkan aplikasi yang saat ini masih memiliki
// refresh grant aktif. Seluruh rotasi token dalam family yang sama ditampilkan
// sebagai satu koneksi aplikasi.
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
	type familyState struct {
		connection ApplicationConnection
		active     bool
	}
	families := make(map[string]*familyState)
	for _, token := range tokens {
		client, exists := clientByID[token.ClientID]
		if !exists {
			continue
		}
		state, exists := families[token.FamilyID]
		if !exists {
			state = &familyState{connection: ApplicationConnection{
				ID: token.FamilyID, ClientID: client.ID, Name: client.Name,
				Description: client.Description, Scopes: strings.Fields(token.Scope),
				ConnectedAt: token.CreatedAt, LastUsedAt: token.CreatedAt,
				ExpiresAt: token.RefreshExpiresAt,
			}}
			families[token.FamilyID] = state
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
			state.connection.Scopes = strings.Fields(token.Scope)
		}
	}

	connections := make([]ApplicationConnection, 0, len(families))
	for _, state := range families {
		if state.active {
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
	result := database.DB.Model(&models.OAuthToken{}).
		Where("family_id = ? AND user_id = ? AND revoked_at IS NULL", c.Param("id"), c.GetString("userID")).
		Update("revoked_at", now)
	if result.Error != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mencabut akses aplikasi.")
		return
	}
	if result.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "not_found", "Koneksi aplikasi tidak ditemukan atau sudah dicabut.")
		return
	}
	auditFromContext(c, AuditOAuthConnectionRevoke, "oauth_connection", c.Param("id"), "Koneksi aplikasi SSO dicabut oleh pemilik akun.")
	c.JSON(http.StatusOK, gin.H{"message": "Akses aplikasi berhasil dicabut."})
}
