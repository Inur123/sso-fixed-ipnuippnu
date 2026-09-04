// backup-keygen runs on the operator's trusted machine, never on the SSO server.
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
		"backend-backup.env": "# Hanya file ini yang konfigurasinya dipasang pada backend.\nBACKUP_ENABLED=true\nBACKUP_AGE_RECIPIENT=" + identity.Recipient().String() + "\nBACKUP_R2_SSE_KEY=" + base64.StdEncoding.EncodeToString(key) + "\nBACKUP_PREFIX=backups/database\nBACKUP_MAX_SIZE_MB=512\nBACKUP_PG_DUMP_PATH=pg_dump\n",
		"README.txt":         "Simpan recovery.agekey DAN backend-backup.env di penyimpanan offline terenkripsi. SSE-C key diperlukan jika mengambil arsip langsung dari R2. Unduhan dashboard berupa SQL tanpa enkripsi: backend membutuhkan BACKUP_IDENTITY_FILE pada path privat 0600/0400 untuk menyiapkannya. Jangan unggah identity ke Git/chat/frontend. Arsip R2 dan file .dump.age lama tetap membutuhkan identity untuk dekripsi offline. Jangan membuang kunci lama selama backup terkait masih disimpan. Baca backend/backup/README.md untuk pemulihan.\n",
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
	directory := flag.String("out", "", "Direktori baru di luar repository untuk recovery kit")
	flag.Parse()
	if err := run(*directory); err != nil {
		fmt.Fprintln(os.Stderr, "Pembuatan recovery kit gagal:", err)
		os.Exit(1)
	}
	fmt.Println("Recovery kit dibuat. Simpan salinan offline terenkripsi sebelum mengaktifkan backup. Tidak ada kunci yang dicetak ke terminal.")
}
