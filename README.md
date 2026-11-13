# Pan-VlessBank

Banyak akun `vless://` -> satu proses Xray -> 1 akun = 2 port: **SOCKS5 + HTTP proxy**.

```
accounts.txt (link vless, 1 baris 1 akun)
     |
     v
./vlessbank gen  ->  config.json (+ config.tunnel.yml jika mode tunnel)
     |
     v
xray run -c config.json
     |
     +--> 127.0.0.1:1081 (SOCKS5)  -> akun 1
     +--> 127.0.0.1:2081 (HTTP)    -> akun 1
     +--> 127.0.0.1:1082 (SOCKS5)  -> akun 2
     +--> 127.0.0.1:2082 (HTTP)    -> akun 2
     ...
```

Semua dijalankan dari satu binary Go `vlessbank` (stdlib only, tanpa dependensi): panel web, generator
config, uji IP akun, dan client tunnel. Binary yang sama dipakai di Windows (`vlessbank.exe`) dan di
STB/Linux (`GOOS=linux GOARCH=arm64`).

Port SOCKS = `base  + i` (default 1081). Port HTTP = `base+1000 + i` (default 2081).
Dua inbound route ke SAMA outbound -> IP keluar identik per akun. IP stabil per port -- cocok untuk sesi login/banking.

## Install binary release

Rilis memiliki dua channel:

- `vX.Y.Z`: release versioned, aset tidak berubah.
- `latest`: release rolling dari tag versioned terakhir.

Installer mengunduh binary `latest`, memverifikasi SHA-256, meminta port panel, lalu membuat konfigurasi LAN (`0.0.0.0`). Karena proxy LAN terbuka berbahaya, installer wajib meminta `user:pass` untuk proxy.

```bash
# Linux, macOS, Termux, atau Windows Git Bash
curl -fsSL https://raw.githubusercontent.com/jhopan/Pan-VlessBank/master/install.sh | bash

# Windows PowerShell
irm https://raw.githubusercontent.com/jhopan/Pan-VlessBank/master/install.ps1 | iex
```

Gunakan channel versi tetap dengan `VLESSBANK_RELEASE=v0.1.0`. Override noninteraktif:

```bash
VLESSBANK_WEBUI_PORT=9000 VLESSBANK_AUTH=user:pass \
  curl -fsSL https://raw.githubusercontent.com/jhopan/Pan-VlessBank/master/install.sh | bash
```

Installer memasang binary Linux arm64 pada Termux. Taruh `xray` dan `cloudflared` pada folder instalasi atau `PATH`. Buka firewall OS/router sendiri bila perangkat LAN lain perlu akses.

## Pakai (CLI)

```bash
# mode lokal (default): hanya 127.0.0.1
./vlessbank gen accounts.txt

# mode direct: VPS / mesin dengan IP publik (wajib auth!)
./vlessbank gen accounts.txt --listen 0.0.0.0 --auth user:pass

# port HTTP manual
./vlessbank gen accounts.txt --base-http 2081

# mode tunnel: mesin tanpa IP publik, lewat Cloudflare Tunnel
./vlessbank gen accounts.txt --tunnel socks.example.com --tunnel-id <UUID>
cloudflared tunnel run
```

`gen` mencetak ringkasan (`N akun -> config.json (listen ..., SOCKS 1081+, HTTP 2081+)`) plus tabel
`SOCKS :1081 | HTTP :2081 <- nama akun`. Flag lain: `-o/--out` (file keluaran), `--base`,
`--insecure` (khusus xray < 26). Kalau `--tunnel` dipakai, `config.tunnel.yml` ditulis di sebelah
file json keluaran. Lihat juga `./vlessbank help`.

Cara bikin binary: bagian **Binary Go** di bawah.

## Cek akun (penting untuk IP berbeda)

```bash
./vlessbank check                          # default accounts.txt, base 1081
./vlessbank check accounts.txt --base 1320 --workers 6   # base lain = hindari bentrok dengan xray panel
```

Perintah ini start xray sendiri, lalu per akun membandingkan **IP keluar** dengan **IP di path WS**
(`/<ip>-<port>`). Flag: `--base`, `--timeout` (default 12 detik), `--workers` (default 6), `--xray`
(path binary xray). Status:

| Status | Arti |
|---|---|
| `OK` | egress = IP di path, akun punya IP sendiri |
| `MISMATCH` | worker menerima path tapi keluar lewat IP lain (mis. Cloudflare) -> path diabaikan, banyak akun jadi 1 IP |
| `NOPATH` | path tidak berisi `ip-port`, IP tidak bisa diprediksi |
| `FAIL` | akun tidak bisa dipakai |

