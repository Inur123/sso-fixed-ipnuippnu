package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Dumper interface {
	Dump(context.Context, io.Writer) error
	Version(context.Context) (int, error)
}
type PostgresDumper struct{ config Config }

func NewPostgresDumper(c Config) *PostgresDumper { return &PostgresDumper{config: c} }

func (d *PostgresDumper) Version(ctx context.Context) (int, error) {
	b, err := exec.CommandContext(ctx, d.config.DumpPath, "--version").Output()
	if err != nil {
		return 0, errors.New("pg_dump belum tersedia; periksa BACKUP_PG_DUMP_PATH")
	}
	m := regexp.MustCompile(`PostgreSQL\)? (\d+)`).FindSubmatch(b)
	if len(m) != 2 {
		return 0, errors.New("versi pg_dump tidak dapat dibaca")
	}
	n, _ := strconv.Atoi(string(m[1]))
	return n, nil
}

func (d *PostgresDumper) Dump(ctx context.Context, output io.Writer) error {
	dir, err := os.MkdirTemp("", "sso-pgpass-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	escape := func(v string) string { return strings.NewReplacer(`\`, `\\`, `:`, `\:`).Replace(v) }
	c := d.config
	for _, v := range []string{c.DBHost, c.DBPort, c.DBName, c.DBUser, c.DBPassword} {
		if strings.ContainsAny(v, "\r\n") {
			return errors.New("konfigurasi PostgreSQL harus satu baris")
		}
	}
	passwordFile := filepath.Join(dir, "pgpass")
	line := strings.Join([]string{escape(c.DBHost), escape(c.DBPort), escape(c.DBName), escape(c.DBUser), escape(c.DBPassword)}, ":") + "\n"
	if err := os.WriteFile(passwordFile, []byte(line), 0600); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, c.DumpPath, "--format=custom", "--compress=1", "--no-owner", "--no-acl", "--no-password", "--lock-wait-timeout=10s")
	// Allowlist the process environment. Do not leak application/R2 secrets to
	// pg_dump, interpolate shell input, or put the database password in argv.
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C", "TZ=Asia/Jakarta", "PGPASSFILE=" + passwordFile, "PGHOST=" + c.DBHost, "PGPORT=" + c.DBPort, "PGUSER=" + c.DBUser, "PGDATABASE=" + c.DBName, "PGSSLMODE=" + c.DBSSLMode, "PGCONNECT_TIMEOUT=10", "PGAPPNAME=pelajarnu-database-backup"}
	cmd.Stdout = output
	cmd.Stderr = io.Discard // Database names, SQL and credentials must not enter logs/UI.
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("pg_dump gagal; periksa izin database, ruang disk, dan koneksi")
	}
	return nil
}

type limitWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, errors.New("ukuran backup melebihi batas konfigurasi")
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}
