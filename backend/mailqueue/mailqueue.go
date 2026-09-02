package mailqueue

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
)

var config struct {
	sync.RWMutex
	maxAttempts int
	concurrency int
}

// wakeDispatcher mempercepat pengiriman setelah transaksi commit. Recovery
// timer tetap menjadi jaring pengaman saat proses restart atau sinyal hilang.
var wakeDispatcher = make(chan struct{}, 1)

func Configure() error {
	if err := utils.ValidateEmailOTPEncryptionConfiguration(); err != nil {
		return fmt.Errorf("verification email encryption: %w", err)
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

// Enqueue menyimpan pekerjaan email pada transaksi yang sama dengan OTP. OTP
// lama yang belum diproses dibatalkan agar hanya kode terbaru yang dikirim.
func Enqueue(tx *gorm.DB, user models.User, code, codeHash string, expiresAt time.Time) error {
	if tx == nil || user.ID == "" || len(code) != 6 || codeHash == "" {
		return errors.New("user, OTP, and hash are required")
	}
	id, err := randomUUID()
	if err != nil {
		return err
	}
	encryptedCode, err := utils.EncryptEmailOTP(id, code)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if !expiresAt.After(now) {
		return errors.New("OTP expiry must be in the future")
	}
	if err := tx.Model(&models.VerificationEmailOutbox{}).
		Where("user_id = ? AND status = ?", user.ID, statusPending).
		Updates(map[string]any{
			"status": statusSuperseded, "encrypted_code": "", "locked_until": nil,
			"last_error": "superseded by a newer OTP",
		}).Error; err != nil {
		return err
	}
	return tx.Create(&models.VerificationEmailOutbox{
		ID: id, UserID: user.ID, CodeHash: codeHash, EncryptedCode: encryptedCode,
		ExpiresAt: expiresAt, Status: statusPending, NextAttemptAt: now,
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
			log.Printf("verification email dispatcher: %v", err)
		}
		delay, err := d.nextWakeDelay()
		if err != nil {
			log.Printf("verification email scheduler: %v", err)
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
			if err := d.db.Where("status IN ? AND updated_at < ?", []string{statusDelivered, statusDead, statusSuperseded}, time.Now().UTC().Add(-7*24*time.Hour)).
				Delete(&models.VerificationEmailOutbox{}).Error; err != nil {
				log.Printf("verification email cleanup: %v", err)
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
		FROM verification_email_outboxes
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

func (d *dispatcher) claimOne() (models.VerificationEmailOutbox, bool, error) {
	var item models.VerificationEmailOutbox
	now := time.Now().UTC()
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
		return models.VerificationEmailOutbox{}, false, nil
	}
	return item, err == nil, err
}

func (d *dispatcher) deliver(item models.VerificationEmailOutbox) {
	now := time.Now().UTC()
	var active int64
	if err := d.db.Model(&models.EmailVerificationOTP{}).
		Where("user_id = ? AND code_hash = ? AND expires_at > ?", item.UserID, item.CodeHash, now).
		Count(&active).Error; err != nil {
		d.fail(item, err)
		return
	}
	if active == 0 || !item.ExpiresAt.After(now) {
		d.finish(item.ID, statusSuperseded, nil, "OTP is no longer active")
		return
	}
	code, err := utils.DecryptEmailOTP(item.ID, item.EncryptedCode)
	if err == nil {
		err = utils.SendVerificationEmail(item.User.Email, item.User.Name, code, time.Until(item.ExpiresAt))
	}
	if err != nil {
		d.fail(item, err)
		return
	}
	deliveredAt := time.Now().UTC()
	d.finish(item.ID, statusDelivered, &deliveredAt, "")
}

func (d *dispatcher) finish(id, status string, deliveredAt *time.Time, lastError string) {
	if err := d.db.Model(&models.VerificationEmailOutbox{}).
		Where("id = ? AND status = ?", id, statusProcessing).
		Updates(map[string]any{
			"status": status, "delivered_at": deliveredAt, "locked_until": nil,
			"last_error": lastError, "encrypted_code": "",
		}).Error; err != nil {
		log.Printf("verification email %s status update failed: %v", id, err)
	}
}

func (d *dispatcher) fail(item models.VerificationEmailOutbox, deliveryErr error) {
	attempts := item.Attempts + 1
	delay := retryDelay(attempts)
	config.RLock()
	maxAttempts := config.maxAttempts
	config.RUnlock()
	if maxAttempts < 1 {
		maxAttempts = defaultMaxAttempts
	}
	status := statusPending
	nextAttemptAt := time.Now().UTC().Add(delay)
	updates := map[string]any{
		"status": status, "attempts": attempts, "next_attempt_at": nextAttemptAt,
		"locked_until": nil,
	}
	if attempts >= maxAttempts || !nextAttemptAt.Before(item.ExpiresAt) {
		updates["status"] = statusDead
		updates["encrypted_code"] = ""
	}
	message := deliveryErr.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	updates["last_error"] = message
	if err := d.db.Model(&models.VerificationEmailOutbox{}).
		Where("id = ? AND status = ?", item.ID, statusProcessing).
		Updates(updates).Error; err != nil {
		log.Printf("verification email %s failure update failed: %v", item.ID, err)
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
