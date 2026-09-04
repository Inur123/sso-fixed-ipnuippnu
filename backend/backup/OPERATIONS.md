# Operasional server dan pemulihan darurat

Dokumen ini khusus pengelola server, bukan langkah yang perlu dilakukan setiap kali mengimpor backup. Untuk penggunaan biasa, ikuti [panduan unduh dan impor SQL](README.md): file `.sql` dari dashboard tidak membutuhkan access key R2, recovery key, atau dekripsi manual.

Kredensial R2 dan kunci enkripsi di bawah ini tetap diperlukan oleh backend untuk menyimpan arsip terenkripsi dan menyiapkan unduhan SQL secara otomatis. Jangan menghapus atau menggantinya hanya karena unduhan sekarang berupa SQL. Tidak ada perubahan deployment otomatis, email notifikasi, atau tombol restore produksi.

## Perilaku

- Manual: konfirmasi kata sandi, lalu masuk antrean persisten PostgreSQL. Hanya satu pekerjaan berjalan, termasuk pada beberapa instance yang memakai database sama.
- Otomatis: setiap Minggu pukul **02.00 Asia/Jakarta**. Pengecekan dilakukan saat startup dan setiap menit. Saat pertama kali diaktifkan, jadwal dimulai pada Minggu berikutnya.
- Jika server mati, satu backup terbaru dibuat setelah aplikasi dan database siap; beberapa minggu yang terlewat tidak menghasilkan antrean berulang. Proses export yang terputus dimulai ulang, bukan dilanjutkan dari file parsial.
- Kegagalan dicoba ulang dengan jeda 2–64 menit. Status dan kesalahan aman ditampilkan di dashboard, tanpa email.
- Simpan **10 backup berhasil**, gabungan manual dan otomatis. Backup baru diekspor, dienkripsi, diunggah, lalu dibaca ulang dan diperiksa ukuran serta SHA-256-nya sebelum file terlama dihapus. Sesaat selama rotasi dapat ada 11 file. Jika penghapusan gagal, backup berikutnya ditahan sampai retensi berhasil, agar jumlah file tidak terus membesar.
- Metadata file yang sudah dihapus dibersihkan setelah 90 hari. Audit hanya mencatat permintaan, hasil, otorisasi unduhan, dan retensi; tidak mencatat password, isi dump, atau kunci.

## Bucket Cloudflare yang sama

