# Changelog

Semua perubahan penting pada PelajarNU Magetan ID dicatat di sini. Format
mengikuti [Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/) dan versi
mengikuti [Semantic Versioning](https://semver.org/lang/id/).

## [Unreleased]

## [2.2.0] - 2026-09-04

### Added

- Menu Backup database khusus super admin dengan pembuatan manual dan jadwal
  otomatis setiap Minggu pukul 02.00 `Asia/Jakarta` (WIB), tanpa notifikasi email.
- Antrean dan jadwal backup persisten, penguncian pekerjaan lintas instance,
  retry kegagalan, serta pemrosesan jadwal terlewat setelah layanan kembali siap.
- Arsip PostgreSQL terenkripsi age X25519 dan SSE-C pada bucket R2 yang sudah
  digunakan, dengan namespace terpisah per environment dan identitas instance.
- Retensi 10 backup berhasil, termasuk manual dan otomatis. Arsip baru dibaca
  ulang dan diverifikasi ukuran serta SHA-256 sebelum backup terlama dihapus.
- Unduhan SQL dari snapshot yang dipilih, termasuk arsip `.dump.age`, dengan
  konfirmasi kata sandi, pemeriksaan sesi/role/Origin, rate limit, dan audit.
- Alat pembuatan recovery kit, dekripsi offline, dan panduan operasional backup.
- Audit permintaan, hasil, unduhan, serta penghapusan backup oleh retensi.

### Changed

- Dashboard menampilkan status konfigurasi, jadwal, riwayat, ukuran arsip R2,
  checksum, serta kesiapan unduhan SQL secara terpisah.
- Loading halaman backup menggunakan placeholder per bagian dan baris tabel.
  Tombol Buat backup tetap terlihat dengan tampilan stabil selama refresh;
  validasi kesiapan tetap dilakukan pada dialog dan backend.
- File dump, SQL backup, serta kunci pemulihan ditambahkan ke aturan ignore Git.

### Fixed

- Operasi aset publik dibatasi pada path avatar agar tidak dapat menyentuh
  namespace backup di bucket yang sama.
- Unduhan SQL memverifikasi integritas dan autentikasi arsip sebelum mengirim
  isi, membatasi ukuran/konkurensi, dan memeriksa ulang sesi setelah konversi.
- Pemeriksaan ukuran unduhan frontend mendukung respons yang dikompresi proxy
  serta menolak file kosong, terpotong, atau respons yang bukan SQL.

### Catatan upgrade

- Rilis minor ini menambahkan fitur backup; kontrak OAuth/OIDC tetap kompatibel.
  Backend menambahkan tabel metadata dan jadwal backup melalui migrasi startup.
- Backup nonaktif secara default. Aktivasi membutuhkan konfigurasi backend,
  kunci age/SSE-C, akses R2 yang sesuai, serta `pg_dump` dengan major version
  yang sama dengan server PostgreSQL. Tidak ada secret baru di frontend.
- Unduhan SQL membutuhkan `pg_restore` dengan major yang sesuai dan
  `BACKUP_IDENTITY_FILE` privat di luar repository. Backend dapat mendekripsi
  arsip untuk fitur ini; tetap simpan salinan kunci pemulihan secara offline.
- SQL hasil unduhan tidak terenkripsi. Impor mengganti objek/tabel yang tercakup
  dalam backup pada database tujuan, bukan menggabungkan data. Uji pada database
  terisolasi sebelum pemulihan terencana; dashboard tidak melakukan restore.
- Backup mencakup satu database aplikasi, bukan file avatar, konfigurasi, atau
  kunci aplikasi. Pertahankan konfigurasi dan kunci tersebut secara terpisah.
- Lihat [panduan backup](backend/backup/README.md) untuk aktivasi dan pemulihan.
  Build ulang frontend dan restart backend diperlukan saat deployment nanti;
  penerbitan release GitHub tidak otomatis memperbarui server production.

## [2.1.0] - 2026-09-03

### Added

- Pemulihan akun melalui `/reset-password` dengan tautan acak sekali pakai,
  masa berlaku 15 menit, dan kata sandi minimal 8 karakter.
- Rate limit per IP, cooldown per akun, batas permintaan per jam, serta respons
  generik untuk akun yang tidak ditemukan, belum terverifikasi, atau nonaktif.
- Antrean email reset dan notifikasi perubahan kata sandi yang persisten,
  dilengkapi retry, enkripsi AES-GCM untuk token reset, dan pembersihan payload.
- Audit permintaan dan penyelesaian reset kata sandi dalam transaksi database.

### Changed

- Template email OTP, reset, dan notifikasi keamanan menggunakan tampilan
  konsisten dengan portal, logo SSO tertanam, serta versi teks biasa.
- Registrasi diperjelas sebagai pembuatan akun SSO, bukan pendaftaran anggota.
- Waktu aplikasi, koneksi database, email, dan tampilan menggunakan
  `Asia/Jakarta` (WIB), tanpa menggeser waktu kejadian atau expiry token.
- Alamat upstream lokal deployment dipindahkan ke konfigurasi environment.
- CI memakai build dan vet backend; pengujian perilaku dilakukan secara
  terpisah dengan file serta data sementara yang dibersihkan setelah digunakan.
- README diperbarui dengan kemampuan pemulihan akun dan informasi rilis.

### Fixed

- Reset mencabut sesi, token aplikasi, authorization code, dan tautan reset lama.
  Login serta pertukaran token memeriksa ulang kredensial di dalam transaksi
  untuk mencegah sesi baru dari kredensial yang sudah diubah.
- Halaman reset menghapus token dari address bar setelah dibaca dan memakai
  kebijakan no-store, no-referrer, serta noindex.
- Validasi artifact produksi menolak URL localhost pada semua port; validasi
  build juga mengenali alamat loopback IPv6.
- Tautan Short URL eksternal tidak lagi dicantumkan dalam tabel layanan resmi
  README; layanan terhubung pada portal tetap tersedia.

### Removed

- File pengujian permanen dari repository sesuai kebijakan proyek. Hasil build
  dan lint tidak diperlakukan sebagai pengganti pengujian perilaku.

### Catatan upgrade

- API OAuth/OIDC tetap kompatibel; rilis minor ini menambahkan pemulihan akun.
- Sebelum menggunakan skrip deployment terbaru, siapkan
  `/etc/ipnu-sso/deploy.env` berdasarkan `deploy/.env.example`; port upstream
  backend harus cocok dengan `BACKEND_PORT`. Pembacaan konfigurasi membutuhkan
  Node.js 22+.
- Backend menambahkan tabel token reset dan antrean email melalui migrasi
  startup yang sudah digunakan proyek. Pastikan backup database tersedia.
- Pertahankan `CLIENT_SECRET_ENCRYPTION_KEY` yang sudah digunakan, SMTP yang
  valid, dan `FRONTEND_PUBLIC_URL` yang benar. Jangan mengganti kunci enkripsi
  ketika antrean masih berisi payload terenkripsi.
- Build ulang frontend diperlukan. Penerbitan tag/release GitHub tidak otomatis
  memperbarui server production.

## [2.0.0] - 2026-09-02

### Breaking Changes

- Endpoint registrasi sekarang mewajibkan `turnstile_token` yang valid.
- Production sekarang mewajibkan Site Key dan Secret Key Cloudflare Turnstile.
- Respons registrasi memakai status antrean email melalui
  `verification_email_queued`; integrasi yang membaca status pengiriman langsung
  perlu disesuaikan.

### Added

- Perlindungan Cloudflare Turnstile pada registrasi dengan validasi server-side,
  pembatasan hostname, dan pengecekan action.
- Skeleton loading untuk widget Turnstile serta penguncian tombol daftar sebelum
  verifikasi berhasil.
- Antrean email OTP persisten dengan retry exponential dan worker asinkron.
- Enkripsi payload OTP menggunakan AES-GCM sebelum disimpan di database.
- Kartu layanan Short URL menuju `s.pelajarnumagetan.or.id`.
- Workflow CI untuk backend, frontend, dan dokumentasi.
- Semantic Versioning, changelog, template pull request, dan kebijakan keamanan
  repository.

### Changed

- Identitas produk diseragamkan menjadi PelajarNU Magetan ID.
- Perilaku indikator kata sandi pada formulir registrasi diperjelas.
- Tampilan portal, halaman autentikasi, sesi, keamanan, dan audit disempurnakan.
- Deployment production diperketat agar menolak key Turnstile test atau
  placeholder.

## [1.0.0] - 2026-08-21

### Added

- Rilis production awal pusat identitas dan Single Sign-On PelajarNU Magetan.
- Registrasi, login, verifikasi email OTP, profil, dan keamanan akun.
- OAuth 2.0 Authorization Code dengan PKCE S256.
- OpenID Connect dengan discovery metadata, ID token RS256, JWKS, dan UserInfo.
- Manajemen aplikasi, client secret, redirect URI, consent, assignment pengguna,
  serta pencabutan akses.
- Refresh-token rotation, reuse detection, revocation, dan authorization code
  sekali pakai.
- Dashboard administrasi pengguna dan audit log untuk super admin.
- Provisioning pengguna berbasis transactional outbox.
- Dokumentasi integrasi aplikasi terpisah berbasis Docusaurus.

[Unreleased]: https://github.com/Inur123/sso-fixed-ipnuippnu/compare/v2.2.0...HEAD
[2.2.0]: https://github.com/Inur123/sso-fixed-ipnuippnu/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/Inur123/sso-fixed-ipnuippnu/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/Inur123/sso-fixed-ipnuippnu/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/Inur123/sso-fixed-ipnuippnu/releases/tag/v1.0.0
