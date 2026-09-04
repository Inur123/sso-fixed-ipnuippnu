// backup-decrypt is for legacy/R2 encrypted archives, not current SQL downloads.
// It produces an authenticated PostgreSQL archive, not a live restore.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"filippo.io/age"
)

func run(keyPath, inputPath, outputPath string) (err error) {
	if keyPath == "" || inputPath == "" || outputPath == "" {
		return fmt.Errorf("-identity, -in dan -out wajib diisi")
	}
	key, err := os.Open(keyPath)
	if err != nil {
		return fmt.Errorf("kunci pemulihan tidak dapat dibaca")
	}
	defer key.Close()
	identities, err := age.ParseIdentities(io.LimitReader(key, 16384))
	if err != nil {
		return fmt.Errorf("format kunci pemulihan tidak valid")
	}
	input, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("backup tidak dapat dibaca")
	}
	defer input.Close()
	reader, err := age.Decrypt(input, identities...)
	if err != nil {
		return fmt.Errorf("kunci tidak cocok atau backup rusak")
	}
	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("output harus file baru pada direktori yang tersedia")
	}
	defer func() {
		output.Close()
		if err != nil {
			_ = os.Remove(outputPath)
		}
	}()
	if _, err = io.Copy(output, reader); err != nil {
		return fmt.Errorf("integritas backup gagal; output tidak dipertahankan")
	}
	if err = output.Sync(); err != nil {
		return fmt.Errorf("output belum tersimpan ke disk")
	}
	return nil
}
func main() {
	key := flag.String("identity", "", "Path recovery.agekey untuk arsip terenkripsi; tidak diperlukan untuk impor .sql")
	input := flag.String("in", "", "Arsip .dump.age lama atau dari R2; bukan file .sql unduhan dashboard saat ini")
	output := flag.String("out", "", "File .dump baru, jangan di dalam repository")
	flag.Parse()
	if err := run(*key, *input, *output); err != nil {
		fmt.Fprintln(os.Stderr, "Dekripsi gagal:", err)
		os.Exit(1)
	}
	fmt.Println("Dekripsi dan pemeriksaan autentikasi selesai. Database belum diubah. Uji pemulihan pada database terisolasi terlebih dahulu.")
}
