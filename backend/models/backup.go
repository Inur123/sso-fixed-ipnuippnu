package models

import "time"

// Backup rows contain operational metadata, never database payloads or keys.
type DatabaseBackup struct {
	ID            string     `gorm:"type:uuid;primaryKey" json:"id"`
	Namespace     string     `gorm:"not null;index:idx_backup_namespace_status,priority:1" json:"-"`
	Kind          string     `gorm:"type:varchar(16);not null" json:"kind"`
	Status        string     `gorm:"type:varchar(16);not null;index:idx_backup_namespace_status,priority:2" json:"status"`
	Phase         string     `gorm:"type:varchar(32)" json:"phase"`
	ScheduledFor  *time.Time `json:"scheduled_for,omitempty"`
	RequestedBy   *string    `gorm:"type:uuid" json:"-"`
	ObjectKey     string     `gorm:"not null" json:"-"`
	SizeBytes     int64      `json:"size_bytes"`
	SHA256        string     `gorm:"type:varchar(64)" json:"sha256,omitempty"`
	Recipient     string     `gorm:"type:varchar(128)" json:"-"`
	Attempts      int        `gorm:"not null;default:0" json:"attempts"`
	NextAttemptAt *time.Time `gorm:"index" json:"next_attempt_at,omitempty"`
	LastError     string     `gorm:"type:varchar(500)" json:"last_error,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	CreatedAt     time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type DatabaseBackupSchedule struct {
	Namespace        string    `gorm:"primaryKey" json:"-"`
	NextRunAt        time.Time `gorm:"not null" json:"next_run_at"`
	MaintenanceError string    `gorm:"type:varchar(500)" json:"maintenance_error,omitempty"`
	CreatedAt        time.Time `json:"-"`
	UpdatedAt        time.Time `json:"-"`
}
