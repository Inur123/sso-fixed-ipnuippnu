package controllers

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sso-backend/database"
	"sso-backend/models"
	"sso-backend/utils"
)

type CreateClientRequest struct {
	Name         string   `json:"name" binding:"required,min=2,max=120"`
	Description  string   `json:"description" binding:"max=500"`
	RedirectURIs []string `json:"redirect_uris" binding:"required,min=1,max=10,dive,required"`
	AccessPolicy string   `json:"access_policy"`
}

type UpdateClientRequest struct {
	Name         string   `json:"name" binding:"required,min=2,max=120"`
	Description  string   `json:"description" binding:"max=500"`
	RedirectURIs []string `json:"redirect_uris" binding:"required,min=1,max=10,dive,required"`
	AccessPolicy string   `json:"access_policy"`
}

type RegenerateClientSecretRequest struct {
	ExpectedVersion uint64 `json:"expected_version" binding:"required,min=1"`
}

var (
	errSecretVersionConflict  = errors.New("client secret version conflict")
	errSecretVersionExhausted = errors.New("client secret version exhausted")
)

type ClientResponse struct {
	ID              string   `json:"client_id"`
	Secret          string   `json:"client_secret,omitempty"`
	SecretAvailable bool     `json:"secret_available"`
	SecretVersion   uint64   `json:"secret_version"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	RedirectURIs    []string `json:"redirect_uris"`
	AllowedScopes   []string `json:"allowed_scopes"`
	OwnerID         string   `json:"owner_id"`
	AccessPolicy    string   `json:"access_policy"`
	Status          string   `json:"status"`
	AssignmentCount int64    `json:"assignment_count"`
}

type AssignClientUserRequest struct {
	Identifier string `json:"identifier" binding:"required,max=254"`
}

type ClientAssignmentResponse struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatar"`
}

func CreateClient(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req CreateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Nama dan minimal satu redirect URI yang valid wajib diisi.")
		return
	}
	redirectURIs, err := validateRedirectURIs(req.RedirectURIs)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
		return
	}
	accessPolicy, err := normalizeAccessPolicy(req.AccessPolicy)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_access_policy", err.Error())
		return
	}
	// Scope identitas dasar konsisten untuk semua client dashboard. Otorisasi
	// bisnis tetap dikelola oleh aplikasi tujuan, bukan oleh IdP.
	scopes, err := normalizeClientScopes(nil)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_scope", err.Error())
		return
	}

	secret, err := utils.RandomToken(32)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal membuat client secret.")
		return
	}
	secretHash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengamankan client secret.")
		return
	}

	client := models.OAuthClient{
		Name:          strings.TrimSpace(req.Name),
		Description:   strings.TrimSpace(req.Description),
		RedirectURIs:  strings.Join(redirectURIs, "\n"),
		AllowedScopes: strings.Join(scopes, " "),
		SecretHash:    string(secretHash),
		SecretVersion: 1,
		OwnerID:       c.GetString("userID"),
		AccessPolicy:  accessPolicy,
		Status:        models.ClientStatusActive,
	}
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&client).Error; err != nil {
			return err
		}
		ciphertext, err := utils.EncryptClientSecret(client.ID, secret)
		if err != nil {
			return err
		}
		client.SecretCiphertext = &ciphertext
		if err := tx.Model(&client).Update("secret_ciphertext", ciphertext).Error; err != nil {
			return err
		}
		// Owner harus dapat mencoba integrasi miliknya sendiri walaupun policy
		// default bersifat deny-by-default.
		return tx.Create(&models.OAuthClientAssignment{
			ClientID: client.ID, UserID: client.OwnerID, IsActive: true,
		}).Error
	}); err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mendaftarkan aplikasi.")
		return
	}

	auditFromContext(c, AuditOAuthClientCreate, "oauth_client", client.ID, "Aplikasi SSO didaftarkan: "+client.Name+".")
	c.JSON(http.StatusCreated, gin.H{
		"message": "Aplikasi berhasil didaftarkan. Simpan client secret di tempat yang aman.",
		"client":  clientResponseWithAssignmentCount(database.DB, client, secret),
	})
}

