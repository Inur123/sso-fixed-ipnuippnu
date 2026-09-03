# Konfigurasi alamat service lokal

Hanya URL localhost/loopback yang dipindahkan ke env. Domain publik dan layanan
eksternal tetap seperti sebelumnya; konfigurasi Turnstile dan R2 tidak berubah.

Salin `deploy/.env.example` ke `/etc/ipnu-sso/deploy.env` pada VPS sebelum
menjalankan setup atau release. `DEPLOY_ENV_FILE` dapat digunakan saat setup
jika lokasi file berbeda. Node.js 22+ diperlukan untuk membaca env tanpa
mengeksekusi isinya sebagai shell.

- `BACKEND_UPSTREAM_URL`: alamat lokal backend untuk Nginx dan health check.
- `FRONTEND_UPSTREAM_URL`: alamat lokal frontend untuk Nginx dan health check.

Keduanya harus berupa origin loopback dengan port eksplisit. Port service
diambil dari URL tersebut; `BACKEND_PORT` di backend.env yang sudah ada harus
cocok. Tidak ada fallback URL lokal di skrip.

Build frontend tetap memakai `frontend/.env.production`. Validator artifact
menolak URL localhost pada semua port, bukan hanya port API/dokumentasi.
Contoh URL lokal dalam dokumentasi bukan konfigurasi aplikasi.

Perubahan konfigurasi ini tidak menjalankan deploy.
