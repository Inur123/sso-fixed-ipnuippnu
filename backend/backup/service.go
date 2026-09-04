package backup

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"filippo.io/age"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sso-backend/internal/apptime"
	"sso-backend/models"
)

const (
	Queued                 = "queued"
	Running                = "running"
	Succeeded              = "succeeded"
	Failed                 = "failed"
	Pruning                = "pruning"
	Deleted                = "deleted"
	ActionRequested        = "backup.requested"
	ActionCompleted        = "backup.completed"
	ActionFailed           = "backup.failed"
	ActionDownloaded       = "backup.downloaded"
	ActionPruned           = "backup.pruned"
	workerLock       int64 = 7182920461971
)

var ErrUnavailable = errors.New("backup belum siap")
var ErrBusy = errors.New("backup lain sedang menunggu atau berjalan")

type Service struct {
	db            *gorm.DB
	config        Config
	store         Store
	dumper        Dumper
	issue         string
	wake          chan struct{}
	mu            sync.Mutex
	downloadMu    sync.Mutex
	downloadIssue string
	now           func() time.Time
}

func New(db *gorm.DB, c Config, store Store, dumper Dumper) *Service {
	return &Service{db: db, config: c, store: store, dumper: dumper, wake: make(chan struct{}, 1), now: apptime.Now}
}

func FromEnvironment(db *gorm.DB) *Service {
	c, err := ConfigFromEnv()
	s := New(db, c, nil, nil)
	if err != nil {
		s.issue = err.Error()
		return s
	}
	if !c.Enabled {
		s.issue = "Aktifkan BACKUP_ENABLED dan siapkan kunci backup di environment backend."
		return s
	}
	s.store = NewR2Store(c)
	s.dumper = NewPostgresDumper(c)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.checkVersion(ctx); err != nil {
		s.issue = err.Error()
	}
	if err := s.checkSQLDownload(ctx); err != nil {
		s.downloadIssue = err.Error()
	}
	return s
}

func (s *Service) Ready() bool {
	return s.config.Enabled && s.issue == "" && s.store != nil && s.dumper != nil
}
func (s *Service) Notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Run catches up at startup; no in-memory-only cron job is used. A bounded
// poll also catches changes after recovery without requiring a logged-in admin.
func (s *Service) Run(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if !s.Ready() {
			return
		}
		timer := time.NewTicker(time.Minute)
		defer timer.Stop()
		for {
			if err := s.Tick(ctx); err != nil && ctx.Err() == nil {
				log.Print("database backup scheduler could not finish; will retry")
			}
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
			case <-timer.C:
			}
		}
	}()
	return done
}

func (s *Service) ensureSchedule(tx *gorm.DB, now time.Time) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.DatabaseBackupSchedule{Namespace: s.config.Namespace, NextRunAt: NextSunday(now)}).Error
}

func (s *Service) Request(ctx context.Context, actorID, ip string) (models.DatabaseBackup, error) {
	var job models.DatabaseBackup
	if !s.Ready() {
		return job, ErrUnavailable
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := s.now()
		if err := s.ensureSchedule(tx, now); err != nil {
			return err
		}
		var schedule models.DatabaseBackupSchedule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&schedule, "namespace = ?", s.config.Namespace).Error; err != nil {
			return err
		}
		found := tx.Where("namespace = ? AND status IN ?", s.config.Namespace, []string{Queued, Running, Failed}).Order("created_at DESC").Limit(1).Find(&job)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected > 0 {
			if job.Status != Failed {
				return ErrBusy
			}
			if err := tx.Model(&job).Updates(map[string]any{"status": Queued, "phase": "waiting", "next_attempt_at": now}).Error; err != nil {
				return err
			}
		} else {
			var err error
			job, err = s.newJob("manual", &actorID, nil, now)
			if err != nil {
				return err
			}
			if err = tx.Create(&job).Error; err != nil {
				return err
			}
		}
		return recordAudit(tx, ActionRequested, job.ID, &actorID, ip, "Backup database diminta melalui super admin.")
	})
	if err == nil {
		s.Notify()
	}
	return job, err
}