func GetClients(c *gin.Context) {
	var clients []models.OAuthClient
	query := database.DB.Order("created_at DESC")
	if user, ok := currentUser(c); !ok || user.Role != models.RoleSuperAdmin {
		query = query.Where("owner_id = ?", c.GetString("userID"))
	}
	if err := query.Find(&clients).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengambil aplikasi.")
		return
	}
	items := make([]ClientResponse, 0, len(clients))
	for _, client := range clients {
		items = append(items, clientResponseWithAssignmentCount(database.DB, client, ""))
	}
	c.JSON(http.StatusOK, gin.H{"clients": items})
}

func GetClient(c *gin.Context) {
	var client models.OAuthClient
	if err := findClientForManagement(database.DB, c, c.Param("id"), &client); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusNotFound, "not_found", "Aplikasi tidak ditemukan.")
			return
		}
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengambil aplikasi.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"client": clientResponseWithAssignmentCount(database.DB, client, "")})
}

func UpdateClient(c *gin.Context) {
	var req UpdateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "Nama dan minimal satu redirect URI yang valid wajib diisi.")
		return
	}
	redirectURIs, err := validateRedirectURIs(req.RedirectURIs)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
		return
	}
	accessPolicy, err := normalizeAccessPolicy(req.AccessPolicy)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_access_policy", err.Error())
		return
	}

	var client models.OAuthClient
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := findClientForManagement(tx.Clauses(clause.Locking{Strength: "UPDATE"}), c, c.Param("id"), &client); err != nil {
			return err
		}
		client.Name = strings.TrimSpace(req.Name)
		client.Description = strings.TrimSpace(req.Description)
		client.RedirectURIs = strings.Join(redirectURIs, "\n")
		policyChanged := client.AccessPolicy != accessPolicy
		client.AccessPolicy = accessPolicy
		if err := tx.Model(&client).Updates(map[string]interface{}{
			"name":          client.Name,
			"description":   client.Description,
			"redirect_uris": client.RedirectURIs,
			"access_policy": client.AccessPolicy,
		}).Error; err != nil {
			return err
		}
		if client.AccessPolicy == models.AccessPolicyAssignedOnly {
			var ownerAssignment models.OAuthClientAssignment
			result := tx.Where("client_id = ? AND user_id = ?", client.ID, client.OwnerID).First(&ownerAssignment)
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				if err := tx.Create(&models.OAuthClientAssignment{ClientID: client.ID, UserID: client.OwnerID, IsActive: true}).Error; err != nil {
					return err
				}
			} else if result.Error != nil {
				return result.Error
			} else if !ownerAssignment.IsActive {
				if err := tx.Model(&ownerAssignment).Update("is_active", true).Error; err != nil {
					return err
				}
			}
		}
		// Authorization code menyimpan redirect URI saat diterbitkan. Hapus code
		// lama agar konfigurasi callback sebelumnya tidak dapat dipakai kembali.
		if err := tx.Where("client_id = ?", client.ID).Delete(&models.OAuthAuthCode{}).Error; err != nil {
			return err
		}
		if policyChanged {
			return tx.Model(&models.OAuthToken{}).Where("client_id = ? AND revoked_at IS NULL", client.ID).Update("revoked_at", tx.NowFunc()).Error
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, http.StatusNotFound, "not_found", "Aplikasi tidak ditemukan.")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal memperbarui aplikasi.")
		return
	}
	auditFromContext(c, AuditOAuthClientUpdate, "oauth_client", client.ID, "Konfigurasi aplikasi SSO diperbarui: "+client.Name+".")
	c.JSON(http.StatusOK, gin.H{
		"message": "Data aplikasi berhasil diperbarui.",
		"client":  clientResponseWithAssignmentCount(database.DB, client, ""),
	})
}

