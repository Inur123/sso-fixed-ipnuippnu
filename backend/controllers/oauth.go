package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
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
	authorizationCodeLifetime = 5 * time.Minute
	accessTokenLifetime       = time.Hour
	refreshTokenLifetime      = 30 * 24 * time.Hour
)

var errInvalidOAuthClientCredentials = errors.New("oauth client credentials are no longer valid")

func AuthorizationServerMetadata(c *gin.Context) {
	issuer := utils.IssuerURL()
	c.JSON(http.StatusOK, gin.H{
		"issuer":                 issuer,
		"authorization_endpoint": issuer + "/oauth/authorize",
		"token_endpoint":         issuer + "/oauth/token",
		"revocation_endpoint":    issuer + "/oauth/revoke",
		"userinfo_endpoint":      issuer + "/v1/user/me",
		"jwks_uri":               issuer + "/oauth/jwks",
		"authorization_response_iss_parameter_supported": true,
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported":          []string{"client_secret_post"},
		"code_challenge_methods_supported":               []string{"S256"},
		"scopes_supported":                               []string{"openid", "profile", "email"},
		"prompt_values_supported":                        []string{promptConsent, promptSelectAccount},
	})
}

func OpenIDConfiguration(c *gin.Context) {
	issuer := utils.IssuerURL()
	c.JSON(http.StatusOK, gin.H{
		"issuer":                 issuer,
		"authorization_endpoint": issuer + "/oauth/authorize",
		"token_endpoint":         issuer + "/oauth/token",
		"revocation_endpoint":    issuer + "/oauth/revoke",
		"userinfo_endpoint":      issuer + "/v1/user/me",
		"jwks_uri":               issuer + "/oauth/jwks",
		"authorization_response_iss_parameter_supported": true,
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"subject_types_supported":                        []string{"public"},
		"id_token_signing_alg_values_supported":          []string{"RS256"},
		"token_endpoint_auth_methods_supported":          []string{"client_secret_post"},
		"code_challenge_methods_supported":               []string{"S256"},
		"scopes_supported":                               []string{"openid", "profile", "email"},
		"claims_supported":                               []string{"sub", "iss", "aud", "exp", "iat", "auth_time", "nonce", "name", "email", "email_verified"},
		"prompt_values_supported":                        []string{promptConsent, promptSelectAccount},
	})
}

func OIDCJWKS(c *gin.Context) {
	set, err := utils.OIDCJWKS()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Signing key OIDC tidak tersedia.")
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, set)
}

func GetOAuthClientInfo(c *gin.Context) {
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	if clientID == "" || redirectURI == "" {
		respondError(c, http.StatusBadRequest, "invalid_request", "client_id dan redirect_uri wajib diisi.")
		return
	}
	var client models.OAuthClient
	if err := database.DB.First(&client, "id = ? AND status = ?", clientID, models.ClientStatusActive).Error; err != nil || !exactRedirectMatch(client, redirectURI) {
		respondError(c, http.StatusBadRequest, "invalid_request", "Client atau redirect URI tidak valid.")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"client_id":      client.ID,
		"name":           client.Name,
		"description":    client.Description,
		"allowed_scopes": strings.Fields(client.AllowedScopes),
	})
}

// OAuthAuthorizationEndpoint adalah endpoint browser standar. Consent UI berada di
// Next.js, tetapi seluruh parameter divalidasi sebelum browser diarahkan ke sana.
func OAuthAuthorizationEndpoint(c *gin.Context) {
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	state := c.Query("state")
	responseType := c.Query("response_type")
	scope := utils.NormalizeScope(c.Query("scope"))
	challenge := c.Query("code_challenge")
	method := c.Query("code_challenge_method")
	nonce := c.Query("nonce")
	prompt := c.Query("prompt")

	var client models.OAuthClient
	redirectIsTrusted := database.DB.First(&client, "id = ? AND status = ?", clientID, models.ClientStatusActive).Error == nil && exactRedirectMatch(client, redirectURI)
	if !redirectIsTrusted {
		respondError(c, http.StatusBadRequest, "invalid_request", "Client atau redirect URI tidak valid.")
		return
	}
	_, promptErr := parseOAuthPrompt(prompt)
	openidWithoutNonce := utils.ScopeAllowed("openid", scope) && nonce == ""
	if responseType != "code" || state == "" || method != "S256" || len(challenge) != 43 || !utils.ScopeAllowed(scope, client.AllowedScopes) || openidWithoutNonce || promptErr != nil {
		location, _ := oauthRedirectURL(redirectURI, map[string]string{"error": "invalid_request", "error_description": "Gunakan Authorization Code, state, scope yang diizinkan, dan PKCE S256.", "state": state, "iss": utils.IssuerURL()})
		c.Redirect(http.StatusFound, location)
		return
	}
	frontendURL := strings.TrimRight(os.Getenv("FRONTEND_PUBLIC_URL"), "/")
	target, err := url.Parse(frontendURL + "/oauth/authorize")
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Konfigurasi frontend tidak valid.")
		return
	}
	target.RawQuery = c.Request.URL.RawQuery
	c.Redirect(http.StatusFound, target.String())
}

