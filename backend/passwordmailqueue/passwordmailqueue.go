package passwordmailqueue

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sso-backend/internal/apptime"
	"sso-backend/models"
	"sso-backend/utils"
)

const (
	statusPending      = "pending"
	statusProcessing   = "processing"
	statusDelivered    = "delivered"
	statusDead         = "dead"
	statusSuperseded   = "superseded"
	defaultMaxAttempts = 8
	defaultConcurrency = 2
	maxRecoveryDelay   = 30 * time.Second
	changedEmailTTL    = 24 * time.Hour
)

var config struct {
	sync.RWMutex
	maxAttempts int
	concurrency int
}

var wakeDispatcher = make(chan struct{}, 1)
var errResetNoLongerActive = errors.New("reset token is no longer active")

func Configure() error {
	if err := utils.ValidatePasswordEmailEncryptionConfiguration(); err != nil {
		return fmt.Errorf("password email encryption: %w", err)
	}
	maxAttempts, err := optionalEnvInt("MAIL_QUEUE_MAX_ATTEMPTS", defaultMaxAttempts, 3, 20)
	if err != nil {
		return err
	}
	concurrency, err := optionalEnvInt("MAIL_QUEUE_CONCURRENCY", defaultConcurrency, 1, 8)
	if err != nil {
		return err
	}
	config.Lock()
	config.maxAttempts = maxAttempts
	config.concurrency = concurrency
	config.Unlock()
	return nil
}

func optionalEnvInt(name string, fallback, minimum, maximum int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be between %d and %d", name, minimum, maximum)
	}
	return value, nil
}

// EnqueueReset menyimpan token reset mentah hanya sebagai ciphertext yang
// terikat ke ID outbox. Tabel token reset tetap hanya menyimpan SHA-256.
func EnqueueReset(tx *gorm.DB, user models.User, token models.PasswordResetToken, rawToken string) error {
	if tx == nil || user.ID == "" || token.ID == "" || token.UserID != user.ID || rawToken == "" {
		return errors.New("user, reset token, and plaintext token are required")
	}
	if utils.HashToken(rawToken) != token.TokenHash {
		return errors.New("reset token hash mismatch")
	}
	now := apptime.Now()
	if !token.ExpiresAt.After(now) {
		return errors.New("reset token expiry must be in the future")
	}
	id, err := randomUUID()
	if err != nil {
		return err
	}
	encrypted, err := utils.EncryptPasswordEmailPayload(id, rawToken)
	if err != nil {
		return err
	}
	resetTokenID := token.ID
	return tx.Create(&models.PasswordEmailOutbox{
		ID: id, UserID: user.ID, EventType: models.PasswordEmailResetLink,
		ResetTokenID: &resetTokenID, EncryptedPayload: encrypted,
		EventAt: now, IPAddress: token.RequestedIPAddress, ExpiresAt: token.ExpiresAt,
		Status: statusPending, NextAttemptAt: now,
	}).Error
}

// EnqueuePasswordChanged mengantrekan pemberitahuan yang tidak membawa
// kredensial. Masa retry dibatasi agar notifikasi lama tidak datang terlambat.
func EnqueuePasswordChanged(tx *gorm.DB, user models.User, changedAt time.Time, ipAddress string, currentSessionKept bool) error {
	if tx == nil || user.ID == "" {
		return errors.New("user is required")
	}
	if changedAt.IsZero() {
		changedAt = apptime.Now()
	}
	id, err := randomUUID()
	if err != nil {
		return err
	}
	return tx.Create(&models.PasswordEmailOutbox{
		ID: id, UserID: user.ID, EventType: models.PasswordEmailChanged,
		EventAt: changedAt, IPAddress: strings.TrimSpace(ipAddress),
		CurrentSessionKept: currentSessionKept,
		ExpiresAt:          changedAt.Add(changedEmailTTL), Status: statusPending,
		NextAttemptAt: apptime.Now(),
	}).Error
}

