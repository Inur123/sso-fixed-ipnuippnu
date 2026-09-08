# Deployment SSO

Deployment produksi menggunakan artifact release, bukan `git pull` di VPS.
Backend, frontend, dan dokumentasi dibangun dari commit Git yang sama kemudian
diaktifkan melalui symlink `/opt/ipnu-sso/current`.

## Pembaruan satu perintah

Pastikan perubahan sudah di-commit dan di-push ke upstream branch, lalu jalankan
dari root repository pada komputer pengembang:

```bash
bash deploy/deploy-release.sh
```

Target default adalah `ubuntu@43.157.224.6`. Target lain dapat diberikan sebagai
argumen pertama:

```bash
bash deploy/deploy-release.sh ubuntu@alamat-vps
```

Komputer pengembang harus memiliki Git, Go, Node.js/npm, SSH, SCP, dan tar. SSH
key harus sudah dapat masuk tanpa prompt interaktif dan pengguna target harus
memiliki akses `sudo -n` pada VPS.

Skrip akan:

1. menolak working tree kotor, detached HEAD, atau commit yang belum sama dengan
   upstream Git;
2. menjalankan `go test`, `go vet`, lint/build frontend, serta typecheck/build
   dokumentasi;
3. membuat artifact Linux berisi ketiga layanan dan metadata commit;
4. memvalidasi runtime Next.js/Sharp sebelum aktivasi;
5. membuat release bertimestamp di `/opt/ipnu-sso/releases`;
6. mempertahankan ENV, sertifikat, systemd, dan Nginx produksi;
7. melakukan health check lokal dan publik, serta rollback otomatis jika
   aktivasi gagal; dan
8. mempertahankan lima release terbaru.

Output sukses berakhir dengan `DEPLOY_OK`. Release bertimestamp di VPS berbeda
dari GitHub Release: perbaikan kecil cukup di-commit dan di-deploy, sedangkan tag
atau GitHub Release hanya dibuat saat memang menerbitkan versi aplikasi baru.

## Konfigurasi produksi

Konfigurasi server tidak dibawa di dalam artifact dan tidak ditimpa saat update:

- `/etc/ipnu-sso/backend.env`
- `/etc/ipnu-sso/frontend.env`
- `/etc/ipnu-sso/deploy.env`
- `/etc/ipnu-sso/oidc-private.pem`
- `/etc/ipnu-sso/backup/`
- konfigurasi Nginx dan sertifikat Let's Encrypt

`BACKEND_UPSTREAM_URL` dan `FRONTEND_UPSTREAM_URL` di `deploy.env` harus berupa
origin loopback dengan port eksplisit. Build frontend memakai
`frontend/.env.production`; variabel `NEXT_PUBLIC_*` tertanam saat build.

`setup-vps.sh` hanya untuk persiapan server pertama kali. Jangan menjalankannya
untuk pembaruan rutin karena setup dapat menulis ulang service, frontend env, dan
virtual host Nginx.

## Rollback manual

Skrip melakukan rollback otomatis jika health check aktivasi gagal. Jika rollback
manual diperlukan, pilih salah satu direktori timestamp valid yang masih tersedia
di `/opt/ipnu-sso/releases`, arahkan kembali symlink `current`, lalu restart kedua
service. Selalu periksa target dengan `readlink -f` sebelum dan sesudah perubahan.
