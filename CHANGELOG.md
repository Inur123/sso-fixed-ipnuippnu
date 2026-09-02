# Changelog

Semua perubahan penting pada proyek ini dicatat di sini. Format mengikuti
[Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/) dan versi mengikuti
[Semantic Versioning](https://semver.org/lang/id/).

## [Unreleased]

## [0.1.0] - 2026-09-02

### Added

- Portal identitas PelajarNU Magetan ID berbasis Next.js dan backend Go.
- Registrasi anggota dengan verifikasi email OTP melalui antrean persisten.
- Enkripsi payload OTP dan retry pengiriman email secara asinkron.
- Perlindungan Cloudflare Turnstile pada registrasi dengan validasi server-side.
- OAuth 2.0 Authorization Code + PKCE dan OpenID Connect dengan JWKS.
- Dashboard akun, aplikasi SSO, sesi, keamanan, dan administrasi pengguna.
- Kartu layanan Short URL menuju `s.pelajarnumagetan.or.id`.
- Pemeriksaan CI untuk backend, frontend, dan dokumentasi.

### Changed

- Identitas produk diseragamkan menjadi PelajarNU Magetan ID.
- Konfigurasi deployment production diperketat untuk menolak key Turnstile test.
- Tampilan portal dan halaman autentikasi disempurnakan untuk desktop dan mobile.

[Unreleased]: https://github.com/Inur123/sso-fixed-ipnuippnu/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/Inur123/sso-fixed-ipnuippnu/releases/tag/v0.1.0
