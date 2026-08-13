package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"sso-backend/database"
	"sso-backend/models"
	"sso-backend/utils"
)

func RequireSession(c *gin.Context) {
	rawToken, err := c.Cookie(sessionCookieName())
	if err != nil || rawToken == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized", "Login diperlukan.")
		c.Abort()
		return
	}

	var session models.Session
	now := time.Now().UTC()
	if err := database.DB.Preload("User").
		Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", utils.HashToken(rawToken), now).
		First(&session).Error; err != nil {
		setSessionCookie(c, "", -1)
		respondError(c, http.StatusUnauthorized, "invalid_session", "Sesi sudah berakhir atau dicabut.")
		c.Abort()
		return
	}
	if status, code, message, denied := accountAccessError(&session.User); denied {
		database.DB.Model(&session).Update("revoked_at", now)
		setSessionCookie(c, "", -1)
		respondError(c, status, code, message)
		c.Abort()
		return
	}

	database.DB.Model(&session).Update("last_seen_at", now)
	c.Set("session", &session)
	c.Set("user", &session.User)
	c.Set("userID", session.UserID)
	c.Next()
}

func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(c *gin.Context) {
		user, ok := currentUser(c)
		if !ok {
			respondError(c, http.StatusUnauthorized, "unauthorized", "Login diperlukan.")
			c.Abort()
			return
		}
		if _, ok := allowed[user.Role]; !ok {
			respondError(c, http.StatusForbidden, "forbidden", "Anda tidak memiliki izin untuk tindakan ini.")
			c.Abort()
			return
		}
		c.Next()
	}
}

func RequireAuthToken(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	parts := strings.Fields(authHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		respondError(c, http.StatusUnauthorized, "invalid_token", "Gunakan header Authorization: Bearer {token}.")
		c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
		c.Abort()
		return
	}

	claims, err := utils.ValidateAccessToken(parts[1])
	if err != nil {
		respondError(c, http.StatusUnauthorized, "invalid_token", "Access token tidak valid atau kedaluwarsa.")
		c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
		c.Abort()
		return
	}

	var token models.OAuthToken
	if err := database.DB.Where("access_jti = ? AND client_id = ? AND user_id = ? AND revoked_at IS NULL AND expires_at > ?", claims.ID, claims.ClientID, claims.Subject, time.Now().UTC()).First(&token).Error; err != nil {
		respondError(c, http.StatusUnauthorized, "invalid_token", "Access token telah dicabut.")
		c.Abort()
		return
	}
	now := time.Now().UTC()
	var client models.OAuthClient
	if err := database.DB.First(&client, "id = ?", claims.ClientID).Error; err != nil {
		database.DB.Model(&token).Where("revoked_at IS NULL").Update("revoked_at", now)
		respondError(c, http.StatusUnauthorized, "invalid_token", "Aplikasi pemilik access token tidak lagi terdaftar.")
		c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
		c.Abort()
		return
	}
	database.DB.Model(&token).Update("last_used_at", now)
	var user models.User
	if err := database.DB.First(&user, "id = ?", claims.Subject).Error; err != nil {
		respondError(c, http.StatusUnauthorized, "invalid_token", "Pemilik access token tidak ditemukan.")
		c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
		c.Abort()
		return
	}
	if _, _, _, denied := accountAccessError(&user); denied {
		database.DB.Model(&models.OAuthToken{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Update("revoked_at", now)
		respondError(c, http.StatusUnauthorized, "invalid_token", "Akun pemilik access token tidak lagi dapat digunakan.")
		c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
		c.Abort()
		return
	}
	if _, err := applicationAccess(database.DB, client, user.ID, now); err != nil {
		database.DB.Model(&token).Where("revoked_at IS NULL").Update("revoked_at", now)
		respondError(c, http.StatusUnauthorized, "invalid_token", "Akses akun ke aplikasi ini telah dicabut.")
		c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
		c.Abort()
		return
	}

	c.Set("accessClaims", claims)
	c.Set("userID", claims.Subject)
	c.Set("user", &user)
	c.Next()
}

func currentUser(c *gin.Context) (*models.User, bool) {
	value, exists := c.Get("user")
	if !exists {
		return nil, false
	}
	user, ok := value.(*models.User)
	return user, ok
}
