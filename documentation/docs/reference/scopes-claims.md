---
title: Scope dan claim
description: Data identitas yang diterbitkan oleh scope openid, profile, dan email.
---

# Scope dan claim

Minta scope minimum yang dibutuhkan. Scope yang diminta harus merupakan subset dari scope yang diizinkan saat client didaftarkan.

| Scope | ID token | UserInfo |
| --- | --- | --- |
| `openid` | Mengaktifkan ID token; claim inti `iss`, `sub`, `aud`, `exp`, `iat`, `auth_time`, `nonce` | `sub` (dan alias `id`) |
| `profile` | `name` | `name`, `phone`, `bio`, `gender`, `avatar` |
| `email` | `email`, `email_verified` | `email`, `email_verified` |

## Subject sebagai kunci identitas

Simpan akun eksternal dengan kunci gabungan:

```text
(issuer, sub)
```

Jangan memakai email sebagai primary key federasi karena email dapat berubah, berbeda aturan normalisasinya, atau digunakan lintas issuer. `sub` adalah string opaque; jangan menebak formatnya.

## Email terverifikasi

SSO menolak autentikasi akun yang emailnya belum terverifikasi. Meski demikian, RP tetap harus memeriksa `email_verified=true` sebelum memakai email untuk komunikasi sensitif atau account linking.

## Role dan permission

IPNU IPPNU ID tidak mengirim role platform `super_admin`/`anggota`, role bisnis, atau entitlement ke relying party. Assignment hanya menjadi gerbang boleh masuk. Aplikasi tujuan mengelola role dan permission sendiri setelah menghubungkan akun berdasarkan `(iss, sub)`.

## Claim ID token vs data terbaru

ID token adalah pernyataan autentikasi pada waktu login dan berlaku satu jam. Untuk profil terbaru, panggil UserInfo dengan access token aktif. Refresh token response tidak menerbitkan ID token baru; bila aplikasi membutuhkan autentikasi/claims baru, lakukan authorization flow baru.

## Contoh UserInfo

```json
{
  "sub": "<subject-id>",
  "id": "<subject-id>",
  "name": "Nama Anggota",
  "email": "anggota@example.com",
  "email_verified": true
}
```

Field kosong tetap dapat muncul untuk atribut profil yang belum diisi. Treat unknown claims as ignorable dan jangan memberi hak akses berdasarkan claim yang tidak terdokumentasi.