// RevokeUserResetAccess membatalkan seluruh tautan reset aktif dan membuang
// ciphertext dari pekerjaan yang belum diambil worker.
func RevokeUserResetAccess(tx *gorm.DB, userID string, now time.Time) error {
	if tx == nil || strings.TrimSpace(userID) == "" {
		return errors.New("database and user ID are required")
	}
	if now.IsZero() {
		now = apptime.Now()
	}
	if err := tx.Model(&models.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error; err != nil {
		return err
	}
	return tx.Model(&models.PasswordEmailOutbox{}).
		Where("user_id = ? AND event_type = ? AND status = ?", userID, models.PasswordEmailResetLink, statusPending).
		Updates(map[string]any{
			"status": statusSuperseded, "encrypted_payload": "", "locked_until": nil,
			"last_error": "superseded by a newer security event",
		}).Error
}

func Notify() {
	select {
	case wakeDispatcher <- struct{}{}:
	default:
	}
}

func Start(ctx context.Context, db *gorm.DB) {
	d := &dispatcher{db: db}
	go d.run(ctx)
}

type dispatcher struct {
	db *gorm.DB
}

func (d *dispatcher) run(ctx context.Context) {
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()
	for {
		if err := d.dispatchAvailable(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("password email dispatcher: %v", err)
		}
		delay, err := d.nextWakeDelay()
		if err != nil {
			log.Printf("password email scheduler: %v", err)
			delay = maxRecoveryDelay
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-wakeDispatcher:
			timer.Stop()
		case <-timer.C:
		case <-cleanupTicker.C:
			timer.Stop()
			cutoff := apptime.Now().Add(-7 * 24 * time.Hour)
			if err := d.db.Where("status IN ? AND updated_at < ?", []string{statusDelivered, statusDead, statusSuperseded}, cutoff).
				Delete(&models.PasswordEmailOutbox{}).Error; err != nil {
				log.Printf("password email cleanup: %v", err)
			}
			if err := d.db.Where("expires_at < ?", cutoff).Delete(&models.PasswordResetToken{}).Error; err != nil {
				log.Printf("password reset token cleanup: %v", err)
			}
		}
	}
}

func (d *dispatcher) dispatchAvailable(ctx context.Context) error {
	config.RLock()
	concurrency := config.concurrency
	config.RUnlock()
	if concurrency < 1 {
		concurrency = defaultConcurrency
	}
	var wg sync.WaitGroup
	errCh := make(chan error, concurrency)
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := d.dispatchBatch(ctx); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *dispatcher) dispatchBatch(ctx context.Context) error {
	for i := 0; i < 100; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		item, ok, err := d.claimOne()
		if err != nil || !ok {
			return err
		}
		d.deliver(item)
	}
	return nil
}

func (d *dispatcher) nextWakeDelay() (time.Duration, error) {
	var next sql.NullTime
	err := d.db.Raw(`
		SELECT MIN(
			CASE WHEN status = ?
				THEN GREATEST(next_attempt_at, COALESCE(locked_until, next_attempt_at))
				ELSE next_attempt_at
			END
		)
		FROM password_email_outboxes
		WHERE status IN (?, ?)
	`, statusProcessing, statusPending, statusProcessing).Scan(&next).Error
	if err != nil || !next.Valid {
		return maxRecoveryDelay, err
	}
	delay := time.Until(next.Time.UTC())
	if delay < 100*time.Millisecond {
		return 100 * time.Millisecond, nil
	}
	if delay > maxRecoveryDelay {
		return maxRecoveryDelay, nil
	}
	return delay, nil
}

func (d *dispatcher) claimOne() (models.PasswordEmailOutbox, bool, error) {
	var item models.PasswordEmailOutbox
	now := apptime.Now()
	err := d.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("next_attempt_at <= ? AND (status = ? OR (status = ? AND locked_until <= ?))", now, statusPending, statusProcessing, now).
			Order("created_at ASC").Limit(1).Find(&item)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.First(&item.User, "id = ?", item.UserID).Error; err != nil {
			return err
		}
		return tx.Model(&item).Updates(map[string]any{"status": statusProcessing, "locked_until": now.Add(time.Minute)}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.PasswordEmailOutbox{}, false, nil
	}
	return item, err == nil, err
}