type AuthorizeRequest struct {
	ClientID            string `json:"client_id" binding:"required"`
	RedirectURI         string `json:"redirect_uri" binding:"required"`
	ResponseType        string `json:"response_type" binding:"required"`
	Scope               string `json:"scope" binding:"required"`
	State               string `json:"state" binding:"required"`
	CodeChallenge       string `json:"code_challenge" binding:"required"`
	CodeChallengeMethod string `json:"code_challenge_method" binding:"required"`
	Nonce               string `json:"nonce"`
	Prompt              string `json:"prompt"`
	ConsentApproved     bool   `json:"consent_approved"`
}

func OAuthAuthorize(c *gin.Context) {
	var req AuthorizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Parameter OAuth tidak lengkap.")
		return
	}
	if req.ResponseType != "code" || req.CodeChallengeMethod != "S256" || len(req.CodeChallenge) != 43 {
		respondError(c, http.StatusBadRequest, "invalid_request", "Gunakan response_type=code dan PKCE S256.")
		return
	}
	prompts, err := parseOAuthPrompt(req.Prompt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Nilai prompt OAuth tidak didukung.")
		return
	}

	var client models.OAuthClient
	if err := database.DB.First(&client, "id = ? AND status = ?", req.ClientID, models.ClientStatusActive).Error; err != nil || !exactRedirectMatch(client, req.RedirectURI) {
		respondError(c, http.StatusBadRequest, "invalid_request", "Client atau redirect URI tidak valid.")
		return
	}
	scope := utils.NormalizeScope(req.Scope)
	if !utils.ScopeAllowed(scope, client.AllowedScopes) {
		redirectOAuthError(c, req.RedirectURI, req.State, "invalid_scope", "Scope tidak diizinkan untuk aplikasi ini.")
		return
	}
	if utils.ScopeAllowed("openid", scope) && req.Nonce == "" {
		redirectOAuthError(c, req.RedirectURI, req.State, "invalid_request", "nonce wajib untuk permintaan OpenID Connect.")
		return
	}
	if _, err := applicationAccess(database.DB, client, c.GetString("userID"), time.Now().UTC()); err != nil {
		if errors.Is(err, errApplicationAccessDenied) {
			redirectOAuthError(c, req.RedirectURI, req.State, "access_denied", "Akun Anda belum diberi akses ke aplikasi ini.")
			return
		}
		respondError(c, http.StatusInternalServerError, "server_error", "Kebijakan akses aplikasi belum dapat diperiksa.")
		return
	}

	userID := c.GetString("userID")
	required, err := consentRequired(database.DB, client.ID, userID, scope)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Status persetujuan aplikasi belum dapat diperiksa.")
		return
	}
	if prompts[promptNone] && required {
		redirectOAuthError(c, req.RedirectURI, req.State, "consent_required", "Persetujuan pengguna diperlukan.")
		return
	}
	if missingRequiredConsentApproval(required, req.ConsentApproved) {
		respondError(c, http.StatusForbidden, "consent_required", "Tinjau dan setujui izin aplikasi sebelum melanjutkan.")
		return
	}

	rawCode, err := utils.RandomToken(32)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal membuat authorization code.")
		return
	}
	authCode := models.OAuthAuthCode{
		CodeHash:            utils.HashToken(rawCode),
		ClientID:            client.ID,
		UserID:              userID,
		RedirectURI:         req.RedirectURI,
		Scope:               scope,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		State:               req.State,
		Nonce:               req.Nonce,
		AuthTime:            currentSessionCreatedAt(c),
		ExpiresAt:           time.Now().UTC().Add(authorizationCodeLifetime),
	}
	now := time.Now().UTC()
	database.DB.Where("expires_at <= ?", now).Delete(&models.OAuthAuthCode{})
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&authCode).Error; err != nil {
			return err
		}
		return persistOAuthConsent(tx, client.ID, userID, scope, now)
	}); err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal menyimpan authorization code.")
		return
	}
	if required {
		auditFromContext(c, AuditOAuthConsent, "oauth_client", client.ID, "Izin akses aplikasi disetujui: "+client.Name+".")
	}

	redirectURL, err := oauthRedirectURL(req.RedirectURI, map[string]string{
		"code":  rawCode,
		"state": req.State,
		"iss":   utils.IssuerURL(),
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal membentuk redirect URL.")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"redirect_url": redirectURL})
}

type TokenRequest struct {
	GrantType    string `json:"grant_type" form:"grant_type" binding:"required"`
	Code         string `json:"code" form:"code"`
	ClientID     string `json:"client_id" form:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" form:"client_secret" binding:"required"`
	RedirectURI  string `json:"redirect_uri" form:"redirect_uri"`
	CodeVerifier string `json:"code_verifier" form:"code_verifier"`
	RefreshToken string `json:"refresh_token" form:"refresh_token"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token,omitempty"`
	auditUserID  string
}

func OAuthToken(c *gin.Context) {
	var req TokenRequest
	if err := c.ShouldBind(&req); err != nil {
		oauthTokenError(c, http.StatusBadRequest, "invalid_request", "Permintaan token tidak lengkap.")
		return
	}
	client, ok := authenticateClient(c, req.ClientID, req.ClientSecret)
	if !ok {
		return
	}

	var response tokenResponse
	var err error
	switch req.GrantType {
	case "authorization_code":
		response, err = exchangeAuthorizationCode(req, client)
	case "refresh_token":
		response, err = rotateRefreshToken(req, client)
	default:
		oauthTokenError(c, http.StatusBadRequest, "unsupported_grant_type", "Grant type tidak didukung.")
		return
	}
	if err != nil {
		if errors.Is(err, errInvalidOAuthClientCredentials) {
			c.Header("WWW-Authenticate", `Basic realm="oauth/token"`)
			oauthTokenError(c, http.StatusUnauthorized, "invalid_client", "Kredensial client tidak valid.")
			return
		}
		oauthTokenError(c, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}
	if req.GrantType == "authorization_code" && response.auditUserID != "" {
		auditWithActor(c, &response.auditUserID, AuditOAuthGrant, "oauth_client", client.ID, "Grant aplikasi SSO berhasil diterbitkan: "+client.Name+".")
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusOK, response)
}

func exchangeAuthorizationCode(req TokenRequest, client models.OAuthClient) (tokenResponse, error) {
	if req.Code == "" || req.RedirectURI == "" || req.CodeVerifier == "" {
		return tokenResponse{}, errors.New("code, redirect_uri, dan code_verifier wajib diisi")
	}
	var response tokenResponse
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockAndVerifyOAuthClient(tx, client.ID, req.ClientSecret); err != nil {
			return err
		}
		var code models.OAuthAuthCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code_hash = ? AND client_id = ?", utils.HashToken(req.Code), client.ID).First(&code).Error; err != nil {
			return errors.New("authorization code tidak valid")
		}
		if time.Now().UTC().After(code.ExpiresAt) || code.RedirectURI != req.RedirectURI || !utils.VerifyPKCES256(req.CodeVerifier, code.CodeChallenge) {
			tx.Delete(&code)
			return errors.New("authorization code kedaluwarsa atau verifikasi PKCE gagal")
		}
		if err := tx.Delete(&code).Error; err != nil {
			return err
		}
		familyID, err := utils.RandomToken(32)
		if err != nil {
			return err
		}
		response, err = issueTokenPair(tx, code.UserID, client.ID, code.Scope, familyID)
		if err != nil {
			return err
		}
		response.auditUserID = code.UserID
		if utils.ScopeAllowed("openid", code.Scope) {
			var user models.User
			if err := tx.First(&user, "id = ?", code.UserID).Error; err != nil {
				return err
			}
			response.IDToken, err = utils.GenerateIDToken(user.ID, user.Name, user.Email, user.EmailVerifiedAt != nil, client.ID, code.Nonce, code.Scope, code.AuthTime)
		}
		return err
	})
	return response, err
}