func (s *Service) newJob(kind string, actor *string, slot *time.Time, now time.Time) (models.DatabaseBackup, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return models.DatabaseBackup{}, err
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	id := fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
	return models.DatabaseBackup{ID: id, Namespace: s.config.Namespace, Kind: kind, Status: Queued, Phase: "waiting", RequestedBy: actor, ScheduledFor: slot, ObjectKey: s.config.Namespace + id + ".dump.age", Recipient: s.config.Recipient.String(), NextAttemptAt: &now, CreatedAt: now}, nil
}

func (s *Service) Tick(ctx context.Context) error {
	if !s.Ready() {
		return ErrUnavailable
	}
	if !s.mu.TryLock() {
		return nil
	}
	defer s.mu.Unlock()
	// Bound maintenance and database operations too, not only the export.
	ctx, cancelTick := context.WithTimeout(ctx, s.config.Timeout)
	defer cancelTick()
	// Session-level advisory lock is held on ONE dedicated connection throughout
	// export/upload/pruning. No long transaction is held on this connection;
	// pg_dump owns a separate, consistent read snapshot while exporting.
	return s.db.WithContext(ctx).Connection(func(db *gorm.DB) error {
		// Connection hands back a mutable GORM statement; start fresh statements
		// while retaining that same dedicated SQL connection for the advisory lock.
		db = db.Session(&gorm.Session{NewDB: true})
		var locked bool
		if err := db.Raw("SELECT pg_try_advisory_lock(?)", workerLock).Scan(&locked).Error; err != nil {
			return err
		}
		if !locked {
			return nil
		}
		defer func() {
			unlock, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var released bool
			err := db.WithContext(unlock).Raw("SELECT pg_advisory_unlock(?)", workerLock).Scan(&released).Error
			if err != nil || !released {
				// Never return a connection that might still own a session lock to
				// the pool. A broken connection also releases its server-side lock.
				if conn, ok := db.Statement.ConnPool.(*sql.Conn); ok {
					_ = conn.Raw(func(any) error { return driver.ErrBadConn })
				}
			}
		}()
		now := s.now()
		// Owning the lock proves no surviving worker owns a previously running job.
		if err := db.Model(&models.DatabaseBackup{}).Where("namespace = ? AND status = ?", s.config.Namespace, Running).
			Updates(map[string]any{"status": Failed, "phase": "waiting_retry", "last_error": "Proses sebelumnya terputus. Backup akan dibuat ulang.", "next_attempt_at": now}).Error; err != nil {
			return err
		}
		if err := s.prune(ctx, db); err != nil {
			return err
		}
		var job models.DatabaseBackup
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := s.ensureSchedule(tx, now); err != nil {
				return err
			}
			var schedule models.DatabaseBackupSchedule
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&schedule, "namespace = ?", s.config.Namespace).Error; err != nil {
				return err
			}
			pending := tx.Where("namespace = ? AND status IN ?", s.config.Namespace, []string{Queued, Failed}).Order("created_at ASC").Limit(1).Find(&job)
			if pending.Error != nil {
				return pending.Error
			}
			if !schedule.NextRunAt.After(now) {
				slot := LatestSunday(now)
				if pending.RowsAffected == 0 {
					var err error
					job, err = s.newJob("scheduled", nil, &slot, now)
					if err != nil {
						return err
					}
					if err := tx.Create(&job).Error; err != nil {
						return err
					}
				} else {
					// Several missed weeks collapse into the latest snapshot, including
					// a pending manual job. We never enqueue one job per missed week.
					job.ScheduledFor = &slot
					job.NextAttemptAt = &now
					if err := tx.Model(&job).Updates(map[string]any{"scheduled_for": slot, "next_attempt_at": now}).Error; err != nil {
						return err
					}
				}
				if err := tx.Model(&schedule).Update("next_run_at", NextSunday(now)).Error; err != nil {
					return err
				}
			}
			if job.ID == "" || (job.NextAttemptAt != nil && job.NextAttemptAt.After(now)) {
				job.ID = ""
				return nil
			}
			job.Attempts++
			job.StartedAt = &now
			job.Status = Running
			return tx.Model(&job).Updates(map[string]any{"status": Running, "phase": "exporting", "started_at": now, "finished_at": nil, "next_attempt_at": nil, "attempts": job.Attempts, "last_error": ""}).Error
		})
		if err != nil || job.ID == "" {
			return err
		}
		work, cancel := context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()
		if err := s.execute(work, db, job); err != nil {
			return s.fail(db, job, err)
		}
		return s.prune(ctx, db)
	})
}

