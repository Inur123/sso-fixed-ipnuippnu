package database

import (
	"log"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"sso-backend/models"
)

var DB *gorm.DB

func Connect() {
	required := []string{"DB_HOST", "DB_USER", "DB_NAME", "DB_PORT"}
	for _, key := range required {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			log.Fatalf("required environment variable %s is not set", key)
		}
	}
	databaseURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD")),
		Host:   net.JoinHostPort(os.Getenv("DB_HOST"), os.Getenv("DB_PORT")),
		Path:   os.Getenv("DB_NAME"),
	}
	query := databaseURL.Query()
	sslMode := strings.TrimSpace(os.Getenv("DB_SSLMODE"))
	if sslMode == "" {
		log.Fatal("required environment variable DB_SSLMODE is not set")
	}
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") && !validProductionSSLMode(os.Getenv("DB_HOST"), sslMode) {
		log.Fatal("DB_SSLMODE must be verify-ca or verify-full in production; disable is only allowed for a loopback database")
	}
	query.Set("sslmode", sslMode)
	databaseURL.RawQuery = query.Encode()

	db, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	DB = db
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("Failed to configure database pool:", err)
	}
	sqlDB.SetMaxOpenConns(envInt("DB_MAX_OPEN_CONNS", 25, 5, 200))
	sqlDB.SetMaxIdleConns(envInt("DB_MAX_IDLE_CONNS", 10, 2, 100))
	sqlDB.SetConnMaxLifetime(time.Duration(envInt("DB_CONN_MAX_LIFETIME_MINUTES", 30, 1, 240)) * time.Minute)
	sqlDB.SetConnMaxIdleTime(time.Duration(envInt("DB_CONN_MAX_IDLE_TIME_MINUTES", 5, 1, 60)) * time.Minute)
	var currentDatabase string
	if err := DB.Raw("SELECT current_database()").Scan(&currentDatabase).Error; err != nil {
		log.Fatal("Failed to verify database:", err)
	}
	if currentDatabase != os.Getenv("DB_NAME") {
		log.Fatalf("database mismatch: connected to %q instead of configured %q", currentDatabase, os.Getenv("DB_NAME"))
	}
	log.Printf("connected to PostgreSQL database %q", currentDatabase)

	// AutoMigrate hanya untuk pengembangan awal. Produksi sebaiknya memakai migrasi terversi.
	err = DB.AutoMigrate(&models.User{}, &models.EmailVerificationOTP{}, &models.Session{}, &models.OAuthClient{}, &models.OAuthClientAssignment{}, &models.OAuthConsent{}, &models.OAuthAuthCode{}, &models.OAuthToken{}, &models.AuditLog{}, &models.ProvisioningOutbox{})
	if err != nil {
		log.Fatal("Failed to migrate:", err)
	}
	// Client yang dibuat sebelum fitur admission control tetap dapat dikelola:
	// pemiliknya menjadi assignment awal. Gunakan model GORM agar backfill tidak
	// bergantung pada tebakan nama tabel untuk initialism OAuth.
	if err := backfillOAuthClientOwnerAssignments(); err != nil {
		log.Fatal("Failed to backfill OAuth client owner assignments:", err)
	}
	// Grant aktif yang dibuat sebelum tabel consent tersedia sudah pernah
	// melewati layar persetujuan. Rekam persetujuan itu satu kali agar upgrade
	// tidak meminta pengguna menyetujui scope yang sama pada setiap login.
	// Consent yang sudah ada (termasuk yang dicabut) tidak pernah diaktifkan
	// kembali oleh backfill ini.
	if err := backfillOAuthConsentsFromActiveGrants(); err != nil {
		log.Fatal("Failed to backfill OAuth consents:", err)
	}
	if err := DB.Model(&models.OAuthClient{}).
		Where("deleted_at IS NULL").
		Update("allowed_scopes", "email openid profile").Error; err != nil {
		log.Fatal("Failed to align OAuth client scopes:", err)
	}

	bootstrapSuperAdmin()
}