func rotateRefreshToken(req TokenRequest, client models.OAuthClient) (tokenResponse, error) {
	if req.RefreshToken == "" {
		return tokenResponse{}, errors.New("refresh_token wajib diisi")
	}
	var response tokenResponse
	var grantError error
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockAndVerifyOAuthClient(tx, client.ID, req.ClientSecret); err != nil {
			return err
		}
		var current models.OAuthToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("refresh_token_hash = ? AND client_id = ?", utils.HashToken(req.RefreshToken), client.ID).First(&current).Error; err != nil {
			return errors.New("refresh token tidak valid")
		}
		if current.RevokedAt != nil {
			now := time.Now().UTC()
			if err := tx.Model(&models.OAuthToken{}).Where("family_id = ? AND revoked_at IS NULL", current.FamilyID).Update("revoked_at", now).Error; err != nil {
				return err
			}
			grantError = errors.New("refresh token reuse terdeteksi; seluruh token family telah dicabut")
			return nil
		}
		if time.Now().UTC().After(current.RefreshExpiresAt) {
			grantError = errors.New("refresh token kedaluwarsa")
			return nil
		}
		now := time.Now().UTC()
		if err := tx.Model(&current).Updates(map[string]interface{}{"revoked_at": now, "last_used_at": now}).Error; err != nil {
			return err
		}
		var err error
		response, err = issueTokenPair(tx, current.UserID, client.ID, current.Scope, current.FamilyID)
		return err
	})
	if err == nil && grantError != nil {
		return tokenResponse{}, grantError
	}
	return response, err
}

