# Portal IPNU IPPNU ID

Portal Next.js 16 berbasis App Router, TypeScript, Tailwind CSS 4, dan komponen shadcn/ui dengan preset Radix Nova.

```bash
cp .env.example .env.local
npm install
npm run dev
```

Frontend hanya memerlukan `NEXT_PUBLIC_BACKEND_URL`. Jangan menyimpan SMTP, database password, JWT secret, atau OAuth client secret pada environment yang diawali `NEXT_PUBLIC_`.

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