// GetClientSecret hanya membuka ciphertext milik actor yang juga merupakan
// owner aplikasi. Nilai maupun aksi melihat tidak ditulis ke audit log dan
// response dilarang di-cache.
func GetClientSecret(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var client models.OAuthClient
	if err := findClientForManagement(database.DB, c, c.Param("id"), &client); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusNotFound, "not_found", "Aplikasi tidak ditemukan.")
			return
		}
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengambil client secret.")
		return
	}
	if client.SecretCiphertext == nil || strings.TrimSpace(*client.SecretCiphertext) == "" {
		respondError(c, http.StatusConflict, "secret_unavailable", "Client secret lama tidak dapat ditampilkan. Buat secret baru untuk mengaktifkan fitur ini.")
		return
	}
	secret, err := utils.DecryptClientSecret(client.ID, *client.SecretCiphertext)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "secret_decryption_failed", "Client secret tidak dapat dibuka. Buat secret baru atau hubungi administrator server.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"client_id": client.ID, "client_secret": secret, "secret_version": normalizedClientSecretVersion(client.SecretVersion)})
}

// RegenerateClientSecret mengganti hash dan ciphertext dalam satu transaksi,
// sekaligus mencabut seluruh grant lama agar kredensial sebelumnya tidak dapat
// mempertahankan akses.
func RegenerateClientSecret(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var req RegenerateClientSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "expected_version wajib diisi dengan versi client secret terbaru.")
		return
	}
	secret, err := utils.RandomToken(32)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal membuat client secret baru.")
		return
	}
	secretHash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal mengamankan client secret baru.")
		return
	}

	var client models.OAuthClient
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := findClientForManagement(tx.Clauses(clause.Locking{Strength: "UPDATE"}), c, c.Param("id"), &client); err != nil {
			return err
		}
		nextVersion, err := nextClientSecretVersion(client.SecretVersion, req.ExpectedVersion)
		if err != nil {
			return err
		}
		ciphertext, err := utils.EncryptClientSecret(client.ID, secret)
		if err != nil {
			return err
		}
		if err := tx.Model(&client).Updates(map[string]interface{}{
			"secret_hash":       string(secretHash),
			"secret_ciphertext": ciphertext,
			"secret_version":    nextVersion,
		}).Error; err != nil {
			return err
		}
		now := tx.NowFunc()
		if err := tx.Model(&models.OAuthToken{}).
			Where("client_id = ? AND revoked_at IS NULL", client.ID).
			Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Where("client_id = ?", client.ID).Delete(&models.OAuthAuthCode{}).Error; err != nil {
			return err
		}
		client.SecretCiphertext = &ciphertext
		client.SecretVersion = nextVersion
		return persistAuditFromContext(tx, c, AuditOAuthClientSecretRegenerate, "oauth_client", client.ID, "Client secret aplikasi dibuat ulang; token dan authorization code lama dicabut.")
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, http.StatusNotFound, "not_found", "Aplikasi tidak ditemukan.")
		return
	}
	if errors.Is(err, errSecretVersionConflict) {
		respondError(c, http.StatusConflict, "secret_version_conflict", "Client secret telah berubah. Muat ulang data aplikasi sebelum mencoba kembali.")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal membuat ulang client secret.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Client secret baru berhasil dibuat. Seluruh akses lama telah dicabut.",
		"client_id":      client.ID,
		"client_secret":  secret,
		"secret_version": client.SecretVersion,
	})
}

func DeleteClient(c *gin.Context) {
	var client models.OAuthClient
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := findClientForManagement(tx.Clauses(clause.Locking{Strength: "UPDATE"}), c, c.Param("id"), &client); err != nil {
			return err
		}
		now := tx.NowFunc()
		if err := tx.Model(&models.OAuthToken{}).
			Where("client_id = ? AND revoked_at IS NULL", client.ID).
			Update("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Where("client_id = ?", client.ID).Delete(&models.OAuthAuthCode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("client_id = ?", client.ID).Delete(&models.OAuthClientAssignment{}).Error; err != nil {
			return err
		}
		return tx.Delete(&client).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		respondError(c, http.StatusNotFound, "not_found", "Aplikasi tidak ditemukan.")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "server_error", "Gagal menghapus aplikasi.")
		return
	}
	auditFromContext(c, AuditOAuthClientDelete, "oauth_client", client.ID, "Aplikasi SSO dihapus dan seluruh aksesnya dicabut: "+client.Name+".")
	c.JSON(http.StatusOK, gin.H{"message": "Aplikasi, authorization code, dan seluruh tokennya telah dicabut."})
}

