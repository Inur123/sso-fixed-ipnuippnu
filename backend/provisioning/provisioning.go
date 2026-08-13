package provisioning

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sso-backend/models"
)

const (
	EventAssigned      = "user.assigned"
	EventUnassigned    = "user.unassigned"
	EventUpdated       = "user.updated"
	statusPending      = "pending"
	statusProcessing   = "processing"
	statusDelivered    = "delivered"
	statusDead         = "dead"
	defaultMaxAttempts = 12
	defaultConcurrency = 4
	maxRecoveryDelay   = 30 * time.Second
)

type target struct {
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

type eventUser struct {
	Subject       string `json:"sub"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Avatar        string `json:"picture,omitempty"`
}

type eventPayload struct {
	SpecVersion     string    `json:"specversion"`
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	Source          string    `json:"source"`
	Subject         string    `json:"subject"`
	Time            time.Time `json:"time"`
	DataContentType string    `json:"datacontenttype"`
	Data            eventData `json:"data"`
}

type eventData struct {
	Audience string    `json:"audience"`
	User     eventUser `json:"user"`
}

var config struct {
	sync.RWMutex
	targets     map[string]target
	issuer      string
	maxAttempts int
	concurrency int
}

// wakeDispatcher menggabungkan banyak notifikasi menjadi satu sinyal. Handler
// API hanya menulis outbox dalam transaksi; pengiriman HTTP tetap asinkron.
var wakeDispatcher = make(chan struct{}, 1)

// Configure membaca target per OAuth client. Jika env kosong, provisioning
// dinonaktifkan tanpa mengubah perilaku OAuth/assignment yang sudah ada.
func Configure() error {
	raw := strings.TrimSpace(os.Getenv("PROVISIONING_TARGETS_JSON"))
	targets := map[string]target{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &targets); err != nil {
			return fmt.Errorf("PROVISIONING_TARGETS_JSON must be a JSON object keyed by client ID: %w", err)
		}
	}
	production := strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
	for clientID, item := range targets {
		parsed, err := url.Parse(strings.TrimSpace(item.URL))
		if strings.TrimSpace(clientID) == "" || err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return fmt.Errorf("provisioning target for %q must contain an absolute URL without query or fragment", clientID)
		}
		if production && parsed.Scheme != "https" {
			return fmt.Errorf("provisioning target for %q must use HTTPS in production", clientID)
		}
		if !production && parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("provisioning target for %q must use HTTP or HTTPS", clientID)
		}
		if len(item.Secret) < 32 {
			return fmt.Errorf("provisioning secret for %q must contain at least 32 characters", clientID)
		}
		item.URL = parsed.String()
		targets[clientID] = item
	}
	maxAttempts := defaultMaxAttempts
	if rawMax := strings.TrimSpace(os.Getenv("PROVISIONING_MAX_ATTEMPTS")); rawMax != "" {
		parsed, err := strconv.Atoi(rawMax)
		if err != nil || parsed < 3 || parsed > 30 {
			return errors.New("PROVISIONING_MAX_ATTEMPTS must be between 3 and 30")
		}
		maxAttempts = parsed
	}
	concurrency := defaultConcurrency
	if rawConcurrency := strings.TrimSpace(os.Getenv("PROVISIONING_CONCURRENCY")); rawConcurrency != "" {
		parsed, err := strconv.Atoi(rawConcurrency)
		if err != nil || parsed < 1 || parsed > 16 {
			return errors.New("PROVISIONING_CONCURRENCY must be between 1 and 16")
		}
		concurrency = parsed
	}
	config.Lock()
	config.targets = targets
	config.issuer = strings.TrimRight(strings.TrimSpace(os.Getenv("BACKEND_PUBLIC_URL")), "/")
	config.maxAttempts = maxAttempts
	config.concurrency = concurrency
	config.Unlock()
	return nil
}

// Notify membangunkan dispatcher setelah transaksi outbox berhasil commit.
// Sinyal bersifat best-effort karena recovery timer tetap menangani crash atau
// event yang ditulis instance lain.
func Notify() {
	select {
	case wakeDispatcher <- struct{}{}:
	default:
	}
}

func HasTarget(clientID string) bool {
	config.RLock()
	defer config.RUnlock()
	_, ok := config.targets[clientID]
	return ok
}

// Enqueue menyimpan event di transaksi yang sama dengan perubahan assignment.
func Enqueue(tx *gorm.DB, eventType string, client models.OAuthClient, user models.User) error {
	if !HasTarget(client.ID) {
		return nil
	}
	eventID, err := randomUUID()
	if err != nil {
		return err
	}
	return enqueue(tx, eventID, nil, eventType, client, user)
}

// EnqueueForUser mengirim perubahan lifecycle akun ke semua aplikasi
// provisioning tempat pengguna masih memiliki assignment aktif.
func EnqueueForUser(tx *gorm.DB, eventType string, user models.User) error {
	config.RLock()
	clientIDs := make([]string, 0, len(config.targets))
	for clientID := range config.targets {
		clientIDs = append(clientIDs, clientID)
	}
	config.RUnlock()
	if len(clientIDs) == 0 {
		return nil
	}
	var assignments []models.OAuthClientAssignment
	if err := tx.Preload("Client").Where("client_id IN ? AND user_id = ? AND is_active = ?", clientIDs, user.ID, true).
		Find(&assignments).Error; err != nil {
		return err
	}
	for _, assignment := range assignments {
		if err := Enqueue(tx, eventType, assignment.Client, user); err != nil {
			return err
		}
	}
	return nil
}

func enqueue(tx *gorm.DB, eventID string, dedupeKey *string, eventType string, client models.OAuthClient, user models.User) error {
	now := time.Now().UTC()
	config.RLock()
	issuer := config.issuer
	config.RUnlock()
	payload := eventPayload{
		SpecVersion: "1.0", ID: eventID, Type: eventType, Source: issuer,
		Subject: user.ID, Time: now, DataContentType: "application/json",
		Data: eventData{Audience: client.ID,
			User: eventUser{Subject: user.ID, Name: user.Name, Email: user.Email,
				EmailVerified: user.EmailVerifiedAt != nil, Avatar: user.Avatar}},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "dedupe_key"}}, DoNothing: true}).
		Create(&models.ProvisioningOutbox{ID: eventID, ClientID: client.ID,
			EventType: eventType, Subject: user.ID, Payload: string(encoded),
			Status: statusPending, NextAttemptAt: now, DedupeKey: dedupeKey}).Error
}

// ReconcileExistingAssignments membuat snapshot satu kali untuk assignment yang
// sudah ada sebelum target provisioning dikonfigurasi. Unique dedupe key membuat
// fungsi ini aman ketika beberapa instance backend start bersamaan.
func ReconcileExistingAssignments(db *gorm.DB) error {
	config.RLock()
	clientIDs := make([]string, 0, len(config.targets))
	for clientID := range config.targets {
		clientIDs = append(clientIDs, clientID)
	}
	config.RUnlock()
	for _, clientID := range clientIDs {
		var client models.OAuthClient
		if err := db.First(&client, "id = ?", clientID).Error; err != nil {
			return fmt.Errorf("configured provisioning client %s: %w", clientID, err)
		}
		if client.AccessPolicy != models.AccessPolicyAssignedOnly {
			return fmt.Errorf("configured provisioning client %s must use assigned_only access policy", clientID)
		}
		var assignments []models.OAuthClientAssignment
		if err := db.Preload("User").Where("client_id = ? AND is_active = ?", clientID, true).Find(&assignments).Error; err != nil {
			return err
		}
		for _, assignment := range assignments {
			eventID, err := randomUUID()
			if err != nil {
				return err
			}
			key := "initial:" + clientID + ":" + assignment.UserID
			if err := enqueue(db, eventID, &key, EventAssigned, client, assignment.User); err != nil {
				return err
			}
		}
	}
	return nil
}

type dispatcher struct {
	db     *gorm.DB
	client *http.Client
}

// Start menjalankan dispatcher non-blocking. SKIP LOCKED memungkinkan beberapa
// instance backend berjalan bersamaan tanpa mengirim event yang sama paralel.
func Start(ctx context.Context, db *gorm.DB) {
	config.RLock()
	enabled := len(config.targets) > 0
	config.RUnlock()
	if !enabled {
		log.Println("provisioning dispatcher disabled: no targets configured")
		return
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
	d := &dispatcher{db: db, client: &http.Client{Transport: transport, Timeout: 15 * time.Second}}
	go d.run(ctx)
}

func (d *dispatcher) run(ctx context.Context) {
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()
	for {
		if err := d.dispatchAvailable(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("provisioning dispatcher: %v", err)
		}
		delay, err := d.nextWakeDelay()
		if err != nil {
			log.Printf("provisioning scheduler: %v", err)
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
			// Event terkirim tidak diperlukan selamanya. Event gagal/dead tetap
			// dipertahankan untuk investigasi dan retry manual.
			if err := d.db.Where("status = ? AND dedupe_key IS NULL AND delivered_at < ?", statusDelivered, time.Now().UTC().Add(-30*24*time.Hour)).
				Delete(&models.ProvisioningOutbox{}).Error; err != nil {
				log.Printf("provisioning cleanup: %v", err)
			}
		}
	}
}

func (d *dispatcher) dispatchAvailable(ctx context.Context) error {
	config.RLock()
	concurrency := config.concurrency
	config.RUnlock()
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
		d.deliver(ctx, item)
	}
	return nil
}

// nextWakeDelay menghindari polling tetap. Event retry membangunkan worker saat
// jatuh tempo; antrean kosong hanya diperiksa setiap 30 detik sebagai recovery
// jika proses mati tepat setelah commit atau event ditulis instance lain.
func (d *dispatcher) nextWakeDelay() (time.Duration, error) {
	var next *time.Time
	err := d.db.Raw(`
		SELECT MIN(
			CASE WHEN status = ?
				THEN GREATEST(next_attempt_at, COALESCE(locked_until, next_attempt_at))
				ELSE next_attempt_at
			END
		)
		FROM provisioning_outboxes
		WHERE status IN (?, ?)
	`, statusProcessing, statusPending, statusProcessing).Scan(&next).Error
	if err != nil || next == nil {
		return maxRecoveryDelay, err
	}
	delay := time.Until(next.UTC())
	if delay < 100*time.Millisecond {
		return 100 * time.Millisecond, nil
	}
	if delay > maxRecoveryDelay {
		return maxRecoveryDelay, nil
	}
	return delay, nil
}

func (d *dispatcher) claimOne() (models.ProvisioningOutbox, bool, error) {
	var item models.ProvisioningOutbox
	now := time.Now().UTC()
	err := d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("next_attempt_at <= ? AND (status = ? OR (status = ? AND locked_until <= ?))", now, statusPending, statusProcessing, now).
			Order("created_at ASC").First(&item).Error; err != nil {
			return err
		}
		return tx.Model(&item).Updates(map[string]any{"status": statusProcessing, "locked_until": now.Add(30 * time.Second)}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ProvisioningOutbox{}, false, nil
	}
	return item, err == nil, err
}

func (d *dispatcher) deliver(ctx context.Context, item models.ProvisioningOutbox) {
	config.RLock()
	target, ok := config.targets[item.ClientID]
	config.RUnlock()
	if !ok {
		d.fail(item, errors.New("provisioning target is no longer configured"))
		return
	}
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.URL, bytes.NewBufferString(item.Payload))
	if err == nil {
		req.Header.Set("Content-Type", "application/cloudevents+json")
		req.Header.Set("User-Agent", "IPNU-IPPNU-SSO-Provisioner/1.0")
		req.Header.Set("X-SSO-Event-ID", item.ID)
		req.Header.Set("X-SSO-Timestamp", timestamp)
		req.Header.Set("X-SSO-Signature", "v1="+Sign(target.Secret, timestamp, []byte(item.Payload)))
		var response *http.Response
		response, err = d.client.Do(req)
		if response != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
			_ = response.Body.Close()
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				err = fmt.Errorf("target returned HTTP %d", response.StatusCode)
			}
		}
	}
	if err != nil {
		d.fail(item, err)
		return
	}
	now := time.Now().UTC()
	if err := d.db.Model(&models.ProvisioningOutbox{}).Where("id = ? AND status = ?", item.ID, statusProcessing).
		Updates(map[string]any{"status": statusDelivered, "delivered_at": now, "locked_until": nil, "last_error": "", "payload": "{}"}).Error; err != nil {
		log.Printf("provisioning event %s delivered but status update failed: %v", item.ID, err)
	}
}

func (d *dispatcher) fail(item models.ProvisioningOutbox, deliveryErr error) {
	attempts := item.Attempts + 1
	config.RLock()
	maxAttempts := config.maxAttempts
	config.RUnlock()
	status := statusPending
	if attempts >= maxAttempts {
		status = statusDead
	}
	message := deliveryErr.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	if err := d.db.Model(&models.ProvisioningOutbox{}).Where("id = ? AND status = ?", item.ID, statusProcessing).
		Updates(map[string]any{"status": status, "attempts": attempts,
			"next_attempt_at": time.Now().UTC().Add(retryDelay(attempts)), "locked_until": nil, "last_error": message}).Error; err != nil {
		log.Printf("provisioning event %s failure update failed: %v", item.ID, err)
	}
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 9 {
		attempt = 9
	}
	return time.Duration(1<<(attempt-1)) * 5 * time.Second
}

func Sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
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
