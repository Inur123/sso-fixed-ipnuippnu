package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"filippo.io/age"
	"sso-backend/models"
)

func (s *Service) sqlDownloadIssue() string {
	if s.config.IdentityFile == "" {
		return "Siapkan BACKUP_IDENTITY_FILE privat pada backend untuk mengunduh SQL."
	}
	return s.downloadIssue
}

func readDownloadIdentities(name string) ([]age.Identity, error) {
	if !filepath.IsAbs(name) {
		return nil, errors.New("BACKUP_IDENTITY_FILE harus berupa path absolut")
	}
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 16384 {
		return nil, errors.New("BACKUP_IDENTITY_FILE harus file privat biasa berizin 0600 atau 0400, bukan symlink")
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, errors.New("kunci unduhan SQL tidak dapat dibaca")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm()&0077 != 0 {
		return nil, errors.New("file kunci unduhan SQL berubah saat dibaca")
	}
	identities, err := age.ParseIdentities(io.LimitReader(file, 16385))
	if err != nil || len(identities) == 0 {
		return nil, errors.New("format kunci unduhan SQL tidak valid")
	}
	// Only local X25519 keys: never run external age plugins on the backend.
	for _, identity := range identities {
		if _, ok := identity.(*age.X25519Identity); !ok {
			return nil, errors.New("unduhan SQL membutuhkan kunci age X25519")
		}
	}
	return identities, nil
}

func (s *Service) checkSQLDownload(ctx context.Context) error {
	identities, err := readDownloadIdentities(s.config.IdentityFile)
	if err != nil {
		return err
	}
	matched := false
	for _, identity := range identities {
		if s.config.Recipient != nil && identity.(*age.X25519Identity).Recipient().String() == s.config.Recipient.String() {
			matched = true
		}
	}
	if !matched {
		return errors.New("kunci unduhan SQL tidak cocok dengan public recipient backup")
	}
	version, err := NewPostgresDumper(Config{DumpPath: s.config.RestorePath}).Version(ctx)
	if err != nil {
		return errors.New("pg_restore belum tersedia; periksa BACKUP_PG_RESTORE_PATH")
	}
	dumpVersion, err := s.dumper.Version(ctx)
	if err != nil || version != dumpVersion {
		return errors.New("major version pg_restore harus sama dengan pg_dump")
	}
	return nil
}

// Unlink immediately after opening. On the supported Unix hosts, even a crash
// cannot leave named plaintext archives/SQL behind; closing releases the data.
func privateScratch() (*os.File, error) {
	f, err := os.CreateTemp("", "sso-backup-download-")
	if err != nil {
		return nil, errors.New("ruang sementara unduhan tidak tersedia")
	}
	if err := os.Remove(f.Name()); err != nil {
		f.Close()
		return nil, errors.New("file sementara unduhan tidak dapat diamankan")
	}
	return f, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

type sqlDownloadBody struct {
	*os.File
	once    sync.Once
	release func()
}

func (b *sqlDownloadBody) Close() error {
	var err error
	b.once.Do(func() { err = b.File.Close(); b.release() })
	return err
}

// DownloadSQL converts the SELECTED historical snapshot, never a fresh dump of
// the live DB. Fully authenticate/decrypt/convert before sending any SQL bytes.
// pg_restore emits a script only: no database connection or restore is executed.
func (s *Service) DownloadSQL(ctx context.Context, id string) (job models.DatabaseBackup, object Object, err error) {
	if !s.Ready() || s.sqlDownloadIssue() != "" {
		return job, object, ErrUnavailable
	}
	if !s.downloadMu.TryLock() {
		return job, object, ErrBusy
	}
	transferred := false
	defer func() {
		if !transferred {
			s.downloadMu.Unlock()
		}
	}()
	identities, err := readDownloadIdentities(s.config.IdentityFile)
	if err != nil {
		return job, object, ErrUnavailable
	}
	job, remote, err := s.Download(ctx, id)
	if err != nil {
		return job, object, err
	}
	defer remote.Body.Close()
	if job.SizeBytes <= 0 || job.SizeBytes > s.config.MaxBytes || remote.Size != job.SizeBytes {
		return job, object, errors.New("ukuran backup tidak valid")
	}
	ciphertext, err := privateScratch()
	if err != nil {
		return job, object, err
	}
	defer ciphertext.Close()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(ciphertext, hash), contextReader{ctx, io.LimitReader(remote.Body, job.SizeBytes+1)})
	if err != nil || n != job.SizeBytes || hex.EncodeToString(hash.Sum(nil)) != job.SHA256 {
		return job, object, errors.New("integritas backup tidak sesuai")
	}
	if _, err = ciphertext.Seek(0, io.SeekStart); err != nil {
		return job, object, err
	}
	reader, err := age.Decrypt(ciphertext, identities...)
	if err != nil {
		return job, object, errors.New("backup tidak dapat didekripsi")
	}
	archive, err := privateScratch()
	if err != nil {
		return job, object, err
	}
	defer archive.Close()
	if _, err = io.Copy(&limitWriter{writer: archive, remaining: s.config.MaxBytes}, contextReader{ctx, reader}); err != nil {
		return job, object, errors.New("dekripsi backup gagal atau melewati batas ukuran")
	}
	ciphertext.Close()
	if _, err = archive.Seek(0, io.SeekStart); err != nil {
		return job, object, err
	}
	script, err := privateScratch()
	if err != nil {
		return job, object, err
	}
	defer func() {
		if !transferred {
			script.Close()
		}
	}()
	output := &limitWriter{writer: script, remaining: s.config.MaxBytes}
	// --clean replaces archived objects only on later import, not on download.
	// No --create/--dbname, no DB credentials, no shell interpolation.
	header := "-- PelajarNU Magetan ID: backup PostgreSQL\n-- PERINGATAN: impor mengganti objek/tabel dalam backup pada database tujuan.\n-- File ini tidak terenkripsi. Simpan privat. Gunakan psql dengan ON_ERROR_STOP.\n-- Snapshot backup ID: " + job.ID + "\n\n"
	if _, err = io.WriteString(output, header); err != nil {
		return job, object, err
	}
	cmd := exec.CommandContext(ctx, s.config.RestorePath, "--file=-", "--clean", "--if-exists", "--no-owner", "--no-acl", "--exit-on-error", "--single-transaction")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C", "TZ=Asia/Jakarta"}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = archive, output, io.Discard
	cmd.WaitDelay = 5 * time.Second
	if err = cmd.Run(); err != nil {
		return job, object, errors.New("konversi SQL gagal; periksa pg_restore, ukuran file, dan ruang disk")
	}
	stat, err := script.Stat()
	if err != nil || stat.Size() <= int64(len(header)) {
		return job, object, errors.New("hasil SQL kosong")
	}
	if _, err = script.Seek(0, io.SeekStart); err != nil {
		return job, object, err
	}
	if err = ctx.Err(); err != nil {
		return job, object, err
	}
	transferred = true
	return job, Object{Body: &sqlDownloadBody{File: script, release: s.downloadMu.Unlock}, Size: stat.Size()}, nil
}
