// Package backup manages encrypted, serialized PostgreSQL backups.
package backup

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/joho/godotenv"
	"sso-backend/internal/apptime"
)

const Retention = 10

type Config struct {
	Enabled                                               bool
	Namespace                                             string
	Recipient                                             *age.X25519Recipient
	StorageKey                                            []byte
	Bucket, AccountID, AccessKey, SecretKey               string
	DumpPath                                              string
	RestorePath, IdentityFile                             string
	DBHost, DBPort, DBName, DBUser, DBPassword, DBSSLMode string
	Timeout                                               time.Duration
	MaxBytes                                              int64
}

var safePrefix = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9/_-]*$`)

func ConfigFromEnv() (Config, error) {
	c := Config{Timeout: 30 * time.Minute, MaxBytes: 512 << 20}
	raw := strings.TrimSpace(os.Getenv("BACKUP_ENABLED"))
	if raw == "" || raw == "false" {
		return c, nil
	}
	if raw != "true" {
		return c, fmt.Errorf("BACKUP_ENABLED harus true atau false")
	}
	c.Enabled = true
	recipient, storageKey, err := keySettings()
	if err != nil {
		return c, err
	}
	c.Recipient, err = age.ParseX25519Recipient(recipient)
	if err != nil {
		return c, fmt.Errorf("BACKUP_AGE_RECIPIENT harus berupa public key age yang valid")
	}
	c.StorageKey, err = base64.StdEncoding.DecodeString(storageKey)
	if err != nil || len(c.StorageKey) != 32 {
		return c, fmt.Errorf("BACKUP_R2_SSE_KEY harus base64 dari 32 byte acak")
	}
	prefix := strings.Trim(strings.TrimSpace(os.Getenv("BACKUP_PREFIX")), "/")
	if prefix == "" {
		prefix = "backups/database"
	}
	if len(prefix) > 200 || !safePrefix.MatchString(prefix) || strings.Contains(prefix, "//") || !strings.HasPrefix(prefix, "backups/") {
		return c, fmt.Errorf("BACKUP_PREFIX harus berupa path di dalam backups/")
	}
	environment := strings.TrimSpace(os.Getenv("APP_ENV"))
	if environment == "" || len(environment) > 32 || !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(environment) {
		return c, fmt.Errorf("APP_ENV tidak valid untuk namespace backup")
	}
	c.DBHost, c.DBPort = os.Getenv("DB_HOST"), os.Getenv("DB_PORT")
	c.DBName, c.DBUser, c.DBPassword, c.DBSSLMode = os.Getenv("DB_NAME"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_SSLMODE")
	c.Bucket, c.AccountID = os.Getenv("R2_BUCKET_NAME"), os.Getenv("R2_ACCOUNT_ID")
	c.AccessKey, c.SecretKey = os.Getenv("R2_ACCESS_KEY_ID"), os.Getenv("R2_SECRET_ACCESS_KEY")
	for _, p := range []struct{ name, value string }{
		{"DB_HOST", c.DBHost}, {"DB_PORT", c.DBPort}, {"DB_NAME", c.DBName}, {"DB_USER", c.DBUser}, {"DB_SSLMODE", c.DBSSLMode},
		{"R2_BUCKET_NAME", c.Bucket}, {"R2_ACCOUNT_ID", c.AccountID}, {"R2_ACCESS_KEY_ID", c.AccessKey}, {"R2_SECRET_ACCESS_KEY", c.SecretKey},
		{"FRONTEND_PUBLIC_URL", os.Getenv("FRONTEND_PUBLIC_URL")},
	} {
		if strings.TrimSpace(p.value) == "" {
			return c, fmt.Errorf("%s wajib untuk backup", p.name)
		}
	}
	if !regexp.MustCompile(`^[a-fA-F0-9]{32}$`).MatchString(c.AccountID) {
		return c, fmt.Errorf("R2_ACCOUNT_ID tidak valid")
	}
	identity := sha256.Sum256([]byte(strings.TrimRight(os.Getenv("FRONTEND_PUBLIC_URL"), "/") + "\x00" + c.DBName))
	c.Namespace = prefix + "/" + environment + "/" + hex.EncodeToString(identity[:6]) + "/"
	c.DumpPath = strings.TrimSpace(os.Getenv("BACKUP_PG_DUMP_PATH"))
	if c.DumpPath == "" {
		c.DumpPath = "pg_dump"
	}
	c.IdentityFile = strings.TrimSpace(os.Getenv("BACKUP_IDENTITY_FILE"))
	c.RestorePath = strings.TrimSpace(os.Getenv("BACKUP_PG_RESTORE_PATH"))
	if c.RestorePath == "" {
		c.RestorePath = "pg_restore"
		if filepath.IsAbs(c.DumpPath) {
			c.RestorePath = filepath.Join(filepath.Dir(c.DumpPath), "pg_restore")
		}
	}
	if raw := os.Getenv("BACKUP_MAX_SIZE_MB"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 16 || n > 2048 {
			return c, fmt.Errorf("BACKUP_MAX_SIZE_MB harus 16 sampai 2048")
		}
		c.MaxBytes = int64(n) << 20
	}
	return c, nil
}

// An optional private file keeps key material outside the application checkout.
// Only these two keys are read; it cannot replace DB/R2 settings or enable backups.
func keySettings() (string, string, error) {
	recipient := strings.TrimSpace(os.Getenv("BACKUP_AGE_RECIPIENT"))
	storageKey := strings.TrimSpace(os.Getenv("BACKUP_R2_SSE_KEY"))
	name := strings.TrimSpace(os.Getenv("BACKUP_KEYS_FILE"))
	if name == "" {
		return recipient, storageKey, nil
	}
	if recipient != "" || storageKey != "" {
		return "", "", fmt.Errorf("gunakan BACKUP_KEYS_FILE atau kunci environment langsung, jangan keduanya")
	}
	if !filepath.IsAbs(name) {
		return "", "", fmt.Errorf("BACKUP_KEYS_FILE harus berupa path absolut")
	}
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 16384 {
		return "", "", fmt.Errorf("BACKUP_KEYS_FILE harus file privat biasa (izin 0600 atau 0400), bukan symlink")
	}
	values, err := godotenv.Read(name)
	if err != nil {
		return "", "", fmt.Errorf("BACKUP_KEYS_FILE tidak dapat dibaca sebagai konfigurasi")
	}
	return strings.TrimSpace(values["BACKUP_AGE_RECIPIENT"]), strings.TrimSpace(values["BACKUP_R2_SSE_KEY"]), nil
}

// NextSunday and LatestSunday use civil time, not a fixed UTC offset.
func NextSunday(now time.Time) time.Time { return LatestSunday(now).AddDate(0, 0, 7) }
func LatestSunday(now time.Time) time.Time {
	local := now.In(apptime.Jakarta)
	day := time.Date(local.Year(), local.Month(), local.Day(), 2, 0, 0, 0, apptime.Jakarta).AddDate(0, 0, -int(local.Weekday()))
	if day.After(local) {
		day = day.AddDate(0, 0, -7)
	}
	return day
}