Exit code 1 kalau ada `MISMATCH`/`FAIL`. Contoh nyata: worker `cfvpn-*` -> semua `MISMATCH` (egress `104.28.x`),
worker `vpn-worker-prem-*` -> `OK` (egress = IP di path).

## Pakai (Panel web, recommended)

```bash
./vlessbank                                    # panel di http://127.0.0.1:9000/ (folder kerja = folder binary)
./vlessbank --port 9000 --listen 0.0.0.0 --dir /root/vlessbank   # STB: panel + proxy ke LAN
```

Flag `serve` (default): `--port`, `--listen`, `--xray` (nama/path binary xray), `--dir` (folder kerja,
default folder binary). Binary `xray` dan `cloudflared` auto-scan di folder project (lalu PATH);
path xray bisa diubah di Pengaturan (`xray_path`), cloudflared bisa diunduh dari menu Cloudflare.

Buka `http://127.0.0.1:9000/` -> login `admin/admin` (ganti di Pengaturan).

Menu: **Dashboard** (`/admin`, ringkasan: jumlah akun, status xray, tunnel, IP LAN, bind port) ·
**Akun** (`/admin/akun`, daftar lengkap: URL SOCKS5 + HTTP per akun, LAN & publik, tombol copy,
Test IP / Test Semua IP, tambah & hapus akun) · **Log** (`/admin/log`) · **Cloudflare** (`/admin/cf`) ·
**Pengaturan** (`/admin/settings`).

### Onlinekan WebUI dari local lab

Untuk mesin tanpa IP publik: setelah login Cloudflare dan memilih domain, buka step **Onlinekan WebUI** di menu Cloudflare.

1. Klik **Buat Tunnel WebUI**. Tunnel ini terpisah dari tunnel proxy akun.
2. Klik **Generate Config WebUI**. File `webui.tunnel.yml` meneruskan `panel.<domain>` ke `http://127.0.0.1:<port-panel>`.
3. Load dan pilih domain dengan API token, lalu klik **Buat DNS WebUI**. CNAME proxied `panel.<domain>` diarahkan ke tunnel WebUI.
4. Klik **Start WebUI**. Panel tetap bind LAN (`0.0.0.0`) untuk perangkat lokal, sedangkan tunnel memakai loopback sehingga perubahan IP LAN tidak mengganggu hostname publik.

Cloudflare Access opsional, tetapi login WebUI tetap wajib. Jangan membuka port panel di router, tunnel membuat koneksi keluar ke Cloudflare.


### Menu Cloudflare (via web, no terminal)

Alur lengkap:

1. Binary cloudflared -- auto-scan / download otomatis
2. Login CF -- klik tombol, link muncul, authorize di browser (fresh setiap kali)
3. Buat / Pilih tunnel -- tunnel ID tersimpan otomatis
4. Setup Domain -- paste CF API token -> Load Domain (list zone) -> pilih domain -> **Generate config.tunnel.yml** (credentials path auto) -> **Buat DNS Wildcard via API** (CNAME `*.socks.domain.com`)
5. DNS wildcard summary
6. Start/Stop tunnel + live log

### Client

| Mode | URI |
|---|---|
| Laptop sendiri | `socks5://127.0.0.1:1081` atau `http://127.0.0.1:2081` |
| LAN device | `socks5://IP_LAPTOP:1081` atau `http://IP_LAPTOP:2081` |
| Remote (tunnel) | lihat **"Tunnel: 1 akun = 1 port lokal"** di bawah |

#### Tunnel: 1 akun = 1 port lokal (WAJIB)

Tunnel cuma transport. IP keluar tetap ditentukan xray dari **port lokal mana** koneksi masuk
(`127.0.0.1:1081` = akun 1, `127.0.0.1:1082` = akun 2, dst). Kalau semua hostname `socksN`
diarahkan ke port lokal yang sama, mis. semuanya `cloudflared access ... --url 127.0.0.1:1081`,
semua request masuk akun 1 dan **IP keluar jadi sama semua**. Satu hostname = satu port lokal beda.

Pakai subcommand (spawn semua listener sekaligus, satu proses cloudflared per hostname):

```bash
./vlessbank tunnel-client --domain example.com --accounts 18          # socks 7001+, http 8001+
./vlessbank tunnel-client --domain example.com --accounts 18 --auth user:pass
./vlessbank tunnel-client --domain example.com --accounts 3 --dry     # cek tabel + perintahnya dulu
# hasil: socks5://127.0.0.1:7001 = akun 1, :7002 = akun 2, ...
```

