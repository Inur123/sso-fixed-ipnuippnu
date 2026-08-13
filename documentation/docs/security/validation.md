---
title: Validasi callback dan ID token
description: Checklist normatif untuk mencegah callback injection, token substitution, dan JWT confusion.
---

# Validasi callback dan ID token

Keamanan SSO tidak selesai ketika token endpoint mengembalikan HTTP 200. RP wajib memvalidasi transaksi, ID token, dan keterikatan UserInfo sebelum membuat sesi lokal.

## Validasi authorization response

Lakukan pemeriksaan dalam urutan berikut:

1. Ambil transaksi login dari sesi server menggunakan ID cookie acak.
2. Hapus transaksi secara atomik agar hanya dapat digunakan sekali.
3. Bandingkan `state` callback dengan state transaksi menggunakan comparison yang aman.
4. Bandingkan parameter authorization response `iss` secara persis dengan issuer terkonfigurasi.
5. Jika terdapat `error`, termasuk `access_denied`, hentikan alur; jangan meneruskan `error_description` mentah ke HTML.
6. Pastikan `code` ada, lalu tukar dari backend dengan verifier dan redirect URI milik transaksi itu.

Jangan memilih issuer, client ID, redirect URI, atau endpoint token dari input callback. Jangan log URL callback utuh karena memuat authorization code.

## Validasi ID token

Gunakan library OIDC/JWT yang terawat dan enforce semua kondisi ini:

| Pemeriksaan | Ketentuan |
| --- | --- |
| Format JWS | Compact JWS bertanda tangan; bukan token plaintext/unsecured |
| `alg` | Tepat `RS256`; jangan menerima `none`, HMAC, atau algoritma lain |
| `kid` | Memilih public key yang cocok dari `jwks_uri` issuer |
| Signature | Valid dengan public RSA key tersebut |
| `iss` | Exact match dengan `SSO_ISSUER` |
| `aud` | Memuat tepat client ID RP yang diharapkan |
| `exp` | Belum kedaluwarsa, dengan clock skew kecil dan eksplisit |
| `iat` | Ada, numerik, dan tidak tidak masuk akal di masa depan |
| `sub` | Ada, string nonkosong, diperlakukan opaque |
| `nonce` | Exact match dengan nonce transaksi login satu-kali |

Issuer saat ini menerbitkan satu audience client. Jika kelak ID token memiliki banyak audience, enforce `azp` sesuai OpenID Connect Core sebelum menerima token.

:::danger Decode bukan verifikasi
Membaca payload JWT dengan Base64 atau fungsi `decode()` tidak membuktikan siapa penerbitnya. Sesi hanya boleh dibuat setelah signature dan seluruh claim di atas valid.
:::

## Discovery dan SSRF

- Konfigurasi issuer berasal dari environment/operator, bukan parameter pengguna.
- Fetch discovery hanya dari issuer yang sudah dipercaya.
- Pastikan field `issuer` metadata sama persis dengan issuer konfigurasi.
- Ikuti `jwks_uri` hanya dari metadata yang sudah tervalidasi.
- Terapkan timeout, batas ukuran respons, TLS verification, dan cache.
- Bila `kid` tidak ditemukan, refresh JWKS satu kali; jangan melakukan fetch tanpa batas untuk setiap token invalid.

## Validasi UserInfo

Setelah ID token valid:

1. kirim access token ke `userinfo_endpoint` lewat header Bearer;
2. require HTTP sukses dan JSON dengan `sub` nonkosong;
3. bandingkan `sub` UserInfo dengan `sub` ID token;
4. hanya gunakan field yang scope-nya diberikan;
5. jika memakai email untuk account linking, require `email_verified=true` dan kebijakan anti-takeover lokal.

Jangan decode atau memvalidasi access token menggunakan JWKS ID token. RP memperlakukannya opaque.

## State, nonce, dan PKCE bukan pengganti satu sama lain

- `state` mengikat browser/callback ke transaksi RP.
- `nonce` mengikat ID token ke permintaan autentikasi.
- PKCE mengikat authorization code ke RP yang memiliki verifier.
- `client_secret` mengautentikasi confidential client; public client tidak mempunyai secret yang dapat dipercaya.

Semua mekanisme tersebut dipakai bersama. Entropi setiap nilai minimal 256 bit dan transaksi berumur pendek, satu-kali, serta dihapus pada sukses maupun gagal.

## Membentuk sesi lokal

Setelah validasi:

- upsert berdasarkan `(issuer, sub)`;
- pastikan login memang berhasil dan terapkan role milik aplikasi sendiri;
- rotasi ID sesi setelah login untuk mencegah session fixation;
- cookie sesi: `HttpOnly`, `Secure`, path sempit, `SameSite=Lax` atau kebijakan yang sesuai;
- jangan simpan access/refresh/ID token dalam cookie browser yang dapat dibaca JavaScript;
- batasi absolute lifetime dan idle timeout sesi lokal.
