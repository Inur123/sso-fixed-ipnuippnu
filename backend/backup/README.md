# Backup database dan impor SQL

Alur penggunaan: **pilih backup → Unduh SQL → konfirmasi kata sandi akun SSO → impor file `.sql` ke PostgreSQL**.

File unduhan sudah siap diimpor. Anda **tidak perlu access key R2, recovery key, `recovery.agekey`, atau menjalankan `backup-decrypt`** untuk mengimpor file `.sql`. Penyiapan dan dekripsi dilakukan otomatis oleh backend sebelum unduhan dikirim.

## Membuat dan mengunduh backup

1. Masuk sebagai **super admin**, lalu buka **Backup database**.
2. Klik **Buat backup** dan konfirmasi kata sandi akun SSO, atau gunakan backup otomatis yang sudah tersedia.
3. Tunggu status **Berhasil** pada riwayat backup.
4. Pilih **Unduh SQL** pada snapshot yang diinginkan dan konfirmasi kata sandi akun SSO.
5. Simpan `backup-<id>.sql` secara privat. Isi file berasal dari waktu snapshot yang dipilih, bukan keadaan database pada waktu unduhan.

Kata sandi akun SSO hanya digunakan untuk mengizinkan tindakan di dashboard, bukan untuk mengenkripsi file unduhan. **File SQL berisi data sensitif tanpa enkripsi.** Jangan unggah ke Git, chat, situs konverter, atau folder publik.

## Impor ke PostgreSQL lokal atau server

Impor membutuhkan koneksi dan izin ke **database PostgreSQL tujuan**; kredensial database berbeda dari kata sandi akun SSO. Tidak ada access key Cloudflare atau recovery key yang perlu dimasukkan saat impor.

Untuk pemulihan pertama, gunakan database kosong terisolasi. Pilih file `.sql` pada fitur **native restore PostgreSQL** yang mendukung SQL biasa dan menjalankan `psql`, atau jalankan:

```sh
psql -X --set=ON_ERROR_STOP=1 --dbname=DATABASE_TUJUAN --file=/path/backup-ID.sql
```

Ganti nama database dan lokasi file dengan tujuan yang benar; atur koneksi host/port/user PostgreSQL sesuai lingkungan Anda. Gunakan PostgreSQL/psql major yang sama dengan sumber backup terlebih dahulu. `ON_ERROR_STOP` menghentikan impor ketika terjadi kesalahan; script backup sudah dibungkus transaksi.

File ini untuk **PostgreSQL**, bukan MySQL/phpMyAdmin. SQL dapat berisi `COPY FROM stdin` dan perintah psql, sehingga bukan sekadar menempelkan seluruh file ke SQL editor biasa. Jangan membuka file `.sql` menggunakan alat dekripsi atau `pg_restore` untuk arsip custom.

**Impor mengganti objek/tabel yang tercakup dalam backup pada database tujuan, bukan menggabungkan data.** Objek tambahan di luar snapshot tidak otomatis dihapus dan dependensi eksternal dapat menyebabkan impor gagal. Database tujuan harus sudah tersedia; file tidak menghapus/membuat seluruh database.

Sebelum menimpa database yang sedang digunakan: buat cadangan terbaru, hentikan aplikasi/worker, pastikan tujuan impor benar, dan siapkan pemulihan terencana. Setelah impor, sesuaikan koneksi backend bila memakai database baru dan periksa data sebelum membuka layanan. Snapshot mengembalikan sesi/token dan antrean lama juga: pengelola perlu mencabut akses lama serta meninjau antrean sebelum layanan kembali aktif. Lihat [pemeriksaan pemulihan oleh pengelola](OPERATIONS.md#pemulihan-operator-bukan-aksi-dashboard).

Dashboard hanya membuat dan mengunduh backup; tidak melakukan impor atau menimpa database secara otomatis.

## Jadwal dan penyimpanan

- Otomatis setiap **Minggu pukul 02.00 WIB (`Asia/Jakarta`)**; manual dapat dibuat melalui dashboard.
- Maksimal **10 backup berhasil**, gabungan manual dan otomatis. File lama dihapus setelah pengganti terverifikasi; selama rotasi dapat ada 11 file sementara.
- Jadwal yang terlewat saat server mati diproses setelah layanan kembali siap. Beberapa minggu yang terlewat digabung menjadi satu snapshot terbaru, bukan snapshot historis yang tidak pernah dibuat. Tidak ada notifikasi email.
- Arsip di R2 tetap terenkripsi. Angka ukuran dan SHA-256 pada riwayat milik arsip R2, bukan file SQL unduhan.
- Backup mencakup satu database aplikasi, bukan avatar, `.env`, role/password PostgreSQL, atau kunci aplikasi. Simpan konfigurasi dan kunci aplikasi secara terpisah agar data aplikasi yang terenkripsi tetap dapat digunakan setelah restore.

## Untuk pengelola server

Kredensial R2 dan kunci arsip **tetap digunakan secara internal oleh backend**. Jangan menghapusnya: tanpa konfigurasi ini, backend tidak dapat menyimpan/membaca arsip atau menyiapkan SQL. Pengguna file `.sql` tidak perlu mengelolanya saat impor.

Aktivasi backend, pengamanan kunci, dan pemulihan darurat untuk arsip `.dump.age` lama dipisahkan ke [panduan operasional server](OPERATIONS.md). Prosedur darurat tersebut bukan langkah impor SQL sehari-hari.
