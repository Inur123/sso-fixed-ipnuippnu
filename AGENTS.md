# Aturan proyek

- Jangan commit, push, membuat release, atau deploy tanpa instruksi eksplisit pengguna.
- Tetap uji perubahan sebelum menyatakan selesai. Gunakan data sintetis dan lingkungan lokal terisolasi.
- Sesuai preferensi pengguna, file tes bersifat sementara: setelah pengujian, bersihkan file tes yang dibuat beserta fixture, laporan, dan hasil uji dari proyek. Jangan menyisakan file tes permanen tanpa persetujuan pengguna.
- Utamakan direktori sementara di luar proyek untuk hasil pengujian. Jangan menghapus kode aplikasi, konfigurasi, dependency, atau data pengguna saat membersihkan hasil uji.
- Laporkan pengujian yang benar-benar dijalankan; build/lint tidak sama dengan pengujian perilaku.
- Zona waktu aplikasi adalah `Asia/Jakarta`. Gunakan `backend/internal/apptime` untuk waktu aplikasi/jadwal dan `frontend/src/lib/date-time.ts` untuk tampilan. Jangan menambah 7 jam ke timestamp atau mengubah Unix timestamp protokol autentikasi. Metadata zona waktu perangkat tetap mencerminkan perangkat sebenarnya.
