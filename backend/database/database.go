package database

import (
	"log"
	"net"
	"net/url"
	"os"
	"strings"

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
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") && sslMode != "verify-ca" && sslMode != "verify-full" {
		log.Fatal("DB_SSLMODE must be verify-ca or verify-full in production")
	}
	query.Set("sslmode", sslMode)
	databaseURL.RawQuery = query.Encode()

	db, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	DB = db
	var currentDatabase string
	if err := DB.Raw("SELECT current_database()").Scan(&currentDatabase).Error; err != nil {
		log.Fatal("Failed to verify database:", err)
	}
	if currentDatabase != os.Getenv("DB_NAME") {
		log.Fatalf("database mismatch: connected to %q instead of configured %q", currentDatabase, os.Getenv("DB_NAME"))
	}
	log.Printf("connected to PostgreSQL database %q", currentDatabase)

	// AutoMigrate hanya untuk pengembangan awal. Produksi sebaiknya memakai migrasi terversi.
	err = DB.AutoMigrate(&models.User{}, &models.EmailVerificationOTP{}, &models.Session{}, &models.OAuthClient{}, &models.OAuthClientAssignment{}, &models.OAuthAuthCode{}, &models.OAuthToken{}, &models.AuditLog{})
	if err != nil {
		log.Fatal("Failed to migrate:", err)
	}
	// Client yang dibuat sebelum fitur admission control tetap dapat dikelola:
	// pemiliknya menjadi assignment awal. Gunakan model GORM agar backfill tidak
	// bergantung pada tebakan nama tabel untuk initialism OAuth.
	if err := backfillOAuthClientOwnerAssignments(); err != nil {
		log.Fatal("Failed to backfill OAuth client owner assignments:", err)
	}
	if err := DB.Model(&models.OAuthClient{}).
		Where("deleted_at IS NULL").
		Update("allowed_scopes", "email openid profile").Error; err != nil {
		log.Fatal("Failed to align OAuth client scopes:", err)
	}

	bootstrapSuperAdmin()
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
