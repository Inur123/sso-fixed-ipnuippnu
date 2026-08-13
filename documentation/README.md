# Dokumentasi IPNU IPPNU ID

Situs dokumentasi integrasi SSO yang berdiri sendiri, dibangun dengan Docusaurus 3.10.2 dalam mode docs-only.

## Menjalankan lokal

Persyaratan: Node.js 20 atau lebih baru.

```bash
cd documentation
npm install
npm run start
```

Buka `http://localhost:3001`.

## Build statis

```bash
npm run typecheck
npm run build
```

Hasil build berada di `documentation/build/`. Untuk deployment, isi `DOCS_SITE_URL` dengan origin dokumentasi sebenarnya dan, bila situs tidak dipasang di root, isi `DOCS_BASE_URL` dengan path yang diawali dan diakhiri `/`. Sidebar sengaja dibuat ringkas: mulai, cara kerja SSO, contoh integrasi, dan referensi.