func lockAndVerifyOAuthClient(tx *gorm.DB, clientID, presentedSecret string) error {
	var activeClient models.OAuthClient
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "secret_hash", "status").First(&activeClient, "id = ?", clientID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errInvalidOAuthClientCredentials
		}
		return err
	}
	if activeClient.Status != models.ClientStatusActive {
		return errInvalidOAuthClientCredentials
	}
	return verifyOAuthClientSecretHash(activeClient.SecretHash, presentedSecret)
}

func verifyOAuthClientSecretHash(secretHash, presentedSecret string) error {
	if bcrypt.CompareHashAndPassword([]byte(secretHash), []byte(presentedSecret)) != nil {
		return errInvalidOAuthClientCredentials
	}
	return nil
}

func issueTokenPair(tx *gorm.DB, userID, clientID, scope, familyID string) (tokenResponse, error) {
	var user models.User
	// Serialize token issuance with admin status/role updates. Jika admin lebih
	// dahulu mengunci pengguna, grant akan melihat status terbaru; jika issuance
	// lebih dahulu, transaksi admin akan mencabut token yang baru dibuat.
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
		return tokenResponse{}, errors.New("user tidak ditemukan")
	}
	if _, code, _, denied := accountAccessError(&user); denied {
		return tokenResponse{}, fmt.Errorf("akun tidak dapat digunakan: %s", code)
	}
	var client models.OAuthClient
	if err := tx.First(&client, "id = ?", clientID).Error; err != nil {
		return tokenResponse{}, errInvalidOAuthClientCredentials
	}
	_, err := applicationAccess(tx, client, user.ID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, errApplicationAccessDenied) {
			return tokenResponse{}, errors.New("akun tidak memiliki akses ke aplikasi")
		}
		return tokenResponse{}, err
	}
	jti, err := utils.RandomToken(32)
	if err != nil {
		return tokenResponse{}, err
	}
	refreshToken, err := utils.RandomToken(48)
	if err != nil {
		return tokenResponse{}, err
	}
	accessToken, err := utils.GenerateAccessToken(user.ID, clientID, scope, jti, accessTokenLifetime)
	if err != nil {
		return tokenResponse{}, err
	}
	now := time.Now().UTC()
	record := models.OAuthToken{
		AccessJTI:        jti,
		RefreshTokenHash: utils.HashToken(refreshToken),
		FamilyID:         familyID,
		ClientID:         clientID,
		UserID:           user.ID,
		Scope:            scope,
		ExpiresAt:        now.Add(accessTokenLifetime),
		RefreshExpiresAt: now.Add(refreshTokenLifetime),
	}
	if err := tx.Create(&record).Error; err != nil {
		return tokenResponse{}, err
	}
	return tokenResponse{AccessToken: accessToken, TokenType: "Bearer", ExpiresIn: int(accessTokenLifetime.Seconds()), RefreshToken: refreshToken, Scope: scope}, nil
}

