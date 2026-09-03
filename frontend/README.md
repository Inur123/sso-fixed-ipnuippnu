# Portal PelajarNU Magetan ID

Portal Next.js 16 berbasis App Router, TypeScript, Tailwind CSS 4, dan komponen shadcn/ui dengan preset Radix Nova.

```bash
cp .env.example .env.local
npm install
npm run dev
```

Frontend hanya memerlukan konfigurasi publik berawalan `NEXT_PUBLIC_`. Jangan
menyimpan SMTP, database password, JWT secret, atau OAuth client secret pada
environment tersebut.

Alamat layanan lokal harus berasal dari env, bukan literal localhost di kode.
Gunakan `NEXT_PUBLIC_BACKEND_URL` dan `NEXT_PUBLIC_DOCUMENTATION_URL` di
`.env.local` untuk lokal, serta `.env.production` untuk build produksi.
URL domain eksternal seperti Short URL dan script Turnstile tetap dipertahankan.
Variabel `NEXT_PUBLIC_*` tertanam saat build, sehingga perubahan env produksi
memerlukan build ulang. Pemeriksaan artifact menolak URL localhost pada port
apa pun, termasuk port frontend, backend, dan dokumentasi.

Untuk development gunakan:

```bash
npm run dev
```

Untuk artifact produksi wajib gunakan:

```bash
npm run build:production
```

Perintah produksi membaca `.env.production` secara eksplisit, mengoverride
`.env.local`, lalu memvalidasi chunk hasil build. Build akan gagal jika endpoint
API/dokumentasi masih menunjuk ke localhost atau URL produksi tidak tertanam.

Halaman utama:

- `/login`, `/register`, dan `/verify-email`
- `/dashboard/profil`
- `/dashboard/keamanan`
- `/dashboard/sesi` untuk grant aplikasi SSO yang terhubung
- `/dashboard/aplikasi` untuk pengelolaan OAuth client milik anggota atau super admin
- `/dashboard/pengguna` untuk pengelolaan pengguna oleh super admin
- `/dashboard/audit-log` untuk riwayat aktivitas keamanan oleh super admin
- `/oauth/authorize` untuk persetujuan akses aplikasi

Jalankan pemeriksaan sebelum deploy:

```bash
npm run lint
npx tsc --noEmit
npm run build
```

Script build menggunakan Webpack karena Turbopack Next.js 16 dapat gagal pada lingkungan terisolasi yang melarang port internal saat transformasi CSS.
