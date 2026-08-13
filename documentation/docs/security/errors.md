---
title: Error dan penanganannya
description: Bentuk error protokol, status HTTP, retry, dan respons aplikasi yang aman.
---

# Error dan penanganannya

## Bentuk respons

Token endpoint:

```json
{
  "error": "invalid_grant",
  "error_description": "<deskripsi aman>",
  "message": "<deskripsi aman>"
}
```

API lain umumnya:

```json
{
  "error": "invalid_token",
  "message": "<deskripsi aman>"
}
```

Authorization error yang terjadi setelah redirect URI dipercaya disampaikan melalui callback (`error`, `error_description`, `state`, `iss`) atau `redirect_url` dari consent API internal. RP tidak menggunakan consent API internal.

## Kode yang relevan bagi RP

| HTTP | `error` | Arti | Tindakan RP |
| --- | --- | --- | --- |
| 400 | `invalid_request` | Parameter hilang/salah, redirect atau PKCE tidak valid | Perbaiki request; jangan retry identik |
| 400 | `invalid_scope` | Scope bukan subset izin client | Kurangi scope atau ubah registrasi client |
| redirect | `access_denied` | Pengguna membatalkan consent atau tidak memiliki assignment ke aplikasi | Jangan membuat sesi; tampilkan pesan aman atau arahkan ke pengelola aplikasi |
| 400 | `invalid_grant` | Code/refresh invalid, expired, reused, redirect atau verifier salah | Hapus transaksi/token; mulai login baru |
| 400 | `unsupported_grant_type` | Grant tidak didukung | Gunakan `authorization_code` atau `refresh_token` |
| 401 | `invalid_client` | Client ID/secret salah atau client dihapus | Hentikan retry; rotasi/perbaiki kredensial |
| 401 | `invalid_token` | Access token invalid, expired, revoked, client/user tidak valid | Hapus grant/sesi lokal; login ulang |
| 429 | `rate_limited` | Batas request terlampaui | Backoff dengan jitter; jangan loop login |
| 500 | `server_error` | Kegagalan sementara issuer | Tampilkan pesan generik; retry terbatas bila aman |

Pengguna yang membatalkan atau menolak consent juga harus kembali ke halaman aplikasi yang aman tanpa membuat sesi.

`access_denied` tidak boleh diatasi dengan retry otomatis. Bila akses memang diperlukan, pemilik aplikasi atau administrator harus memperbarui assignment/grup pengguna terlebih dahulu.

## Kebijakan retry

- Authorization code exchange: jangan retry paralel; code sekali pakai. Jika hasil tidak pasti, mulai login baru.
- Refresh: jangan retry dengan token lama ketika server mungkin sudah merotasinya; login ulang lebih aman.
- Discovery/JWKS: retry terbatas dengan exponential backoff dan gunakan cache valid terakhir.
- UserInfo `5xx`: retry terbatas bila access token masih valid; jangan memberi akses baru berdasarkan data parsial.
- Revocation: setelah logout, hapus state lokal terlepas dari hasil jaringan.

## Logging aman

Log correlation ID, endpoint, status, kode error, client ID, dan timestamp. Jangan log:

- client secret;
- authorization code atau URL callback utuh;
- access, refresh, atau ID token;
- code verifier, state, nonce, OTP, atau cookie;
- payload profil lengkap tanpa kebutuhan audit yang sah.

Jangan tampilkan `error_description` mentah sebagai HTML. Map kode ke pesan pengguna Bahasa Indonesia dan escape semua data eksternal.