type RevokeRequest struct {
	Token        string `json:"token" form:"token" binding:"required"`
	ClientID     string `json:"client_id" form:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" form:"client_secret" binding:"required"`
}

func OAuthRevoke(c *gin.Context) {
	var req RevokeRequest
	if err := c.ShouldBind(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Token dan kredensial client wajib diisi.")
		return
	}
	client, ok := authenticateClient(c, req.ClientID, req.ClientSecret)
	if !ok {
		return
	}
	// RFC 7009 menerima refresh token maupun access token. Jika token dikenali,
	// seluruh family grant dicabut agar pasangan refresh/access token tidak
	// tetap dapat dipakai setelah pengguna meminta pemutusan akses.
	var record models.OAuthToken
	err := database.DB.Where("client_id = ? AND refresh_token_hash = ?", client.ID, utils.HashToken(req.Token)).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Access token tidak disimpan mentah. JTI di dalam JWT dipakai untuk
		// menemukan grant apabila access token aktif dikirim ke endpoint ini.
		if claims, tokenErr := utils.ValidateAccessToken(req.Token); tokenErr == nil && claims.ClientID == client.ID {
			err = database.DB.Where("client_id = ? AND access_jti = ?", client.ID, claims.ID).First(&record).Error
		}
	}
	revoked := false
	if err == nil {
		now := time.Now().UTC()
		result := database.DB.Model(&models.OAuthToken{}).
			Where("client_id = ? AND user_id = ? AND family_id = ? AND revoked_at IS NULL", record.ClientID, record.UserID, record.FamilyID).
			Update("revoked_at", now)
		if result.Error != nil {
			respondError(c, http.StatusInternalServerError, "server_error", "Token belum dapat dicabut.")
			return
		}
		revoked = result.RowsAffected > 0
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, http.StatusInternalServerError, "server_error", "Token belum dapat dicabut.")
		return
	}
	if revoked {
		auditWithActor(c, &record.UserID, AuditOAuthTokenRevoke, "oauth_connection", record.FamilyID, "Grant aplikasi dicabut melalui endpoint revocation.")
	}
	// RFC 7009 mengharuskan respons sukses walaupun token tidak dikenal.
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Status(http.StatusOK)
}

func authenticateClient(c *gin.Context, clientID, secret string) (models.OAuthClient, bool) {
	var client models.OAuthClient
	if err := database.DB.First(&client, "id = ? AND status = ?", clientID, models.ClientStatusActive).Error; err != nil || verifyOAuthClientSecretHash(client.SecretHash, secret) != nil {
		c.Header("WWW-Authenticate", `Basic realm="oauth/token"`)
		oauthTokenError(c, http.StatusUnauthorized, "invalid_client", "Kredensial client tidak valid.")
		return models.OAuthClient{}, false
	}
	return client, true
}

func exactRedirectMatch(client models.OAuthClient, requested string) bool {
	for _, registered := range splitLines(client.RedirectURIs) {
		if registered == requested {
			return true
		}
	}
	return false
}

func oauthRedirectURL(base string, params map[string]string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	for key, value := range params {
		if value != "" {
			query.Set(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func redirectOAuthError(c *gin.Context, redirectURI, state, code, description string) {
	redirectURL, err := oauthRedirectURL(redirectURI, map[string]string{"error": code, "error_description": description, "state": state, "iss": utils.IssuerURL()})
	if err != nil {
		respondError(c, http.StatusBadRequest, code, description)
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": code, "message": description, "redirect_url": redirectURL})
}

func oauthTokenError(c *gin.Context, status int, code, description string) {
	c.Header("Cache-Control", "no-store")
	c.JSON(status, gin.H{"error": code, "error_description": description, "message": description})
}

func currentSessionCreatedAt(c *gin.Context) time.Time {
	if value, exists := c.Get("session"); exists {
		if session, ok := value.(*models.Session); ok {
			return session.CreatedAt
		}
	}
	return time.Now().UTC()
}
