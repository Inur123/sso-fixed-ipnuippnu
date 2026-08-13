---
title: Provisioning pengguna realtime
description: Menyinkronkan assignment SSO ke aplikasi tujuan secara aman tanpa menahan request portal.
---

# Provisioning pengguna realtime

Assignment SSO sudah cukup untuk menolak login yang tidak berhak. Jika aplikasi
tujuan juga perlu menampilkan pengguna **sebelum login pertama**, aktifkan
provisioning event untuk OAuth client tersebut.

```text
assignment disimpan
       ↓ transaksi yang sama
event masuk PostgreSQL outbox
       ↓ asinkron
webhook aplikasi menerima perubahan
```

Request portal tidak menunggu jaringan aplikasi tujuan. Setelah commit,
dispatcher langsung dibangunkan. Retry memakai exponential backoff, worker
dibatasi, dan `FOR UPDATE SKIP LOCKED` mencegah event yang sama diproses dua
instance secara paralel.

## Event

SSO mengirim CloudEvents structured JSON berikut:

- `user.assigned`: buat/aktifkan membership lokal;
- `user.updated`: perbarui salinan nama, email, dan avatar;
- `user.unassigned`: cabut membership serta sesi lokal, tetapi pertahankan
histori dan data bisnis.

`user.updated` hanya memperbarui membership yang sudah ada; event tersebut tidak
boleh membuat akses baru. Hanya `user.assigned` yang boleh membuat membership.

Identitas utama aplikasi adalah pasangan `(source, subject)` atau `(iss, sub)`.
Jangan menjadikan email sebagai primary key federasi karena email dapat berubah.
Role platform SSO, password, OTP, token, cookie, dan client secret tidak pernah
dikirim. Role bisnis tetap dikelola oleh aplikasi tujuan.

## Verifikasi penerima

Setiap request membawa `X-SSO-Event-ID`, `X-SSO-Timestamp`, dan
`X-SSO-Signature: v1=<hmac-sha256>`. Backend aplikasi harus:

1. membaca raw body;
2. menolak timestamp dengan selisih lebih dari lima menit;
3. menghitung `HMAC-SHA256(secret, timestamp + "." + rawBody)` dan membandingkan
   secara constant-time;
4. memvalidasi `source` serta `data.audience`;
5. menyimpan event ID pada unique index agar pemrosesan idempotent;
6. mengabaikan event yang lebih tua untuk subject yang sama;
7. commit perubahan lokal sebelum membalas HTTP 2xx.

Delivery bersifat **at least once**, sehingga idempotensi penerima wajib.

:::info Bukan SCIM
Webhook ini adalah kontrak provisioning internal, bukan implementasi SCIM.
OAuth/OIDC menangani login; event ini hanya menjaga membership lokal tetap
sinkron. Jika kelak diperlukan interoperabilitas provisioning umum, tambahkan
SCIM secara terpisah.
:::

## Konfigurasi backend SSO

```env
PROVISIONING_TARGETS_JSON={"<client-id>":{"url":"https://app.example.org/internal/sso/provisioning","secret":"<secret-random-min-32-karakter>"}}
PROVISIONING_MAX_ATTEMPTS=12
PROVISIONING_CONCURRENCY=4
```

Production mewajibkan HTTPS. Gunakan secret berbeda per aplikasi dan simpan di
secret manager. Client yang dikonfigurasi wajib memakai policy `assigned_only`.
Super admin dapat memantau metadata antrean melalui `GET /api/admin/provisioning`
dan retry event dead melalui `POST /api/admin/provisioning/:id/retry`; payload
identitas tidak dikembalikan oleh endpoint status.