func clientResponse(client models.OAuthClient, secret string) ClientResponse {
	return ClientResponse{
		ID:              client.ID,
		Secret:          secret,
		SecretAvailable: client.SecretCiphertext != nil && strings.TrimSpace(*client.SecretCiphertext) != "",
		SecretVersion:   normalizedClientSecretVersion(client.SecretVersion),
		Name:            client.Name,
		Description:     client.Description,
		RedirectURIs:    splitLines(client.RedirectURIs),
		AllowedScopes:   strings.Fields(client.AllowedScopes),
		OwnerID:         client.OwnerID,
		AccessPolicy:    client.AccessPolicy,
		Status:          client.Status,
	}
}

func clientResponseWithAssignmentCount(db *gorm.DB, client models.OAuthClient, secret string) ClientResponse {
	response := clientResponse(client, secret)
	if response.AccessPolicy == "" {
		response.AccessPolicy = models.AccessPolicyAssignedOnly
	}
	if response.Status == "" {
		response.Status = models.ClientStatusActive
	}
	_ = db.Model(&models.OAuthClientAssignment{}).Where("client_id = ?", client.ID).Count(&response.AssignmentCount).Error
	return response
}

func findClientForManagement(db *gorm.DB, c *gin.Context, id string, client *models.OAuthClient) error {
	query := db.Where("id = ?", id)
	user, ok := currentUser(c)
	if !ok || user.Role != models.RoleSuperAdmin {
		query = query.Where("owner_id = ?", c.GetString("userID"))
	}
	return query.First(client).Error
}

func normalizedClientSecretVersion(version uint64) uint64 {
	if version == 0 {
		return 1
	}
	return version
}

func nextClientSecretVersion(current, expected uint64) (uint64, error) {
	current = normalizedClientSecretVersion(current)
	if current != expected {
		return 0, errSecretVersionConflict
	}
	if current == ^uint64(0) {
		return 0, errSecretVersionExhausted
	}
	return current + 1, nil
}

func validateRedirectURIs(values []string) ([]string, error) {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, raw := range values {
		raw = strings.TrimSpace(raw)
		parsed, err := url.ParseRequestURI(raw)
		// ParseRequestURI memperlakukan sebagian karakter '#' sebagai bagian
		// opaque dari request URI; tolak secara eksplisit sebelum parsing.
		if err != nil || parsed.Host == "" || strings.Contains(raw, "#") || parsed.Fragment != "" {
			return nil, &validationError{"Redirect URI harus berupa URL absolut tanpa fragment."}
		}
		isLocalhost := parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1"
		if parsed.Scheme != "https" && !(isLocalhost && parsed.Scheme == "http") {
			return nil, &validationError{"Redirect URI produksi wajib menggunakan HTTPS; HTTP hanya diizinkan untuk localhost."}
		}
		if _, exists := seen[raw]; !exists {
			seen[raw] = struct{}{}
			result = append(result, raw)
		}
	}
	return result, nil
}

func normalizeClientScopes(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{"email", "openid", "profile"}, nil
	}
	allowed := map[string]bool{"openid": true, "profile": true, "email": true}
	seen := make(map[string]struct{})
	for _, scope := range values {
		scope = strings.TrimSpace(scope)
		if !allowed[scope] {
			return nil, &validationError{"Scope yang didukung hanya openid, profile, dan email."}
		}
		seen[scope] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for scope := range seen {
		result = append(result, scope)
	}
	sort.Strings(result)
	return result, nil
}

func splitLines(value string) []string {
	result := make([]string, 0)
	for _, item := range strings.Split(value, "\n") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

type validationError struct{ message string }

func (e *validationError) Error() string { return e.message }
