# IPNU IPPNU Magetan ID

Pusat identitas dan Single Sign-On resmi PC IPNU IPPNU Kabupaten Magetan.

Identity Provider terpusat untuk ekosistem aplikasi IPNU dan IPPNU. Backend menggunakan Go, Gin, GORM, dan PostgreSQL; portal akun menggunakan Next.js, TypeScript, Tailwind CSS, dan komponen shadcn/ui. Seluruh project dijalankan langsung tanpa Docker.

## Fitur utama

- Registrasi anggota dengan akun langsung aktif dan verifikasi email menggunakan OTP enam digit.
- Login menolak akun yang belum memverifikasi email atau dinonaktifkan super admin.
- Dua role sistem: `super_admin` dan `anggota`.
- Manajemen pengguna untuk super admin: pencarian, pagination, perubahan role, aktivasi/nonaktivasi, serta penghapusan akun permanen.
- Audit log khusus super admin untuk aktivitas autentikasi, administrasi pengguna, dan grant OAuth.
- Client secret unik per aplikasi, tersimpan terenkripsi untuk reveal terkontrol, serta dapat diregenerasi saat bocor.
- OAuth 2.0 Authorization Code dengan PKCE S256 dan OpenID Connect RS256/JWKS.
- Exact redirect URI matching, authorization code sekali pakai, refresh-token rotation, reuse detection, dan revocation.
- Halaman **Sesi aplikasi** menampilkan aplikasi yang masih memiliki grant SSO aktif, tanpa menampilkan access token atau refresh token.
- Portal responsif berbasis shadcn/ui untuk profil, keamanan, aplikasi OAuth, consent, dan administrasi.

## Struktur konfigurasi

Konfigurasi sengaja dipisahkan agar URL backend dan frontend tidak tertukar saat deployment:

| Variabel | Lokasi | Fungsi |
| --- | --- | --- |
| `BACKEND_PUBLIC_URL` | `backend/.env` | URL publik API sekaligus issuer OAuth/OIDC |
| `FRONTEND_PUBLIC_URL` | `backend/.env` | URL publik portal yang dipakai backend untuk redirect |
| `BACKEND_CORS_ALLOWED_ORIGINS` | `backend/.env` | Daftar origin frontend yang boleh mengakses backend, dipisahkan koma |
| `SESSION_COOKIE_NAME` | `backend/.env` | Nama cookie sesi HttpOnly backend |
| `SESSION_COOKIE_DOMAIN` | `backend/.env` | Domain bersama cookie bila backend dan frontend memakai subdomain berbeda |
| `NEXT_PUBLIC_BACKEND_URL` | `frontend/.env.local` | URL backend yang diakses browser |
| `BACKEND_SESSION_COOKIE_NAME` | `frontend/.env.local` | Nama cookie sesi backend yang dibaca Next.js ketika server-render |
| `CLIENT_SECRET_ENCRYPTION_KEY` | `backend/.env` | Kunci AES-256 untuk penyimpanan client secret yang dapat dilihat ulang |

Semua variabel `MAIL_*`, database, JWT, dan client secret hanya boleh berada di backend. Jangan menaruh rahasia pada variabel `NEXT_PUBLIC_*` karena nilainya dapat dibaca browser.

Contoh development sudah tersedia di [`backend/.env.example`](backend/.env.example) dan [`frontend/.env.example`](frontend/.env.example).

Untuk OpenID Connect, development dapat membuat RSA key sementara saat backend dimulai. Production wajib mengisi `OIDC_PRIVATE_KEY_PATH` dengan path private key RSA persisten; public key diterbitkan melalui `/oauth/jwks`. Pisahkan pula `OTP_HASH_SECRET` dari `JWT_SECRET`, dan simpan `CLIENT_SECRET_ENCRYPTION_KEY` 32-byte di secret manager.

## Menjalankan tanpa Docker

Prasyarat: Go sesuai versi pada `backend/go.mod`, Node.js yang didukung Next.js 16, dan PostgreSQL lokal.

1. Buat database:

   ```bash
   createdb ipnu_ippnu_id_sso
   ```

2. Siapkan dan jalankan backend:

   ```bash
   cd backend
   cp .env.example .env
   # Sesuaikan koneksi PostgreSQL, JWT_SECRET, SUPER_ADMIN_EMAIL, dan SMTP.
   go run .
   ```

3. Pada terminal lain, siapkan dan jalankan frontend:

   ```bash
   cd frontend
   cp .env.example .env.local
   npm install
   npm run dev
   ```

Portal tersedia di `http://localhost:3000`, sedangkan backend dan issuer berada di `http://localhost:8080`.

## Alur akun dan super admin

