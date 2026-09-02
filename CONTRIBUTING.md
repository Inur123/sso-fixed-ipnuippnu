# Panduan kontribusi

Repository ini menggunakan GitHub Flow, Semantic Versioning, dan Conventional
Commits agar riwayat perubahan mudah ditinjau dan dirilis.

## Alur branch

1. Selalu mulai dari `main` terbaru.
2. Buat branch berumur pendek dengan nama yang menjelaskan tujuan:
   - `feat/nama-fitur`
   - `fix/nama-perbaikan`
   - `docs/nama-dokumentasi`
   - `chore/nama-pekerjaan`
   - `release/vMAJOR.MINOR.PATCH` untuk persiapan rilis
3. Push branch dan buka pull request menuju `main`.
4. Merge hanya setelah CI lulus. Hapus branch setelah merge.

Jangan melakukan force-push ke `main` dan jangan menyimpan credential atau file
`.env` di Git.

## Format commit

Gunakan Conventional Commits:

```text
feat: tambahkan verifikasi Turnstile
fix: perbaiki validasi hostname
docs: perbarui panduan deployment
chore: perbarui workflow CI
```

Gunakan `feat` untuk fitur baru, `fix` untuk perbaikan bug, `docs` untuk
dokumentasi, `test` untuk pengujian, `refactor` untuk perubahan internal, dan
`chore` untuk pemeliharaan repository.

## Pemeriksaan sebelum pull request

```bash
cd backend
go test ./...
go vet ./...

cd ../frontend
npm ci
npm run lint -- --max-warnings=0
npx tsc --noEmit

cd ../documentation
npm ci
npm run typecheck
```

## Rilis

- Versi proyek disimpan di `VERSION` dan dicatat di `CHANGELOG.md`.
- Tag rilis menggunakan format `vMAJOR.MINOR.PATCH`, misalnya `v0.1.0`.
- Rilis production hanya dibuat dari commit yang sudah berada di `main`.
- Perbarui bagian `Unreleased`, versi package terkait, dan catatan perubahan
  sebelum membuat tag.
