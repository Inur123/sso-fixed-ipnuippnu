---
title: Authorization Code + PKCE S256
description: Alur protokol OAuth 2.0/OIDC yang didukung IPNU IPPNU ID.
---

# Authorization Code + PKCE S256

IPNU IPPNU ID hanya mendukung `response_type=code`, grant `authorization_code` dan `refresh_token`, serta PKCE `S256`. Confidential client mengautentikasi dirinya sesuai metode yang diumumkan discovery. Public client tidak mengirim secret dan mengandalkan PKCE untuk mengikat code ke peminta awal.

## Parameter transaksi

RP membuat empat nilai untuk setiap percobaan login:

| Nilai | Tujuan | Penyimpanan |
| --- | --- | --- |
| `state` | Mengikat callback ke transaksi dan mencegah login CSRF | Sesi server, satu kali, berumur pendek |
| `nonce` | Mengikat ID token ke permintaan OIDC | Sesi server, satu kali, berumur pendek |
| `code_verifier` | Bukti rahasia PKCE saat token exchange | Sesi server, satu kali, berumur pendek |
| `code_challenge` | Hash verifier yang dikirim saat authorization | Parameter publik |

Verifier harus 43–128 karakter dari karakter unreserved RFC 7636. Praktik sederhana adalah menghasilkan 32–64 byte acak lalu encode Base64 URL-safe tanpa padding.

```js
import {createHash, randomBytes} from 'node:crypto';

const base64url = (input) => Buffer.from(input).toString('base64url');
const state = base64url(randomBytes(32));
const nonce = base64url(randomBytes(32));
const codeVerifier = base64url(randomBytes(32)); // panjang 43 karakter
const codeChallenge = createHash('sha256')
  .update(codeVerifier, 'ascii')
  .digest('base64url');
```

## 1. Authorization request

Redirect browser dengan `GET` ke endpoint dari discovery:

```http
GET /oauth/authorize?response_type=code
  &client_id=<client-id>
  &redirect_uri=<redirect-uri-yang-di-encode>
  &scope=openid%20profile%20email
  &state=<state>
  &nonce=<nonce>
  &code_challenge=<challenge>
  &code_challenge_method=S256 HTTP/1.1
Host: localhost:8080
```

Semua parameter berikut wajib: `response_type`, `client_id`, `redirect_uri`, `scope`, `state`, `code_challenge`, dan `code_challenge_method`. `nonce` wajib ketika scope memuat `openid`.

Authorization server memvalidasi client, exact redirect URI, scope, PKCE, akun aktif, email terverifikasi, serta assignment aplikasi. Browser kemudian melihat layar login/consent portal.

Persetujuan scope disimpan per pasangan pengguna dan aplikasi. Request berikutnya
tidak meminta persetujuan yang sama lagi selama scope tidak bertambah dan grant
belum dicabut. Gunakan `prompt=select_account` bila RP ingin selalu menampilkan
pemilih akun SSO; pengguna dapat melanjutkan dengan akun yang sedang aktif atau
memilih **Gunakan akun lain**. Logout lokal RP maupun berakhirnya sesi login SSO
tidak menghapus persetujuan scope. Persetujuan diminta lagi hanya ketika pengguna
menekan **Cabut akses**, assignment dicabut lalu diberikan kembali, scope bertambah,
atau RP secara eksplisit mengirim `prompt=consent`.

## 2. Authorization response

Sukses:

```text
http://localhost:3002/auth/callback?code=<code>&state=<state>&iss=http%3A%2F%2Flocalhost%3A8080
```

Error setelah redirect URI dipercaya:

```text
http://localhost:3002/auth/callback?error=invalid_request&error_description=<pesan>&state=<state>&iss=http%3A%2F%2Flocalhost%3A8080
```

RP harus memeriksa `state` dan `iss` sebelum memproses `code` maupun error. Authorization code bersifat sekali pakai dan berlaku 5 menit.

Jika akun valid tetapi tidak ditugaskan ke client terbatas, error menggunakan `access_denied`. Ini keputusan authorization, bukan kegagalan password.

## 3. Token exchange

RP mengirim form URL-encoded. `redirect_uri` harus sama persis dengan authorization request dan `code_verifier` harus menghasilkan challenge sebelumnya.

```http
POST /oauth/token HTTP/1.1
Host: localhost:8080
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code&
client_id=<client-id>&
client_secret=<client-secret-confidential-client>&
redirect_uri=http%3A%2F%2Flocalhost%3A3002%2Fauth%2Fcallback&
code=<authorization-code>&
code_verifier=<verifier>
```

```json title="Contoh bentuk respons"
{
  "access_token": "<opaque-access-token>",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "<refresh-token>",
  "scope": "email openid profile",
  "id_token": "<compact-JWS-RS256>"
}
```

`id_token` hanya diterbitkan jika grant meminta `openid`. Header respons token memakai `Cache-Control: no-store`.

Public client menghilangkan `client_secret`. Confidential client tidak boleh mengirim secret melalui browser atau query string.

## 4. UserInfo

Setelah ID token lolos validasi, panggil UserInfo dengan access token:

```http
GET /v1/user/me HTTP/1.1
Host: localhost:8080
Authorization: Bearer <access-token>
```

Gunakan `(issuer, sub)` sebagai identitas stabil. Pastikan `sub` UserInfo sama dengan `sub` ID token.

## 5. Refresh

```http
POST /oauth/token HTTP/1.1
Host: localhost:8080
Content-Type: application/x-www-form-urlencoded

grant_type=refresh_token&
client_id=<client-id>&
client_secret=<client-secret-confidential-client>&
refresh_token=<refresh-token-terbaru>
```

Setiap refresh mengembalikan access token dan refresh token baru. Ganti keduanya secara atomik; refresh response tidak menerbitkan ID token baru. Detail penanganan kegagalan ada di [rotation dan revocation](../security/tokens-revocation.md).

## Urutan yang tidak boleh ditukar

1. Validasi callback `state` dan `iss`.
2. Tukar code hanya dari server.
3. Validasi ID token, termasuk `nonce`.
4. Opsional: ambil UserInfo dan cocokkan `sub`.
5. Baru buat sesi lokal RP.

Jangan membuat sesi dari query callback, email yang belum diverifikasi, atau payload JWT yang hanya di-decode tanpa verifikasi signature.
