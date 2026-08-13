---
title: Rotation, reuse, dan revocation
description: Siklus hidup authorization code, access token, refresh token family, serta pencabutan akses.
---

# Rotation, reuse, dan revocation

## Masa berlaku

| Artefak | Masa berlaku | Karakteristik |
| --- | --- | --- |
| Authorization code | 5 menit | Sekali pakai, hash disimpan server, terikat client/redirect/PKCE |
| Access token | 1 jam | Bearer, diperlakukan opaque oleh RP |
| ID token | 1 jam | JWS RS256, bukti autentikasi pada waktu penerbitan |
| Refresh token | 30 hari | Opaque, dirotasi setiap penggunaan |

## Refresh token rotation

Setiap request refresh yang sukses:

1. mencabut refresh token lama;
2. menerbitkan access token baru;
3. menerbitkan refresh token baru dalam family grant yang sama.

RP harus memiliki **satu refresh worker per sesi/grant** dan mengganti access + refresh token dalam satu transaksi storage. Jangan biarkan dua request paralel memakai refresh token yang sama.

```text
load refresh_token_v1
  → POST /oauth/token
  → terima access_v2 + refresh_v2
  → transaksi DB: ganti v1 dengan v2
  → commit
```

Jika respons refresh timeout setelah server mungkin memprosesnya, jangan terus mengulang token lama. Retry tersebut dapat terlihat sebagai reuse. Akhiri grant lokal dan minta login ulang bila status hasil tidak dapat dipastikan.

## Reuse detection

Penggunaan refresh token yang sudah dicabut dianggap kemungkinan pencurian atau race. Issuer mencabut seluruh token family dan token endpoint mengembalikan `invalid_grant`. RP harus:

- menghapus semua token family itu dari storage;
- mengakhiri atau menurunkan sesi lokal;
- meminta autentikasi ulang;
- mencatat event tanpa menulis nilai token.

Jangan melakukan fallback ke refresh token yang lebih lama.

## Mencabut grant dari RP

Panggil dari backend menggunakan refresh token terbaru (atau access token):

```bash
curl -sS -X POST http://localhost:8080/oauth/revoke \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'token=<refresh-token-terbaru>' \
  --data-urlencode 'client_id=<client-id>' \
  --data-urlencode 'client_secret=<client-secret>'
```

Respons sukses adalah HTTP `200` tanpa body, termasuk jika token sudah tidak dikenal. Bila token dikenali, seluruh family grant dicabut. Setelah request—bahkan bila jaringan gagal—hapus token dan sesi lokal sesuai kebijakan logout aplikasi.

## Pencabutan dari portal

Portal pengguna menampilkan koneksi aplikasi berdasarkan token family. Pengguna dapat mencabut satu koneksi; pemilik client dapat menghapus aplikasinya, dan super admin dapat menonaktifkan atau menghapus akun. Operasi tersebut menyebabkan access/refresh token terkait ditolak pada penggunaan berikutnya.

RP harus memperlakukan HTTP `401 invalid_token` dari UserInfo sebagai sinyal untuk:

1. menghentikan penggunaan token;
2. menghapus grant lokal;
3. mengakhiri atau membatasi sesi aplikasi;
4. meminta login ulang bila pengguna melanjutkan.

## Grant SSO bukan sesi lokal RP

:::warning Penting
Mencabut grant di IPNU IPPNU ID **tidak otomatis menghapus cookie/session aplikasi RP**. Sebaliknya, logout dari RP juga tidak otomatis logout dari portal IdP. Saat ini belum ada `sid`, front-channel logout, atau back-channel logout.
:::

Karena itu RP bertanggung jawab atas sesi lokalnya sendiri. Jangan membuat sesi lokal tanpa batas hanya karena ID token pernah valid. Terapkan absolute lifetime, revalidasi berkala yang proporsional dengan risiko, dan hentikan sesi ketika API issuer menolak token.

ID token yang sudah diterbitkan adalah assertion bertanda tangan dan dapat lolos verifikasi offline sampai `exp`; pencabutan grant tidak menarik kembali file token tersebut. Jangan memakai ID token berulang kali sebagai session bearer.

## Perubahan akun

Issuer memeriksa akun aktif dan email terverifikasi saat login, pertukaran code, refresh, dan akses UserInfo. Penonaktifan akun mencabut sesi portal, authorization code, dan token grant. Meski demikian, sesi lokal RP tetap harus mematuhi aturan di atas.
