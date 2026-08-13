---
title: Quickstart
sidebar_position: 2
description: Alur singkat menghubungkan aplikasi ke SSO IPNU IPPNU ID.
---

# Quickstart

Contoh ini memakai issuer `http://localhost:8080` dan callback aplikasi `http://localhost:3002/auth/callback`.

## 1. Buat aplikasi

Buka **Dashboard → Aplikasi OAuth → Tambah aplikasi**, lalu daftarkan callback:

```text
http://localhost:3002/auth/callback
```

Simpan kredensial di server aplikasi:

```dotenv
SSO_ISSUER=http://localhost:8080
SSO_CLIENT_ID=<client-id>
SSO_CLIENT_SECRET=<client-secret>
SSO_REDIRECT_URI=http://localhost:3002/auth/callback
```

## 2. Atur siapa yang boleh masuk

Untuk aplikasi internal, pilih kebijakan **Pengguna yang ditugaskan**, lalu tambahkan UUID atau email pengguna secara persis. Untuk layanan yang terbuka bagi seluruh anggota aktif, pilih **Semua pengguna aktif**.

Role aplikasi seperti `viewer` atau `operator` dikelola oleh aplikasi tujuan dan berbeda dari role portal `anggota`/`super_admin`. Baca [akses per aplikasi](../protocol/application-access.md) sebelum membuka client untuk production.

## 3. Ambil konfigurasi OIDC

```bash
curl -sS http://localhost:8080/.well-known/openid-configuration
```

Gunakan endpoint dari respons discovery. Jangan menebak URL endpoint ketika masuk production.

## 4. Arahkan pengguna ke login

Server aplikasi membuat `state`, `nonce`, dan `code_verifier` acak. Hitung `code_challenge` dengan SHA-256, lalu arahkan browser ke:

```text
http://localhost:8080/oauth/authorize
  ?response_type=code
  &client_id=<client-id>
  &redirect_uri=http%3A%2F%2Flocalhost%3A3002%2Fauth%2Fcallback
  &scope=openid%20profile%20email
  &state=<state>
  &nonce=<nonce>
  &code_challenge=<challenge>
  &code_challenge_method=S256
```

Simpan `state`, `nonce`, dan `code_verifier` di sesi server yang singkat—bukan di local storage.

## 5. Tangani callback

SSO mengembalikan `code`, `state`, dan `iss`. Sebelum menukar code:

1. pastikan `state` sama dengan nilai di sesi;
2. pastikan `iss` sama persis dengan `SSO_ISSUER`;
3. bila terdapat `error`, termasuk `access_denied`, hentikan alur tanpa membuat sesi;
4. hapus transaksi login agar hanya bisa dipakai satu kali.

## 6. Tukar code dari server

```bash
curl -sS -X POST http://localhost:8080/oauth/token \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'grant_type=authorization_code' \
  --data-urlencode 'client_id=<client-id>' \
  --data-urlencode 'client_secret=<client-secret>' \
  --data-urlencode 'redirect_uri=http://localhost:3002/auth/callback' \
  --data-urlencode 'code=<authorization-code>' \
  --data-urlencode 'code_verifier=<code-verifier>'
```

## 7. Validasi lalu buat sesi lokal

Validasi ID token `RS256` memakai `jwks_uri` dari discovery. Periksa `kid`, signature, `iss`, `aud`, `exp`, `iat`, dan `nonce`. Setelah valid, buat cookie sesi lokal aplikasi yang `HttpOnly`, `Secure`, dan memiliki `SameSite` yang sesuai.

Untuk profil terbaru, panggil UserInfo:

```bash
curl -sS http://localhost:8080/v1/user/me \
  -H 'Authorization: Bearer <access-token>'
```

Gunakan pasangan `(issuer, sub)` sebagai ID pengguna SSO. Jangan memakai email sebagai kunci utama federasi.

Selanjutnya pilih [contoh Next.js](../integrations/nextjs-node.md), [Laravel](../integrations/laravel.md), atau [Go](../integrations/go.md).
