# Provisioning aplikasi

Modul ini menyinkronkan assignment SSO ke backend aplikasi tujuan tanpa
menahan request dashboard. Assignment dan event outbox disimpan dalam transaksi
PostgreSQL yang sama. Dispatcher mengirim event sesudah transaksi berhasil.

## Konfigurasi

Target hanya dikonfigurasi di environment backend SSO:

```env
PROVISIONING_TARGETS_JSON={"<oauth-client-id>":{"url":"https://laci.example.org/internal/sso/provisioning","secret":"<random-minimum-32-characters>"}}
PROVISIONING_MAX_ATTEMPTS=12
PROVISIONING_CONCURRENCY=4
```

Gunakan secret berbeda untuk setiap aplikasi. Production hanya menerima URL
HTTPS. Client yang memakai provisioning wajib memakai policy `assigned_only`.

## Kontrak request

Request menggunakan CloudEvents structured content mode:

```http
POST /internal/sso/provisioning
Content-Type: application/cloudevents+json
X-SSO-Event-ID: <uuid>
X-SSO-Timestamp: <unix-seconds>
X-SSO-Signature: v1=<hex-hmac-sha256>
```

Contoh body:

```json
{
  "specversion": "1.0",
  "id": "event-uuid",
  "type": "user.assigned",
  "source": "https://sso.example.org",
  "subject": "user-uuid",
  "time": "2026-08-13T07:00:00Z",
  "datacontenttype": "application/json",
  "data": {
    "audience": "oauth-client-uuid",
    "user": {
      "sub": "user-uuid",
      "name": "Nama pengguna",
      "email": "user@example.org",
      "email_verified": true,
      "picture": "https://example.org/avatar.png"
    }
  }
}
```

`type` bernilai `user.assigned`, `user.updated`, atau `user.unassigned`. Event
`user.updated` dikirim ketika profil pengguna berubah. Role platform SSO,
password, OTP, session cookie, access token, dan client secret tidak dikirim.

## Verifikasi di aplikasi tujuan

1. Baca raw request body tanpa mengubah byte-nya.
2. Tolak timestamp yang berbeda lebih dari lima menit dari waktu server.
3. Hitung `HMAC-SHA256(secret, timestamp + "." + rawBody)`.
4. Bandingkan signature secara constant-time.
5. Pastikan `source` sama dengan issuer SSO dan `data.audience` sama dengan
   Client ID aplikasi.
6. Simpan `id` pada unique index. Event dengan ID yang sama harus menghasilkan
   respons sukses tanpa diproses ulang.
7. Simpan `time` terakhir per pasangan `source + audience + subject`. Abaikan
   event dengan waktu yang lebih lama daripada event terakhir. Ini mencegah
   retry event lama mengembalikan akses yang sudah dicabut.
8. Terapkan event dalam transaksi lokal. Balas HTTP 2xx hanya setelah commit.

Penerima harus memperlakukan event sebagai *at-least-once delivery*. Event
`user.unassigned` mencabut membership dan sesi lokal, tetapi tidak menghapus
histori/data bisnis pengguna.

## Operasional

- `GET /api/admin/provisioning` menampilkan jumlah/status event tanpa payload.
- `POST /api/admin/provisioning/:id/retry` menjadwalkan ulang event `dead`.
- Retry memakai exponential backoff dan maksimum attempt dari environment.
- Transaksi sukses membangunkan dispatcher secara langsung; tidak ada polling
  database dua detik. Timer dinamis dipakai untuk retry dan recovery 30 detik
  menangani crash atau event dari instance lain.
- Pengiriman dibatasi `PROVISIONING_CONCURRENCY` per instance dan memakai
  koneksi HTTP keep-alive agar tetap ringan.
- Event sukses non-bootstrap dibersihkan setelah 30 hari.
- Payload event sukses segera dikosongkan; metadata delivery tetap tersedia
  tanpa menyimpan salinan profil lebih lama dari yang diperlukan.
- Saat target pertama kali dikonfigurasi, startup membuat event snapshot untuk
  seluruh assignment aktif yang sudah ada secara idempotent.