Memakai `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, dan `R2_BUCKET_NAME` yang sudah ada. Tidak perlu bucket baru. Token R2 harus punya **Object Read & Write** pada bucket itu, termasuk izin menghapus untuk retensi.

Prefix sebenarnya adalah `backups/database/<APP_ENV>/<identitas-instance>/`; identitas dibuat dari URL frontend dan nama database. Dengan demikian lokal dan production tidak saling menghapus backup. Perubahan URL/nama database/prefix akan membuat namespace baru; file pada namespace lama harus dikelola secara sengaja oleh operator, bukan dihapus otomatis. Jangan memasang lifecycle deletion bucket yang bertabrakan dengan retensi ini.

**Folder bukan pembatas akses.** Jika bucket memakai URL `r2.dev`, custom domain, atau Worker publik, jangan menganggap folder backup otomatis privat. Backup selalu memakai dua lapisan:

1. **age X25519** sebelum data backup ditulis ke disk/diunggah. Pembuatan backup hanya membutuhkan public recipient. Untuk unduhan SQL, backend juga harus dapat membaca private identity melalui `BACKUP_IDENTITY_FILE` privat di luar repository.
2. **SSE-C AES-256 R2**, dengan kunci storage wajib pada upload/read. Sistem memeriksa bahwa pembacaan tanpa SSE-C ditolak dan tidak pernah beralih ke upload biasa. Jangan memberi kunci SSE-C kepada Worker publik/proxy aset.

Avatar hanya boleh mengunggah/menghapus path `avatars/…`. Akses download backup selalu lewat backend, RBAC super admin, pemeriksaan Origin, konfirmasi kata sandi, dan rate limit. Tidak ada presigned/public link. **Unduhan dashboard adalah SQL tanpa enkripsi**; kata sandi hanya mengotorisasi unduhan, bukan melindungi file setelah disimpan. Backend yang bisa mendekripsi berarti kompromi backend juga berisiko membuka backup: ini trade-off untuk unduhan SQL langsung. R2 credentials yang bocor tetap dapat dipakai menghapus objek: satu bucket tidak memberi isolasi terhadap pencurian kredensial penghapus. Simpan salinan pemulihan offline.

## Aktivasi backend

1. Di komputer tepercaya, dari direktori `backend`, buat recovery kit pada **direktori baru di luar repository**:

   ```sh
   go run ./cmd/backup-keygen -out /path/aman/recovery-kit
   ```

2. Simpan seluruh recovery kit dalam penyimpanan offline terenkripsi/password manager. `recovery.agekey` adalah kunci privat: **jangan unggah ke Git, chat, atau frontend**. Untuk unduhan SQL, salinan operasionalnya perlu tersedia bagi backend pada path privat; jangan memakai folder publik. `backend-backup.env` juga mengandung kunci rahasia SSE-C; simpan salinannya untuk mengambil backup langsung dari R2 ketika server hilang.
3. Salin nilai dari `backend-backup.env` ke environment **backend**. Pada deployment saat ini gunakan `/etc/ipnu-sso/backend.env`; untuk lokal gunakan `.env` backend yang diabaikan Git. Tambahkan nilai backup, jangan menimpa seluruh konfigurasi SSO. Tidak ada environment frontend tambahan.
4. Pastikan `pg_dump` terpasang pada host/container backend dengan **major version sama** seperti PostgreSQL database. Atur `BACKUP_PG_DUMP_PATH` ke path absolut jika binary tidak berada di PATH service; misalnya `/usr/lib/postgresql/17/bin/pg_dump` hanya bila server memang PostgreSQL 17.
5. Pastikan proses backend dapat menulis file sementara di temp directory. Default maksimum **512 MiB per file terenkripsi** dan timeout **30 menit**; `BACKUP_MAX_SIZE_MB` dapat 16–2048. Sediakan ruang disk bebas melebihi batas satu file dan kuota R2 untuk 11 file selama rotasi. Konkurensi export satu, kompresi level 1, streaming ke disk/R2, tidak memuat seluruh database ke RAM. Tetap ada beban I/O; ini bukan jaminan tanpa dampak pada server.
6. Untuk unduhan SQL, set `BACKUP_IDENTITY_FILE=/path/privat/recovery.agekey` (file biasa 0600/0400, bukan symlink) dan siapkan `pg_restore` major yang sama dengan `pg_dump`. `BACKUP_PG_RESTORE_PATH` opsional, default binary `pg_restore` di direktori yang sama dengan path absolut `pg_dump`, atau dari PATH. File identity dapat menyimpan beberapa identity X25519 saat rotasi; pertahankan identity lama untuk backup lama. Jangan mengganti pasangan kunci yang sudah digunakan.
7. Restart backend setelah konfigurasi siap, periksa dashboard, buat satu backup manual, unduh SQL, lalu uji restore terisolasi. Jangan menganggap sebuah backup berguna hanya karena upload berhasil.

Unduhan SQL tetap memakai snapshot yang dipilih, termasuk arsip lama `.dump.age`. Backend memeriksa ukuran/SHA-256 arsip, mendekripsi dan memeriksa autentikasi age sampai selesai, lalu menjalankan `pg_restore` **tanpa koneksi database** untuk menghasilkan script SQL. Tidak ada perubahan database saat download. Arsip dan SQL sementara dibatasi masing-masing oleh `BACKUP_MAX_SIZE_MB` (default 512 MiB), tidak dimuat penuh ke RAM backend; hasil SQL bisa lebih besar daripada arsip terkompresi. Sediakan disk hingga sekitar dua kali batas ini per unduhan, selain proses backup. File sementara 0600 langsung di-unlink pada host Unix (macOS/Linux), sehingga tidak menyisakan path plaintext setelah selesai/crash. Hanya satu unduhan SQL per instance dapat diproses pada suatu waktu, timeout 10 menit. Unduhan ditolak sebelum mengirim isi jika konversi gagal. Frontend memuat hasil unduhan sebagai Blob sebelum menyimpan, sehingga unduhan besar tetap membutuhkan memori perangkat.

Kunci/konfigurasi tidak tersedia: halaman menampilkan status belum siap dan menolak pengiriman tindakan yang belum dapat diproses; tombol Buat backup tetap tampil stabil untuk membuka dialog. Login/SSO tetap berjalan. Jika PostgreSQL belum siap saat startup, service backend harus direstart oleh process manager seperti systemd; jadwal tersimpan akan dibaca setelah koneksi tersedia.

### File kunci terpisah (opsional)

Backend juga dapat membaca dua kunci dari file privat di luar repository. Salin **hanya** `backend-backup.env` dari recovery kit ke direktori konfigurasi backend berizin 0700, dengan file berizin 0600. Simpan `recovery.agekey` tetap terpisah. Pada `.env` backend isi:

```dotenv
BACKUP_ENABLED=true
BACKUP_KEYS_FILE=/path/absolut/konfigurasi/backend-backup.env
```

Biarkan `BACKUP_AGE_RECIPIENT` dan `BACKUP_R2_SSE_KEY` tidak terisi pada `.env`/environment proses jika memakai cara ini. File hanya menjadi sumber dua kunci tersebut, bukan pengganti seluruh konfigurasi backend. `BACKUP_IDENTITY_FILE` disetel terpisah untuk unduhan SQL; tanpa identity, backup tetap berjalan tetapi unduhan SQL nonaktif. Symlink, file yang dapat diakses group/other, serta gabungan dua sumber kunci ditolak agar tidak salah memilih kunci. Restart backend setelah perubahan.

## Pemulihan (operator, bukan aksi dashboard)

Backup meliputi isi **satu database aplikasi PostgreSQL**, bukan seluruh cluster, role/password PostgreSQL, file avatar R2, konfigurasi `.env`, atau private signing/encryption keys aplikasi. Simpan konfigurasi dan kunci aplikasi secara terpisah: ciphertext OAuth/email dalam database masih membutuhkan kunci aplikasi aslinya. Backup mingguan memungkinkan kehilangan perubahan sejak backup terakhir (hingga sekitar satu minggu saat jadwal normal); buat manual sebelum perubahan penting.

### File SQL dari dashboard

Gunakan [panduan impor SQL](README.md#impor-ke-postgresql-lokal-atau-server). Dekripsi sudah diselesaikan backend sebelum file diunduh. Jangan menjalankan `backup-decrypt` atau `pg_restore` langsung pada file `.sql`; impor dengan `psql` atau native restore yang menjalankan psql.

### Arsip `.dump.age` lama atau diambil langsung dari R2

Bagian ini **hanya** untuk arsip `.dump.age` lama atau pemulihan darurat dari R2 ketika dashboard tidak tersedia. Ini bukan format unduhan dashboard saat ini. Jika dashboard masih tersedia, pilih snapshot yang sama lalu **Unduh SQL**; backend akan menyiapkannya tanpa dekripsi manual.

Arsip R2 tetap `.dump.age`. Untuk pemulihan darurat, catat SHA-256 dari riwayat/metadata dan bandingkan checksum, lalu dekripsi di komputer pemulihan tepercaya:

```sh
shasum -a 256 /path/backup.dump.age
go run ./cmd/backup-decrypt -identity /path/recovery.agekey -in /path/backup.dump.age -out /path/baru/restore.dump
pg_restore --list /path/baru/restore.dump
```

Perintah dekripsi menolak menimpa file yang ada, memverifikasi autentikasi age sampai akhir, dan menghapus output parsial jika rusak. File `.dump` adalah **plaintext sensitif**, gunakan direktori/disk privat terenkripsi. `pg_restore --list` hanya memeriksa daftar arsip; lakukan juga restore penuh ke database kosong terisolasi dengan PostgreSQL major yang sesuai:

```sh
pg_restore --no-owner --no-acl --exit-on-error --dbname=DATABASE_UJI_KOSONG /path/baru/restore.dump
```

Gunakan kredensial koneksi database uji (misalnya `.pgpass` privat); jangan menaruh password pada argumen command atau menjalankan contoh ini ke database aktif. Periksa data dan konsistensi aplikasi sebelum memutuskan pemulihan produksi. Jangan mengeksekusi arsip dari sumber tidak dipercaya.

Jika server hilang, operator masih dapat mengambil objek melalui S3 API R2 memakai kredensial bucket **dan kunci SSE-C** dari recovery kit, kemudian menjalankan dekripsi age. Nama objek berformat UUID `.dump.age` di namespace instance; metadata objek mencantumkan checksum SHA-256. Kunci SSE-C wajib dikirim sebagai header `x-amz-server-side-encryption-customer-key` (base64) beserta algorithm `AES256` dan key-MD5, melalui HTTPS. Gunakan SDK/klien S3 yang mendukung SSE-C, bukan URL publik.

Sebelum aplikasi hasil restore dihubungkan kembali ke pengguna: hentikan worker, cabut sesi/token autentikasi lama, batalkan token reset/OTP yang tidak lagi relevan, tinjau outbox email/provisioning agar pekerjaan lama tidak terkirim ulang, dan tinjau ulang jadwal/status backup. Restore akan mengembalikan state antrean pada waktu snapshot. Uji alur login pada lingkungan terisolasi terlebih dahulu; jangan membuka hasil restore langsung ke publik.

Jangan mengganti `BACKUP_R2_SSE_KEY` sembarangan: file lama masih memerlukan kunci lama. Implementasi memakai satu kunci storage aktif per instance; lakukan migrasi/re-enkripsi objek dengan prosedur operator sebelum rotasi. Jika recipient age diubah, pertahankan seluruh private identity lama selama backup terkait masih disimpan.