func validProductionSSLMode(host, sslMode string) bool {
	mode := strings.ToLower(strings.TrimSpace(sslMode))
	if mode == "verify-ca" || mode == "verify-full" {
		return true
	}
	if mode != "disable" {
		return false
	}

	// PostgreSQL pada host yang sama tidak melewati jaringan. Mengizinkan
	// koneksi tanpa TLS hanya untuk alamat loopback menghindari sertifikat
	// self-signed lokal tanpa melonggarkan koneksi database eksternal.
	host = strings.TrimSpace(strings.ToLower(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func envInt(key string, fallback, minimum, maximum int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		log.Fatalf("%s must be between %d and %d", key, minimum, maximum)
	}
	return value
}

func backfillOAuthClientOwnerAssignments() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var clients []models.OAuthClient
		if err := tx.Find(&clients).Error; err != nil {
			return err
		}
		for _, client := range clients {
			var assignmentCount int64
			if err := tx.Unscoped().Model(&models.OAuthClientAssignment{}).
				Where("client_id = ?", client.ID).Count(&assignmentCount).Error; err != nil {
				return err
			}
			if assignmentCount > 0 {
				continue
			}
			if err := tx.Create(&models.OAuthClientAssignment{
				ClientID: client.ID,
				UserID:   client.OwnerID,
				IsActive: true,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func backfillOAuthConsentsFromActiveGrants() error {
	now := time.Now().UTC()
	return DB.Transaction(func(tx *gorm.DB) error {
		var tokens []models.OAuthToken
		if err := tx.
			Where("revoked_at IS NULL AND (expires_at > ? OR refresh_expires_at > ?)", now, now).
			Order("created_at ASC").
			Find(&tokens).Error; err != nil {
			return err
		}

		for _, candidate := range oauthConsentBackfillCandidates(tokens) {
			var existing int64
			if err := tx.Model(&models.OAuthConsent{}).
				Where("client_id = ? AND user_id = ?", candidate.ClientID, candidate.UserID).
				Count(&existing).Error; err != nil {
				return err
			}
			if existing > 0 {
				continue
			}
			if err := tx.Create(&candidate).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func oauthConsentBackfillCandidates(tokens []models.OAuthToken) []models.OAuthConsent {
	type grant struct {
		clientID  string
		userID    string
		scopes    map[string]struct{}
		grantedAt time.Time
	}
	grants := make(map[string]*grant)
	for _, token := range tokens {
		key := token.ClientID + ":" + token.UserID
		item, exists := grants[key]
		if !exists {
			item = &grant{
				clientID: token.ClientID, userID: token.UserID,
				scopes: make(map[string]struct{}), grantedAt: token.CreatedAt,
			}
			grants[key] = item
		}
		if token.CreatedAt.Before(item.grantedAt) {
			item.grantedAt = token.CreatedAt
		}
		for _, scope := range strings.Fields(token.Scope) {
			item.scopes[scope] = struct{}{}
		}
	}

	candidates := make([]models.OAuthConsent, 0, len(grants))
	for _, item := range grants {
		scopes := make([]string, 0, len(item.scopes))
		for scope := range item.scopes {
			scopes = append(scopes, scope)
		}
		sort.Strings(scopes)
		candidates = append(candidates, models.OAuthConsent{
			ClientID: item.clientID, UserID: item.userID,
			Scope: strings.Join(scopes, " "), GrantedAt: item.grantedAt,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ClientID == candidates[j].ClientID {
			return candidates[i].UserID < candidates[j].UserID
		}
		return candidates[i].ClientID < candidates[j].ClientID
	})
	return candidates
}

// bootstrapSuperAdmin hanya mempromosikan akun terverifikasi yang emailnya
// dikunci melalui environment. Registrasi publik tetap selalu dimulai sebagai
// anggota; promosi normal dilakukan pada transaksi verifikasi OTP.
func bootstrapSuperAdmin() {
	email := strings.ToLower(strings.TrimSpace(os.Getenv("SUPER_ADMIN_EMAIL")))
	if email == "" {
		return
	}
	result := DB.Model(&models.User{}).
		Where("LOWER(email) = ? AND email_verified_at IS NOT NULL", email).
		Update("role", models.RoleSuperAdmin)
	if result.Error != nil {
		log.Printf("failed to bootstrap super admin: %v", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("super admin bootstrap applied to configured email")
	}
}
