---
title: Membuat aplikasi OAuth
description: Daftarkan aplikasi dan Redirect URI dari portal IPNU IPPNU ID.
---

# Membuat aplikasi OAuth

Setiap akun anggota yang aktif dan sudah memverifikasi email dapat membuat aplikasi. Akun hanya dapat melihat dan mengelola aplikasi miliknya sendiri.

## Langkah di dashboard

1. Masuk ke portal IPNU IPPNU ID.
2. Buka **Aplikasi OAuth**.
3. Pilih **Tambah aplikasi**.
4. Isi nama, deskripsi opsional, dan minimal satu **Redirect URI**.
5. Pilih kebijakan akses aplikasi.
6. Untuk kebijakan terbatas, buka **Kelola akses pengguna**, lalu masukkan UUID pengguna atau email secara persis.
7. Simpan aplikasi.
8. Simpan credential sesuai jenis client.

Scope `openid profile email` diizinkan otomatis. Aplikasi tetap harus meminta scope minimum pada authorization request.

## Jenis client yang tersedia

Versi ini mendukung **confidential client** untuk backend web, BFF, Laravel, atau Go server. Token endpoint memerlukan `client_id`, `client_secret`, dan PKCE S256. SPA murni, mobile, dan desktop public client belum didukung; jangan menanam client secret ke JavaScript browser atau aplikasi mobile.

## Pilih kebijakan akses

- **Semua pengguna aktif**: seluruh akun aktif dan terverifikasi boleh meminta akses.
- **Pengguna yang ditugaskan**: hanya assignment pengguna langsung yang aktif yang boleh meminta akses.

Gunakan pilihan kedua sebagai default untuk aplikasi internal. Assignment hanya mengatur boleh masuk; role dan permission tetap diperiksa oleh API aplikasi tujuan. Lihat [akses per aplikasi](../protocol/application-access.md).

## Redirect URI

Redirect URI adalah endpoint callback di aplikasi Anda. Nilainya harus sama persis pada dashboard, authorization request, dan token exchange.

```text title="Development"
http://localhost:3002/auth/callback
```

```text title="Production"
https://akun.example.org/auth/callback
```

- Production wajib menggunakan HTTPS.
- HTTP hanya diizinkan untuk `localhost` atau `127.0.0.1`.
- URL tidak boleh memakai fragment `#` atau wildcard.
- Tambahkan beberapa URI melalui tombol **Tambah URI**, bukan dalam satu input.

## Client secret confidential client

Gunakan tombol mata untuk menampilkan secret sementara. Aksi melihat tidak membuat toast sukses dan tidak dicatat sebagai audit log. Nilai secret tidak boleh masuk source code, browser bundle, screenshot, atau repository.

Jika secret bocor, pilih **Regenerate secret**. Secret lama langsung tidak berlaku; token dan authorization code lama juga dicabut. Aksi regenerate tetap masuk audit log karena mengubah kredensial.

## Edit atau hapus aplikasi

- Tombol edit memperbarui nama, deskripsi, dan Redirect URI. Authorization code yang belum dipakai dihapus saat konfigurasi berubah.
- Tombol hapus menghapus aplikasi dan mencabut seluruh akses terkait.

:::warning Aplikasi harus memiliki backend
Confidential client harus memiliki backend atau BFF yang dapat menjaga secret. Jangan menaruh client secret di SPA, aplikasi mobile, `NEXT_PUBLIC_*`, atau JavaScript browser. Gunakan tipe public client untuk aplikasi yang tidak mampu menjaga secret.
:::
