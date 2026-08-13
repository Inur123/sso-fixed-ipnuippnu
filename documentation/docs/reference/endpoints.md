---
title: Referensi endpoint
description: Daftar endpoint publik OAuth 2.0 dan OpenID Connect IPNU IPPNU ID.
---

# Referensi endpoint

Gunakan origin issuer dari konfigurasi lingkungan. Tabel menggunakan `http://localhost:8080` sebagai contoh lokal.

| Metode | Path | Pemanggil | Fungsi |
| --- | --- | --- | --- |
| `GET` | `/.well-known/openid-configuration` | Server RP | Metadata OpenID Provider |
| `GET` | `/.well-known/oauth-authorization-server` | Server RP | Metadata OAuth Authorization Server |
| `GET` | `/oauth/jwks` | Server RP | Public key ID token RS256 |
| `GET` | `/oauth/authorize` | Browser | Memulai authorization dan mengarahkan ke login/consent |
| `POST` | `/oauth/token` | Server RP | Menukar code atau merotasi refresh token |
| `POST` | `/oauth/revoke` | Server RP | Mencabut grant berdasarkan access/refresh token |
| `GET` | `/v1/user/me` | Server RP | UserInfo sesuai scope access token |
| `GET` | `/api/oauth/client-info` | Portal consent | Nama/deskripsi client berdasarkan `client_id` + `redirect_uri` |

## Authorization endpoint

```http
GET /oauth/authorize
```

Query: `response_type=code`, `client_id`, `redirect_uri`, `scope`, `state`, `nonce` (wajib untuk `openid`), `code_challenge`, dan `code_challenge_method=S256`.

Endpoint ini adalah navigasi browser, bukan request backend dengan `fetch`. Jika redirect URI tidak dipercaya, server mengembalikan error lokal dan tidak redirect ke URI tersebut.

## Token endpoint

```http
POST /oauth/token
Content-Type: application/x-www-form-urlencoded
```

### Grant `authorization_code`

Field: `grant_type`, `client_id`, `client_secret`, `code`, `redirect_uri`, `code_verifier`.

### Grant `refresh_token`

Field: `grant_type`, `client_id`, `client_secret`, `refresh_token`.

Autentikasi client yang diumumkan discovery adalah `client_secret_post`. Jangan mengirim secret melalui query string. Respons token tidak boleh di-cache.

## Revocation endpoint

```http
POST /oauth/revoke
Content-Type: application/x-www-form-urlencoded
```

Field: `token`, `client_id`, `client_secret`. `token` boleh berupa access token atau refresh token. Bila dikenal, seluruh family grant yang terkait dicabut. Token yang tidak dikenal tetap menghasilkan HTTP `200` sesuai semantik RFC 7009.

Implementasi saat ini tidak memakai `token_type_hint`; jangan mengirim field yang tidak diperlukan.

## UserInfo

```http
GET /v1/user/me
Authorization: Bearer <access-token>
```

Respons selalu memiliki `sub` dan alias nonstandar `id`; field lain mengikuti scope. Gunakan `sub`, bukan alias `id`, untuk kode OIDC portabel. Respons memakai `Cache-Control: no-store`.

## Endpoint manajemen client di portal

Endpoint berikut memakai cookie sesi portal. Anggota mengelola client miliknya; super admin dapat membantu mengelola seluruh client:

| Metode | Path | Fungsi |
| --- | --- | --- |
| `GET` | `/api/clients` | Daftar client; `secret_available` menandai apakah secret terenkripsi tersedia |
| `POST` | `/api/clients` | Membuat client dan secret acak unik |
| `GET` | `/api/clients/:id` | Detail satu client milik pengguna yang sedang login |
| `PATCH` | `/api/clients/:id` | Mengubah nama, deskripsi, dan Redirect URI client |
| `GET` | `/api/clients/:id/secret` | Membuka secret; respons `Cache-Control: no-store` |
| `POST` | `/api/clients/:id/secret/regenerate` | Mengganti secret dengan `expected_version` dan mencabut token/code client lama |
| `GET` | `/api/clients/:id/assignments` | Detail client dan daftar assignment terpaginasikan; mendukung `page`, `page_size`, dan `search` |
| `POST` | `/api/clients/:id/assignment-lookup` | Memeriksa satu UUID/email exact sebelum assignment dibuat |
| `POST` | `/api/clients/:id/assignments` | Menambah assignment dengan body `identifier` berisi UUID atau email persis |
| `DELETE` | `/api/clients/:id/assignments/:userId` | Menghapus assignment dan mencabut grant pengguna |
| `DELETE` | `/api/clients/:id` | Menghapus client dan seluruh grant terkait |

Endpoint reveal/regenerate hanya untuk pemilik client melalui dashboard, bukan mekanisme distribusi credential otomatis ke RP. `expected_version` mencegah dua rotasi paralel menghasilkan respons credential yang sudah usang. Kirim credential ke konfigurasi server RP melalui kanal rahasia.

## Endpoint portal bukan API RP

Path `/api/auth/*`, `/api/clients`, dan `/api/connections` adalah API portal IPNU IPPNU ID berbasis cookie sesi. Aplikasi RP tidak boleh bergantung pada cookie portal tersebut. Integrasi RP hanya memakai endpoint protokol publik di tabel awal halaman ini.