func (s *Service) checkVersion(ctx context.Context) error {
	version, err := s.dumper.Version(ctx)
	if err != nil {
		return err
	}
	var raw string
	if err := s.db.WithContext(ctx).Raw("SHOW server_version_num").Scan(&raw).Error; err != nil {
		return errors.New("versi PostgreSQL belum dapat diperiksa")
	}
	n, _ := strconv.Atoi(raw)
	if version != n/10000 {
		return fmt.Errorf("pg_dump harus major version %d, sama dengan PostgreSQL server", n/10000)
	}
	return nil
}

func (s *Service) execute(ctx context.Context, db *gorm.DB, job models.DatabaseBackup) error {
	if err := s.checkVersion(ctx); err != nil {
		return errors.New("pg_dump tidak tersedia atau versinya tidak sesuai server")
	}
	if !validObjectKey(s.config.Namespace, job.ObjectKey) {
		return errors.New("path backup tidak valid")
	}
	dir, err := os.MkdirTemp("", "sso-db-backup-")
	if err != nil {
		return errors.New("ruang sementara backup tidak tersedia")
	}
	defer os.RemoveAll(dir)
	file, err := os.OpenFile(dir+"/snapshot.dump.age", os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("file sementara backup tidak dapat dibuat")
	}
	defer file.Close()
	hash := sha256.New()
	w, err := age.Encrypt(&limitWriter{writer: io.MultiWriter(file, hash), remaining: s.config.MaxBytes}, s.config.Recipient)
	if err != nil {
		return errors.New("enkripsi backup gagal dimulai")
	}
	if err := s.dumper.Dump(ctx, w); err != nil {
		return errors.New("Export gagal atau melewati batas waktu/ukuran. Periksa PostgreSQL, pg_dump, dan ruang disk.")
	}
	if err := w.Close(); err != nil {
		return errors.New("file backup terenkripsi tidak dapat diselesaikan")
	}
	stat, err := file.Stat()
	if err != nil || stat.Size() < 32 {
		return errors.New("file backup kosong atau tidak valid")
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	// A write through the lock-owning connection fences off a worker whose
	// connection disappeared while pg_dump ran (e.g. a database restart).
	if err := db.WithContext(ctx).Model(&job).Updates(map[string]any{"phase": "uploading", "size_bytes": stat.Size(), "sha256": sum, "recipient": s.config.Recipient.String()}).Error; err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := s.store.Put(ctx, job.ObjectKey, file, stat.Size(), sum); err != nil {
		s.cleanupObject(job.ObjectKey)
		return errors.New("Upload/perlindungan R2 gagal. Periksa izin bucket, koneksi, dan kunci SSE-C.")
	}
	if err := db.WithContext(ctx).Model(&job).Update("phase", "verifying").Error; err != nil {
		return err
	}
	remote, err := s.store.Get(ctx, job.ObjectKey)
	if err != nil {
		return errors.New("Backup terunggah tetapi belum dapat diperiksa ulang; akan dicoba ulang.")
	}
	verify := sha256.New()
	n, copyErr := io.Copy(verify, io.LimitReader(remote.Body, s.config.MaxBytes+1))
	remote.Body.Close()
	if copyErr != nil || n != stat.Size() || remote.Size != stat.Size() || hex.EncodeToString(verify.Sum(nil)) != sum {
		s.cleanupObject(job.ObjectKey)
		return errors.New("Pemeriksaan integritas backup gagal. Backup lama tetap dipertahankan.")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&job).Updates(map[string]any{"status": Succeeded, "phase": "complete", "finished_at": s.now(), "next_attempt_at": nil, "last_error": ""}).Error; err != nil {
			return err
		}
		return recordAudit(tx, ActionCompleted, job.ID, nil, "", "Backup database terenkripsi berhasil diunggah dan diverifikasi.")
	})
}