Hostname yang dipakai: `socks1.example.com`, `http1.example.com`, `socks2.example.com`, dst.
Flag lain: `--sub` (prefix subdomain, mis. `--sub socks` -> `socks1.socks.example.com`),
`--socks-base` / `--http-base` (port lokal pertama, default 7001 / 8001), `--no-http` (hanya
listener SOCKS), `--cloudflared` (path/command cloudflared). Ctrl+C mematikan semua listener.


Atau manual, port lokal wajib beda per akun:

```bash
cloudflared access tcp --hostname socks1.socks.example.com --url 127.0.0.1:7001 &
cloudflared access tcp --hostname socks2.socks.example.com --url 127.0.0.1:7002 &
```

Verifikasi tiap akun benar-benar beda IP (jalankan di client):

```bash
for p in 7001 7002 7003; do
  curl -s --socks5-hostname 127.0.0.1:$p --max-time 15 "http://ip-api.com/json/?fields=query"; echo " <- $p"
done
```

DNS wildcard: CNAME `*.<domain>` -> `<tunnel-id>.cfargotunnel.com`, Proxy **ON** (orange).
Wajib ON -- record DNS-only membuat hostname tidak resolve (`no such host`) dan `cloudflared access` gagal.

Hostname **1 level** saja: `socks1.domain.com`. Pola 2 level (`socks1.socks.domain.com`) cuma jalan
kalau `socks.domain.com` didaftarkan sebagai zone sendiri; cert Universal SSL free plan hanya cover
`*.domain.com` (satu level), jadi 2 level -> `tls: handshake failure`.

## Binary Go (satu binary, tanpa Python)

```bash
cd go
go build -o vlessbank.exe ./cmd/vlessbank                       # Windows
GOOS=linux GOARCH=arm64 go build -o vlessbank ./cmd/vlessbank   # STB Armbian

./vlessbank.exe                                                   # serve: panel di :9000 (folder kerja = folder binary)
./vlessbank.exe --port 9000 --listen 0.0.0.0 --dir /root/vlessbank
./vlessbank.exe gen accounts.txt --base 1081                      # config.json (+ tunnel yml)
./vlessbank.exe check accounts.txt --base 1320                    # uji IP tiap akun (lihat bagian "Cek akun")
./vlessbank.exe tunnel-client --domain example.com --accounts 3   # client publik
./vlessbank.exe unzip file.zip folder                             # ekstrak zip (dipakai install.sh kalau unzip tidak ada)
./vlessbank.exe help
```

Uji: `cd go && go vet ./... && go test ./... -count=1` (test hermetic, tanpa network dan tanpa akun
vless asli). Butuh `xray` + `cloudflared` tetap seperti biasa (folder yang sama, atau `xray_path` di
Pengaturan). `install.sh` membangun binary ini kalau belum ada, mengunduh kedua binary, lalu
menjalankan `./vlessbank --port ... --listen ... --dir ...`.

## API tarik daftar proxy

Selain panel, ada endpoint JSON/teks untuk aplikasi lain yang mau "narik" daftar proxy
(auth sama seperti `/api/*`: Basic auth panel, atau cookie sesi `vb_session`):

```bash
curl -u admin:admin http://192.168.42.124:9000/api/proxies                  # JSON lengkap
curl -u admin:admin "http://192.168.42.124:9000/api/proxies?only=socks"     # SOCKS saja
curl -u admin:admin "http://192.168.42.124:9000/api/proxies?format=text"    # URI per baris
```

`format=text` keluarannya siap tempel:

```
socks5://vb:25i3FZ9JCuSM@192.168.42.124:1081#1
http://vb:25i3FZ9JCuSM@192.168.42.124:2081#1
```

Bentuk JSON: `{host, auth, listen, tunnel, count, proxies:[{idx, name, upstream, type,
socks:{host,port,url}, http:{host,port,url}, public:{socks,http,access_socks,access_http}}]}`.
`public` hanya muncul kalau tunnel diatur; `only=socks|http` menghapus sisi yang tidak dipakai.

Endpoint lain: `/api/status`, `/api/accounts`, `/api/ip?port=`, `/api/ipall`, `/api/add`,
`/api/delete`, `/api/xray`, `/api/regen`, `/api/cf/*`. Halaman `/` (status) sengaja publik, sisanya
butuh login.

## Catatan

