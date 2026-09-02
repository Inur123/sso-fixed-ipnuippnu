# Changelog

Semua perubahan penting pada PelajarNU Magetan ID dicatat di sini. Format
mengikuti [Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/) dan versi
mengikuti [Semantic Versioning](https://semver.org/lang/id/).

## [Unreleased]

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

[Unreleased]: https://github.com/Inur123/sso-fixed-ipnuippnu/compare/v2.0.0...HEAD
[2.0.0]: https://github.com/Inur123/sso-fixed-ipnuippnu/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/Inur123/sso-fixed-ipnuippnu/releases/tag/v1.0.0