func (s *Service) cleanupObject(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if validObjectKey(s.config.Namespace, key) {
		_ = s.store.Delete(ctx, key)
	}
}

func (s *Service) fail(db *gorm.DB, job models.DatabaseBackup, cause error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	delay := time.Minute * time.Duration(1<<min(job.Attempts, 6))
	next := s.now().Add(delay)
	// Only our safe phase errors reach the UI; never expose database/S3 errors.
	message := "Backup terputus atau layanan tidak tersedia. Percobaan ulang dijadwalkan."
	if cause != nil {
		switch cause.Error() {
		case "pg_dump tidak tersedia atau versinya tidak sesuai server", "path backup tidak valid", "ruang sementara backup tidak tersedia", "file sementara backup tidak dapat dibuat", "enkripsi backup gagal dimulai", "Export gagal atau melewati batas waktu/ukuran. Periksa PostgreSQL, pg_dump, dan ruang disk.", "file backup terenkripsi tidak dapat diselesaikan", "file backup kosong atau tidak valid", "Upload/perlindungan R2 gagal. Periksa izin bucket, koneksi, dan kunci SSE-C.", "Backup terunggah tetapi belum dapat diperiksa ulang; akan dicoba ulang.", "Pemeriksaan integritas backup gagal. Backup lama tetap dipertahankan.":
			message = cause.Error()
		}
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&job).Updates(map[string]any{"status": Failed, "phase": "waiting_retry", "last_error": message, "next_attempt_at": next}).Error; err != nil {
			return err
		}
		return recordAudit(tx, ActionFailed, job.ID, nil, "", message)
	})
}

func (s *Service) prune(ctx context.Context, db *gorm.DB) error {
	var old []models.DatabaseBackup
	if err := db.Where("namespace = ? AND status = ?", s.config.Namespace, Succeeded).Order("finished_at DESC, id DESC").Offset(Retention).Find(&old).Error; err != nil {
		return err
	}
	var interrupted []models.DatabaseBackup
	if err := db.Where("namespace = ? AND status = ?", s.config.Namespace, Pruning).Find(&interrupted).Error; err != nil {
		return err
	}
	old = append(interrupted, old...)
	for _, job := range old {
		if !validObjectKey(s.config.Namespace, job.ObjectKey) {
			return s.maintenanceFailure(db)
		}
		if err := db.Model(&job).Update("status", Pruning).Error; err != nil {
			return err
		}
		if err := s.store.Delete(ctx, job.ObjectKey); err != nil {
			return s.maintenanceFailure(db)
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&job).Updates(map[string]any{"status": Deleted, "phase": "removed"}).Error; err != nil {
				return err
			}
			return recordAudit(tx, ActionPruned, job.ID, nil, "", "Backup terlama dihapus setelah backup baru terverifikasi; maksimal 10 file disimpan.")
		}); err != nil {
			return err
		}
	}
	if err := db.Model(&models.DatabaseBackupSchedule{}).Where("namespace = ?", s.config.Namespace).Update("maintenance_error", "").Error; err != nil {
		return err
	}
	// Only historical metadata for already-removed objects; never delete assets
	// or run a broad bucket listing/deletion during retention maintenance.
	return db.Where("namespace = ? AND status = ? AND updated_at < ?", s.config.Namespace, Deleted, s.now().AddDate(0, 0, -90)).Delete(&models.DatabaseBackup{}).Error
}