- `accounts.txt` berisi UUID = kredensial. Gitignored -- jangan pernah push.
- **IP keluar per port ditentukan endpoint upstream di akun**, yaitu `address:port` di `config.json`
  (`settings.vnext[0]`). Seratus akun yang semuanya ke Worker/host yang sama = 1 IP, walaupun port,
  UUID, dan path-nya beda. Tunnel CF tidak mengubah ini -- tunnel cuma transport.
  Cek cepat: jalankan `./vlessbank gen`, `gen` mencetak `WARN` kalau endpoint upstream unik < jumlah akun.
- **Tambah akun**: link divalidasi minimal (skema `vless`, ada uuid, ada host, port wajar). Link
  rusak seperti `vless://abc` dihitung `dilewati` dan **tidak** ditulis ke `accounts.txt`, jadi tidak
  bisa lagi bikin xray gagal start. Hasilnya muncul di pesan halaman Akun:
  `N akun ditambah, D duplikat dibuang, M dilewati (bukan vless)`.
- Mode direct (0.0.0.0) WAJIB `--auth`. Proxy terbuka tanpa auth = open proxy, IP cepat ke-flag.
  `./vlessbank gen` menolak `--listen 0.0.0.0` tanpa `--auth`.
- **Bind LAN**: set `listen` = `0.0.0.0` di Pengaturan (tetap wajib `auth user:pass`). Client LAN pakai
  `socks5://user:pass@IP_STB:1081` atau `http://user:pass@IP_STB:2081`. Panelnya sendiri:
  `./vlessbank --listen 0.0.0.0`.
- **Regenerate config tidak lagi merusak tunnel**: `config.tunnel.yml` memakai path kredensial asli
  (`~/.cloudflared/<tunnel-id>.json`) atau mempertahankan path yang sudah tertulis di yml; dulu regen
  menimpanya dengan placeholder `<PATH>/<TUNNEL-UUID>.json` sampai tunnel gagal start ("doesn't exist
  or is not a file"). Placeholder cuma dipakai kalau kredensial belum ada sama sekali.
- xray dan cloudflared adalah proses anak panel: mematikan panel ikut mematikan keduanya, jadi start
  ulang xray/tunnel setelah panel di-restart.
- **Login panel**: sekali masuk, sesi disimpan cookie `vb_session` (HttpOnly, 30 hari, file
  `webui.sessions`) -- buka panel lagi langsung dashboard, tidak usah login ulang. Basic auth tetap
  didukung untuk script/bot. `/logout` menghapus sesi + cookie.
- **xray satu instance**: klik Start berkali-kali tidak menumpuk proses; kalau xray masih hidup
  (termasuk sisa panel yang di-restart) Start menolak dan menyebut PID-nya, dan Stop mematikan
  semua proses xray yang ketemu.
- **`--insecure` hanya untuk xray < 26.** Xray 26 menghapus `allowInsecure` (ganti
  `pinnedPeerCertSha256`); `gen` mendeteksi versi binary (`xray version`) dan otomatis melewati opsi
  itu, karena config yang masih memuatnya gagal di-load (`exit 23`).
- HTTPS client traffic pakai CONNECT -- HTTP inbound xray juga tunneling TLS-nya, sama bagus SOCKS.
- Xray: binary ~35MB, RAM ~30-60MB stabil walau banyak akun (18 akun da ketahua).
- cloudflared: 1 proses, RAM ~40-80MB, unmetered bandwidth, 1000 tunnel/akun limit free plan.
- xray diunduh `install.sh` (atau taruh `xray`/`xray.exe` di folder kerja, path bisa diatur di
  Pengaturan); cloudflared bisa diunduh otomatis dari menu Cloudflare.
- Tampilan panel (dark, lebar penuh) mengikuti token di `DESIGN.md`.

## Struktur

```
go/                     IMPLEMENTASI UTAMA -- satu binary Go, stdlib only (module vlessbank)
  cmd/vlessbank         serve (panel) | gen (config) | check (uji IP akun) | tunnel-client | unzip
  internal/...          config, settings, xray, netcheck, cf, web, diag, tunclient
  go.mod                module vlessbank (Go 1.26)
install.sh              installer satu-shot (build binary Go, unduh xray + cloudflared, tulis settings, start panel)
accounts.txt            akun asli (gitignored)
accounts.example.txt    contoh format
config.json             output xray (gitignored)
config.tunnel.yml       output cloudflared (gitignored)
webui.login             login hash panel (gitignored)
webui.sessions          token sesi login (gitignored, 30 hari)
webui.settings.json     settings panel (gitignored)
xray.exe / cloudflared.exe  binary lokal (gitignored)
```

`webui.py`, `gen.py`, dan `cf.py` (versi Python) sudah dihapus dari repo ini; kalau masih perlu,
riwayatnya ada di git.
