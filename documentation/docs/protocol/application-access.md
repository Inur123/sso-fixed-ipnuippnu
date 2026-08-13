---
title: Akses per aplikasi
description: Memisahkan login SSO, izin masuk aplikasi, dan otorisasi bisnis aplikasi.
---

# Akses per aplikasi

Berhasil masuk ke IPNU IPPNU ID belum berarti pengguna boleh memakai semua aplikasi. OIDC membuktikan **siapa pengguna**, sedangkan kebijakan aplikasi menentukan **apakah pengguna boleh masuk** dan **apa yang boleh dilakukan**.

Gunakan tiga lapisan yang terpisah:

| Lapisan | Pertanyaan | Contoh |
| --- | --- | --- |
| Identitas SSO | Siapa pengguna ini? | `iss` + `sub`, email terverifikasi |
| Assignment aplikasi | Boleh masuk ke aplikasi mana? | Semua akun aktif atau hanya pengguna yang ditugaskan |
| Otorisasi aplikasi | Apa yang boleh dilakukan di aplikasi itu? | Dikelola pada database dan API aplikasi tujuan |

Role platform `super_admin` dan `anggota` hanya berlaku untuk portal IPNU IPPNU ID. Role tersebut tidak otomatis menjadi kewenangan di aplikasi lain.

## Kebijakan akses client

Setiap OAuth client memiliki kebijakan akses. Pilihan yang aman untuk aplikasi internal adalah **hanya pengguna yang ditugaskan**.

```text
all_active_users  → semua akun aktif dan terverifikasi boleh meminta akses
assigned_only     → hanya assignment pengguna aktif yang boleh meminta akses
```

Status aplikasi dipisahkan dari kebijakan akses:

```text
active    → client dapat memulai alur OAuth
suspended → seluruh authorization baru ditolak
```

Memisahkan status dan kebijakan mencegah nilai seperti `disabled` tercampur dengan aturan keanggotaan.

## Assignment pengguna

Assignment langsung cocok untuk pengecualian atau tim kecil:

```text
Aplikasi Administrasi
├── Ahmad
├── Siti
└── Budi
```

Tambahkan assignment dengan memasukkan UUID pengguna atau email secara persis. Hanya akun aktif dengan email terverifikasi yang dapat ditambahkan. Versi saat ini mendukung assignment pengguna langsung; sinkronisasi grup/SCIM merupakan pengembangan berikutnya, bukan fitur yang diklaim tersedia sekarang.

## Role dan permission aplikasi

Assignment di SSO hanya menjawab “boleh masuk”. Setelah login berhasil, aplikasi tujuan membuat atau menghubungkan akun lokal memakai `(iss, sub)`, lalu menentukan role dan permission dari database miliknya sendiri.

```text
Aplikasi Administrasi: operator
Aplikasi Keuangan: viewer
```

Role `operator` pada satu aplikasi tidak memberikan hak apa pun pada aplikasi lain. SSO tidak mengirim role platform maupun role bisnis. Aplikasi tetap wajib memeriksa role pada setiap endpoint sensitif; menyembunyikan tombol di frontend bukan kontrol akses.

## Pemeriksaan saat authorization

SSO memeriksa akses setelah sesi pengguna tervalidasi dan sebelum authorization code diterbitkan:

```text
akun aktif + email terverifikasi
              ↓
client aktif + Redirect URI cocok
              ↓
kebijakan all_active_users atau assignment aktif
              ↓
consent → authorization code → token
```

Jika pengguna tidak memiliki akses dan Redirect URI telah dipercaya, SSO mengembalikan error OAuth standar:

```text
https://app.example.org/auth/callback
  ?error=access_denied
  &error_description=Pengguna%20tidak%20memiliki%20akses
  &state=<state-asli>
  &iss=<issuer>
```

Aplikasi harus memvalidasi `state` dan `iss`, menghentikan login, serta menampilkan pesan lokal yang aman. Jangan membuat akun atau sesi lokal ketika menerima `access_denied`.

:::important Scope bukan assignment
Scope seperti `openid`, `profile`, dan `email` menjelaskan data yang diminta client. Scope tidak menentukan pengguna mana yang boleh masuk. Consent juga bukan assignment: pengguna tidak dapat memberi dirinya sendiri akses hanya dengan menyetujui consent.
:::

## Data yang dipercaya aplikasi

Gunakan `(iss, sub)` sebagai identitas stabil. Jangan memakai email sebagai primary key federasi.

- **ID token** membuktikan peristiwa autentikasi dan audience client.
- **UserInfo** memberikan profil yang diizinkan scope.
- **Assignment di SSO** menjadi gerbang penerbitan authorization code.
- **Policy aplikasi** tetap melindungi endpoint dan data bisnis aplikasi.

ID token bukan daftar izin yang harus dipercaya selamanya. Jika assignment atau akun dicabut, aplikasi perlu mengakhiri sesi lokal berdasarkan kebijakan risikonya; lihat [rotation dan revocation](../security/tokens-revocation.md).

## Pola default yang disarankan

Untuk client baru:

1. gunakan `assigned_only` untuk aplikasi internal;
2. gunakan `all_active_users` hanya untuk layanan yang memang terbuka bagi seluruh anggota;
3. kelola role dan permission di aplikasi tujuan;
4. audit perubahan assignment, tetapi jangan mencatat token atau secret;
5. cabut grant aktif ketika assignment dicabut.

Model ini mengikuti pembagian tanggung jawab OAuth/OIDC: authorization server menerapkan kebijakan sebelum menerbitkan credential, sementara aplikasi tetap menerapkan authorization pada resource miliknya.