func (s *Service) maintenanceFailure(db *gorm.DB) error {
	_ = db.Model(&models.DatabaseBackupSchedule{}).Where("namespace = ?", s.config.Namespace).Update("maintenance_error", "Penghapusan backup lama belum berhasil. Backup berikutnya menunggu agar penyimpanan tidak terus bertambah.").Error
	return errors.New("backup retention pending")
}

type Status struct {
	Ready            bool                    `json:"ready"`
	DownloadReady    bool                    `json:"download_ready"`
	DownloadIssue    string                  `json:"download_issue,omitempty"`
	Issue            string                  `json:"issue,omitempty"`
	Timezone         string                  `json:"timezone"`
	Retention        int                     `json:"retention"`
	Namespace        string                  `json:"namespace,omitempty"`
	NextRunAt        *time.Time              `json:"next_run_at,omitempty"`
	LastSuccessful   *time.Time              `json:"last_successful,omitempty"`
	MaintenanceError string                  `json:"maintenance_error,omitempty"`
	Backups          []models.DatabaseBackup `json:"backups"`
	TotalSuccessful  int64                   `json:"total_successful"`
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	status := Status{Ready: s.Ready(), Issue: s.issue, Timezone: apptime.Zone, Retention: Retention, Namespace: s.config.Namespace, Backups: []models.DatabaseBackup{}}
	status.DownloadIssue = s.sqlDownloadIssue()
	status.DownloadReady = s.Ready() && status.DownloadIssue == ""
	if s.config.Namespace == "" {
		return status, nil
	}
	db := s.db.WithContext(ctx)
	var schedule models.DatabaseBackupSchedule
	r := db.Where("namespace = ?", s.config.Namespace).Limit(1).Find(&schedule)
	if r.Error != nil {
		return status, r.Error
	}
	if r.RowsAffected > 0 {
		status.NextRunAt = &schedule.NextRunAt
		status.MaintenanceError = schedule.MaintenanceError
	}
	if err := db.Where("namespace = ? AND status <> ?", s.config.Namespace, Deleted).Order("created_at DESC").Limit(30).Find(&status.Backups).Error; err != nil {
		return status, err
	}
	if err := db.Model(&models.DatabaseBackup{}).Where("namespace = ? AND status = ?", s.config.Namespace, Succeeded).Count(&status.TotalSuccessful).Error; err != nil {
		return status, err
	}
	var latest models.DatabaseBackup
	if err := db.Where("namespace = ? AND status = ?", s.config.Namespace, Succeeded).Order("finished_at DESC").Limit(1).Find(&latest).Error; err != nil {
		return status, err
	}
	status.LastSuccessful = latest.FinishedAt
	return status, nil
}

func (s *Service) Download(ctx context.Context, id string) (models.DatabaseBackup, Object, error) {
	var job models.DatabaseBackup
	if !s.Ready() {
		return job, Object{}, ErrUnavailable
	}
	if err := s.db.WithContext(ctx).Where("id = ? AND namespace = ? AND status = ?", id, s.config.Namespace, Succeeded).First(&job).Error; err != nil {
		return job, Object{}, err
	}
	object, err := s.store.Get(ctx, job.ObjectKey)
	return job, object, err
}

func recordAudit(tx *gorm.DB, action, id string, actor *string, ip, message string) error {
	return tx.Create(&models.AuditLog{Action: action, TargetType: "database_backup", TargetID: id, ActorID: actor, IPAddress: ip, Description: message}).Error
}