func (d *dispatcher) deliver(item models.PasswordEmailOutbox) {
	now := apptime.Now()
	if !item.ExpiresAt.After(now) {
		d.finish(item.ID, statusSuperseded, nil, "password email is no longer active")
		return
	}
	var err error
	switch item.EventType {
	case models.PasswordEmailResetLink:
		err = d.deliverReset(item, now)
	case models.PasswordEmailChanged:
		err = utils.SendPasswordChangedEmail(item.User.Email, item.User.Name, item.EventAt, item.IPAddress, item.CurrentSessionKept)
	default:
		d.finish(item.ID, statusDead, nil, "unsupported password email event")
		return
	}
	if err != nil {
		if errors.Is(err, errResetNoLongerActive) {
			d.finish(item.ID, statusSuperseded, nil, err.Error())
			return
		}
		d.fail(item, err)
		return
	}
	deliveredAt := apptime.Now()
	d.finish(item.ID, statusDelivered, &deliveredAt, "")
}

func (d *dispatcher) deliverReset(item models.PasswordEmailOutbox, now time.Time) error {
	var eligible int64
	if err := d.db.Model(&models.User{}).Where("id = ? AND is_active = true AND email_verified_at IS NOT NULL", item.UserID).Count(&eligible).Error; err != nil {
		return err
	}
	if eligible != 1 {
		return errResetNoLongerActive
	}
	if item.ResetTokenID == nil || strings.TrimSpace(*item.ResetTokenID) == "" {
		return errors.New("reset token reference is missing")
	}
	var token models.PasswordResetToken
	if err := d.db.Where("id = ? AND user_id = ? AND used_at IS NULL AND revoked_at IS NULL AND expires_at > ?", *item.ResetTokenID, item.UserID, now).
		First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errResetNoLongerActive
		}
		return err
	}
	rawToken, err := utils.DecryptPasswordEmailPayload(item.ID, item.EncryptedPayload)
	if err != nil {
		return err
	}
	if utils.HashToken(rawToken) != token.TokenHash {
		return errors.New("reset token ciphertext does not match stored hash")
	}
	resetURL, err := buildResetURL(rawToken)
	if err != nil {
		return err
	}
	return utils.SendPasswordResetEmail(item.User.Email, item.User.Name, resetURL, time.Until(item.ExpiresAt))
}

func buildResetURL(rawToken string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(os.Getenv("FRONTEND_PUBLIC_URL")))
	if err != nil || base.Host == "" || base.User != nil || (base.Scheme != "https" && !(base.Scheme == "http" && (base.Hostname() == "localhost" || base.Hostname() == "127.0.0.1" || base.Hostname() == "::1"))) {
		return "", errors.New("FRONTEND_PUBLIC_URL is invalid")
	}
	base.Path = "/reset-password"
	base.RawQuery = ""
	base.Fragment = ""
	query := base.Query()
	query.Set("token", rawToken)
	base.RawQuery = query.Encode()
	return base.String(), nil
}

func (d *dispatcher) finish(id, status string, deliveredAt *time.Time, lastError string) {
	if err := d.db.Model(&models.PasswordEmailOutbox{}).
		Where("id = ? AND status = ?", id, statusProcessing).
		Updates(map[string]any{
			"status": status, "delivered_at": deliveredAt, "locked_until": nil,
			"last_error": lastError, "encrypted_payload": "",
		}).Error; err != nil {
		log.Printf("password email %s status update failed: %v", id, err)
	}
}

func (d *dispatcher) fail(item models.PasswordEmailOutbox, deliveryErr error) {
	attempts := item.Attempts + 1
	delay := retryDelay(attempts)
	config.RLock()
	maxAttempts := config.maxAttempts
	config.RUnlock()
	if maxAttempts < 1 {
		maxAttempts = defaultMaxAttempts
	}
	status := statusPending
	nextAttemptAt := apptime.Now().Add(delay)
	updates := map[string]any{
		"status": status, "attempts": attempts, "next_attempt_at": nextAttemptAt,
		"locked_until": nil,
	}
	if attempts >= maxAttempts || !nextAttemptAt.Before(item.ExpiresAt) {
		updates["status"] = statusDead
		updates["encrypted_payload"] = ""
	}
	message := deliveryErr.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	updates["last_error"] = message
	if err := d.db.Model(&models.PasswordEmailOutbox{}).
		Where("id = ? AND status = ?", item.ID, statusProcessing).
		Updates(updates).Error; err != nil {
		log.Printf("password email %s failure update failed: %v", item.ID, err)
	}
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return time.Duration(1<<(attempt-1)) * 2 * time.Second
}

func randomUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
