// backup-keygen prepares backend/archive recovery keys on an operator's trusted
// machine. It is not a prerequisite for importing a dashboard SQL download.
package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"filippo.io/age"
)

func run(directory string) error {
	if directory == "" {
		return fmt.Errorf("gunakan -out /path/direktori-baru-di-luar-repository")
	}
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	// Mkdir (not MkdirAll) refuses an existing directory and never overwrites keys.
	if err := os.Mkdir(directory, 0700); err != nil {
		return fmt.Errorf("direktori harus baru dan parent harus tersedia: %w", err)
	}
	files := map[string]string{
		"recovery.agekey":    "# KUNCI PRIVAT: simpan salinan offline, jangan unggah ke Git/chat/frontend. Salinan operasional hanya pada path backend privat untuk unduhan SQL.\n" + identity.String() + "\n",
		"backend-backup.env": "# Kunci arsip untuk backend, bukan syarat impor SQL.\n# Atur BACKUP_IDENTITY_FILE terpisah ke recovery.agekey privat untuk unduhan SQL.\nBACKUP_ENABLED=true\nBACKUP_AGE_RECIPIENT=" + identity.Recipient().String() + "\nBACKUP_R2_SSE_KEY=" + base64.StdEncoding.EncodeToString(key) + "\nBACKUP_PREFIX=backups/database\nBACKUP_MAX_SIZE_MB=512\nBACKUP_PG_DUMP_PATH=pg_dump\n",
		"README.txt":         "PENGGUNAAN BIASA: Unduh SQL di dashboard, konfirmasi kata sandi akun SSO, lalu impor file .sql ke PostgreSQL. Tidak perlu access key R2, recovery.agekey, atau backup-decrypt untuk mengimpor SQL. Koneksi/izin database tujuan tetap diperlukan. SQL tidak terenkripsi; simpan privat dan periksa tujuan impor karena tabel dalam backup diganti. Panduan: backend/backup/README.md.\n\nKHUSUS PENGELOLA SERVER: Folder ini melindungi arsip terenkripsi R2, bukan file SQL yang sudah diunduh. Backend memakai BACKUP_IDENTITY_FILE privat untuk menyiapkan SQL secara otomatis. Simpan recovery.agekey DAN backend-backup.env dalam salinan offline terenkripsi. Jangan unggah kunci ke Git/chat/frontend atau membuang kunci lama. Pemulihan darurat arsip .dump.age membutuhkan identity dan, untuk mengambil dari R2, SSE-C key. Setup dan prosedur darurat: backend/backup/OPERATIONS.md.\n",
	}
	for name, content := range files {
		file, err := os.OpenFile(filepath.Join(directory, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := file.WriteString(content)
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
func main() {
	directory := flag.String("out", "", "Direktori baru di luar repository untuk kunci backend/recovery; bukan untuk impor .sql")
	flag.Parse()
	if err := run(*directory); err != nil {
		fmt.Fprintln(os.Stderr, "Pembuatan recovery kit gagal:", err)
		os.Exit(1)
	}
	fmt.Println("Recovery kit dibuat. Simpan salinan offline terenkripsi sebelum mengaktifkan backup. Tidak ada kunci yang dicetak ke terminal.")
}
