# DESIGN.md — arah desain vlessbank

Catatan jujur: arah ini disusun oleh agent (bukan dari brand guide pemilik produk). Selera AI
cenderung default dan monoton, jadi risiko hasil generik itu nyata. Kalau kamu punya referensi
visual sendiri (palet, font, screenshot yang kamu suka), arah ini harus diganti dengan punya kamu.

Dial: **ENERGY 1 / RHYTHM 2 / MOTION 1**

## Produk & pengguna

Panel operasional untuk satu operator (kamu) yang mengelola puluhan akun `vless://`, port SOCKS5/HTTP
per akun, dan satu Cloudflare Tunnel. Dipakai harian, sering dibuka di HP saat mengecek akun, dan
sering dibuka malam. Halaman yang paling sering dilihat: **Akun** (daftar port + URL) dan
**Dashboard** (status).

Konsekuensi: informasi > dekorasi. Angka (port, IP, PID) harus paling mudah dipindai.

## Palet (dark, dipakai sebagai default)

Alasan tema gelap: panel ini sering dipakai malam untuk mengecek akun dan log proses; log xray dan
terminal cloudflared juga gelap. Ini pilihan pemilik produk, bukan "karena terlihat tech". Tidak ada
toggle terang karena satu tema yang benar lebih baik daripada dua tema yang setengah jadi.

| Token | Nilai | Peran | Kontras terukur (di bg / surface) |
|---|---|---|---|
| `bg` | `#0f1115` | kanvas | nilai dasar, bukan teks |
| `surface` | `#161a21` | kartu, tabel | nilai dasar, bukan teks |
| `border` | `#2a3038` | garis pemisah, bukan teks | 1.42 (elemen non-teks) |
| `text` | `#e6e9ef` | teks utama | 15.54 / 14.34 |
| `muted` | `#9aa4b2` | label, hint | 7.49 / 6.92 |
| `accent` | `#3b82f6` | satu aksen: fokus, link, tombol utama | 5.14 / 4.74 |
| `ok` | `#22c55e` | status RUNNING | 8.29 / 7.66 |
| `warn` | `#f59e0b` | peringatan | 8.80 / 8.12 |
| `err` | `#f87171` | status MATI, aksi hapus | 6.83 / 6.31 |

Aturan warna: 2 warna inti (`bg`, `surface`) + 1 aksen (`accent`) + warna status yang hanya muncul
saat ada status. Tidak ada gradien, tidak ada glow, tidak ada glassmorphism: tidak ada hierarki yang
butuh itu di panel operasional.

## Tipografi

- Teks: stack sistem (`system-ui`, `Segoe UI`, `Roboto`), alasan: nol unduhan, konsisten lintas OS,
  sesuai batasan stdlib-only dan binari tunggal.
- Angka penting (port, IP, PID, URL): monospace sistem (`ui-monospace`, `Consolas`), alasan: angka
  sebaris jadi rata dan gampang dibandingkan antar baris, bukan untuk gaya terminal.
- Skala: judul kartu 15-16px tebal, isi 13-14px, label/hint 11.5-12px `muted`. Tidak ada huruf
  uppercase dengan letter-spacing lebar sebagai hiasan.

## Tata letak

- Satu focal point per layar: di **Dashboard** itu strip status (Akun / Xray / Tunnel); di **Akun**
  itu kartu akun pertama (kartu lain mengikuti pola yang sama, bukan bersaing).
- Whitespace sebagai pemisah bagian: jarak antar kartu 14-20px, di dalam kartu 12-14px, tidak seragam
  buta di semua tempat (RHYTHM 2: bagian statistik padat, kartu akun lebih lapang).
- Ritme antar halaman berbeda sesuai isi: Dashboard = ringkasan + kontrol, Akun = grid kartu,
  Log = blok `pre`, Cloudflare = langkah 1..6 berurutan. Bukan "judul tengah + grid kartu" berulang.
- Lebar konten: **penuh** (tanpa batas 1180px), padding 22/26px sebagai margin. Alasan: panel
  operasional dipakai di layar lebar, baris URL/port panjang tidak boleh terpotong, dan grid kartu
  akun jadi lebih banyak kolom dalam satu pandangan. Di layar sempit semua turun jadi satu kolom
  (breakpoint 900px) dan padding 12px.
- Motif identitas (satu gerakan yang diulang): **label kecil di atas nilai monospace**. Contoh:
  `PORT SOCKS` di atas `1081`, `IP LAN` di atas `192.168.42.124`, tag `LAN` / `PUBLIK` di dalam baris
  URL. Ini satu-satunya "hiasan" yang dipakai, dan gunanya memisahkan label dari nilai.

## Komponen

- Radius: 8px untuk kontrol kecil (chip, tombol, input), 12px untuk kartu. Tidak ada bentuk pil.
- Kartu akun: kepala (nomor + nama + upstream), baris URL (chip: tag sumber + nilai + tombol copy),
  kaki (IP keluar + Test IP + Hapus). Hierarki dari ukuran dan warna, bukan dari ikon besar.
- Status: teks + warna (`RUNNING`, `MATI`, `BELUM`), tanpa ikon dekoratif dan tanpa glow.
- Tombol: satu tombol utama per kartu (`Tambah`, `Start`), sisanya `sec` (outline) atau `del` (merah).
  Tidak ada tanda panah di tombol; panah hanya di link navigasi "Kelola Akun" karena memang berpindah
  halaman.
- Ikon menu (`▤ ≡ ☰ ☁ ⚙`): dipertahankan sebagai penanda posisi di sidebar, bukan hiasan konten.
- Motion: transisi hover/focus 120-160ms saja. Tidak ada animasi masuk/scroll (MOTION 1).

## Keadaan yang wajib ada

- Kosong: "Belum ada akun, tempel link vless:// di bawah", dengan form tambah tetap tampil.
- Memuat: teks status pada aksi yang butuh waktu (Test IP: "mengambil...", tunnel: "menunggu...").
- Error: pesan di kotak merah (`.flash.err`) berisi teks error asli dari server atau `HTTP <kode>`.
- Status proses: RUNNING / MATI selalu terlihat tanpa perlu klik.

## Aksesibilitas & aturan teks

- Kontras minimum WCAG AA (4.5:1 teks normal, 3:1 teks besar) untuk semua teks, termasuk `muted`.
- Fokus keyboard selalu terlihat (`outline` aksen), semua aksi bisa dicapai Tab, dialog tutup Esc.
- Target sentuh minimum 44px di layar HP.
- Tidak memakai em dash (`—`) di teks UI (aturan R-02). Kalau perlu jeda, pakai koma, titik dua, atau
  tanda kurung.

## Yang sengaja tidak dipakai

Gradien, glow, background grid/dot, glassmorphism, ikon sparkle/robot, badge "AI Powered"/"Beta",
angka statistik tanpa sumber, kartu fitur seragam tanpa hierarki, dan klon visual produk lain
(Linear/Vercel/Stripe). Semua itu tidak melayani hierarki di panel ini.