1. Registrasi publik selalu membuat role `anggota` dengan status aktif.
2. Sistem mengirim OTP ke email pengguna. Pengguna belum dapat login sebelum OTP valid diverifikasi.
3. Super admin dapat menonaktifkan akun dari **Dashboard → Pengguna**. Penonaktifan langsung mencabut sesi browser, authorization code, dan token OAuth pengguna tersebut.
4. Untuk super admin pertama, isi `SUPER_ADMIN_EMAIL`, lalu registrasikan dan verifikasi alamat tersebut. Transaksi verifikasi OTP langsung mempromosikan akun yang cocok menjadi `super_admin`; bootstrap saat backend dimulai juga memperbaiki role akun lama yang sudah terverifikasi.

SMTP Gmail memerlukan App Password, bukan password akun utama. Untuk production, gunakan kredensial khusus aplikasi dan rotasikan segera jika pernah terekspos.

## Integrasi aplikasi SSO

Setiap anggota terverifikasi dapat mendaftarkan aplikasi miliknya melalui **Dashboard → Aplikasi**. Setiap client memperoleh secret acak yang berbeda. Portal menyimpannya terenkripsi agar pemilik dapat melihat ulang; tombol regenerate mengganti secret sekaligus mencabut token dan authorization code lama. Semua client wajib menggunakan PKCE S256 dan menjaga `client_secret` di server aplikasi.

Endpoint discovery dan protokol:

- OAuth metadata: `/.well-known/oauth-authorization-server`
- OpenID configuration: `/.well-known/openid-configuration`
- Authorization: `/oauth/authorize`
- Token: `/oauth/token`
- Revocation: `/oauth/revoke`
- JSON Web Key Set: `/oauth/jwks`
- UserInfo: `/v1/user/me`

Request authorization wajib memakai `response_type=code`, `state`, exact `redirect_uri`, dan PKCE `S256`. Gunakan `nonce` saat meminta scope `openid`. Sesi aplikasi di portal merepresentasikan grant aplikasi yang masih aktif, bukan daftar browser atau perangkat. Tombol **Cabut akses** menghentikan token/grant di Identity Provider; sesi lokal yang sudah dibuat aplikasi klien baru berakhir bila aplikasi tersebut juga mengimplementasikan logout sendiri atau OIDC front/back-channel logout.

Setiap aplikasi dapat memakai policy `assigned_only` (default aman) atau `all_active_users`. Pada policy terbatas, pemilik aplikasi menambahkan pengguna berdasarkan UUID atau email secara persis. Halaman detail aplikasi memisahkan informasi umum client dari daftar pengguna yang ditugaskan dan memuat daftar tersebut secara terpaginasikan. Pemeriksaan dilakukan saat authorization, pertukaran code/refresh, dan UserInfo; pencabutan assignment langsung mencabut grant lama.

Role `super_admin` dan `anggota` adalah otoritas internal platform dan tidak dibagikan ke relying party. Role serta permission bisnis dikelola sendiri oleh aplikasi tujuan dengan identitas stabil `(iss, sub)`.

Panduan integrasi lengkap tersedia sebagai situs Docusaurus terpisah di folder [`documentation`](documentation/README.md). Untuk menjalankannya secara lokal:

```bash
cd documentation
npm install
npm run start
```

Dokumentasi akan tersedia di `http://localhost:3001` dan memuat quickstart, Authorization Code + PKCE, validasi ID token/JWKS, contoh integrasi framework, revocation, serta checklist produksi.

## Checklist production

- Set `APP_ENV=production`.
- Gunakan HTTPS untuk `BACKEND_PUBLIC_URL` dan `FRONTEND_PUBLIC_URL`.
- Gunakan `JWT_SECRET` acak minimal 32 karakter dan kredensial database khusus aplikasi.
- Gunakan `OTP_HASH_SECRET` yang berbeda dan private key RSA persisten lewat `OIDC_PRIVATE_KEY_PATH`.
- Gunakan `CLIENT_SECRET_ENCRYPTION_KEY` base64 32-byte yang berbeda dan simpan di secret manager.
- Set `DB_SSLMODE=verify-full` (atau `verify-ca` jika keterbatasan penyedia sudah dipahami).
- Batasi `BACKEND_CORS_ALLOWED_ORIGINS` hanya ke domain frontend resmi.
- Samakan `SESSION_COOKIE_NAME` dengan `BACKEND_SESSION_COOKIE_NAME`. Jika frontend dan backend berbeda subdomain, isi `SESSION_COOKIE_DOMAIN` dengan parent domain yang menaungi keduanya.
- Simpan file `.env` di server/secret manager dan jangan commit ke Git.
- Pastikan SMTP menggunakan App Password yang masih valid.

## Validasi

```bash
cd backend
go test ./...
go vet ./...

cd ../frontend
npm run lint
npx tsc --noEmit
npm run build

cd ../documentation
npm run typecheck
npm run build
```
