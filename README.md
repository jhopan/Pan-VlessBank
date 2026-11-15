# Pan-VlessBank

Pan-VlessBank mengubah daftar `vless://` menjadi satu konfigurasi Xray. Setiap akun mendapat dua inbound, SOCKS5 dan HTTP, yang keduanya route ke outbound VLESS akun itu. Satu port akun berarti satu jalur exit yang tetap.

Runtime utama satu binary Go statis, tanpa dependensi runtime Go pihak ketiga.

> [!WARNING]
> `accounts.txt`, credential proxy, token client, dan credential Tunnel adalah rahasia. Jangan commit, kirim lewat chat, atau masukkan ke contoh dokumentasi.

## Arsitektur

```text
accounts.txt
  │
  ├─ vlessbank gen / Panel Regenerate
  ▼
config.json + config.tunnel.yml
  │
  ├─ Xray
  │   ├─ 127.0.0.1:1081 SOCKS5 ─┐ akun 1 ── VLESS upstream ── exit IP akun 1
  │   ├─ 127.0.0.1:2081 HTTP   ─┘
  │   ├─ 127.0.0.1:1082 SOCKS5 ─┐ akun 2 ── VLESS upstream ── exit IP akun 2
  │   └─ 127.0.0.1:2082 HTTP   ─┘
  │
  └─ cloudflared Tunnel
      socks2.example.com ── tcp://localhost:1082
      http2.example.com  ── tcp://localhost:2082
```

Port SOCKS adalah `base + i`. Port HTTP adalah `base_http + i`; jika `base_http=0`, nilainya `base + 1000 + i`. Index `i` dihitung dari akun valid dan deduplikasi, bukan nomor baris `accounts.txt`.

Cloudflare Tunnel hanya transport. Exit IP ditentukan outbound Xray pada port tujuan. Setiap hostname harus menuju port Xray berbeda.

## Komponen

| Komponen | Fungsi |
|---|---|
| `vlessbank` | Panel, generator config, checker IP, client tunnel CLI, dan utilitas unzip |
| Xray | Membuka inbound SOCKS5/HTTP dan konek ke VLESS upstream |
| cloudflared server | Connector Cloudflare Tunnel ke port Xray lokal |
| Pan-VlessBank Client | Bridge client loopback ke hostname Tunnel untuk Camoufox |
| `accounts.txt` | Link VLESS asli, satu link per baris |
| `webui.settings.json` | Port, bind, auth proxy, tunnel, dan path Xray |

## Install dan update

Release punya dua channel:

| Channel | Fungsi |
|---|---|
| `latest` | Build rolling terbaru, default installer |
| `vX.Y.Z` | Release tetap untuk install atau rollback |

Installer mengunduh binary sesuai OS/arsitektur, memverifikasi SHA-256 sebelum replace, lalu mempertahankan `accounts.txt`, config, settings, token, dan log.

### Linux dan macOS

```bash
curl -fsSL https://raw.githubusercontent.com/jhopan/Pan-VlessBank/master/install.sh | bash
```

### Windows

```bat
curl.exe -fsSL -o install.bat https://raw.githubusercontent.com/jhopan/Pan-VlessBank/master/install.bat
install.bat
```

Pin versi atau ubah folder install:

```bash
PAN_VLESSBANK_RELEASE=v0.6.0 PAN_VLESSBANK_DIR=$HOME/pan-vlessbank \
  curl -fsSL https://raw.githubusercontent.com/jhopan/Pan-VlessBank/master/install.sh | bash
```

Windows memakai environment variable yang sama sebelum menjalankan `install.bat`. Installer tidak memasang Xray atau cloudflared otomatis; sediakan kedua binary di folder server atau `PATH`.

## Menjalankan panel

```bash
# Default: panel http://127.0.0.1:9000
./pan-vlessbank

# Host Linux dengan state di folder khusus
./pan-vlessbank --port 9000 --listen 0.0.0.0 --dir /opt/pan-vlessbank
```

Panel publik status ada di `/`. Halaman `/admin` dan `/api/*` memerlukan login panel, lewat HTTP Basic atau cookie sesi `vb_session`.

> [!WARNING]
> Bind proxy ke `0.0.0.0` hanya boleh dengan `auth` proxy kuat. Jangan forward port proxy ke internet bila Tunnel adalah jalur remote yang dipilih.

## CLI

```bash
# Tulis config dari akun
./pan-vlessbank gen accounts.txt
./pan-vlessbank gen accounts.txt --base 1081 --base-http 2081
./pan-vlessbank gen accounts.txt --listen 0.0.0.0 --auth USER_PROXY:PASSWORD_PROXY

# Cek egress setiap akun
./pan-vlessbank check accounts.txt --base 1320 --workers 6

# Fan-out client manual, tanpa UI client
./pan-vlessbank tunnel-client --domain example.com --accounts 3 --auth USER_PROXY:PASSWORD_PROXY

./pan-vlessbank help
```

`gen` menolak `--listen 0.0.0.0` tanpa `--auth`. `check` exit `1` bila ada akun `MISMATCH` atau `FAIL`.

## Setup Cloudflare Tunnel

Di menu **Cloudflare** panel:

1. Pastikan `cloudflared` tersedia, download bila perlu.
2. Login Cloudflare, buat atau pilih proxy tunnel.
3. Pilih domain dan generate `config.tunnel.yml`.
4. Buat DNS wildcard proxied untuk domain Tunnel.
5. Start proxy tunnel.
6. Buka diagnostics Tunnel. Pastikan connector, service, dan ingress valid.

Contoh ingress dua akun:

```yaml
ingress:
  - hostname: socks1.example.com
    service: tcp://localhost:1081
  - hostname: http1.example.com
    service: tcp://localhost:2081
  - hostname: socks2.example.com
    service: tcp://localhost:1082
  - hostname: http2.example.com
    service: tcp://localhost:2082
```

Aturan wajib:

- DNS Tunnel harus proxied melalui Cloudflare.
- Gunakan hostname satu level seperti `socks1.example.com`.
- `socksN` dan `httpN` tidak boleh menunjuk port lokal sama.
- `/api/regen` menulis config baru, tetapi tidak restart Xray. Restart Xray setelah regenerate bila config yang sedang berjalan harus memakai perubahan.
- Tunnel credentials pada `config.tunnel.yml` tidak boleh ditimpa placeholder saat regenerate.

## Client remote melalui Cloudflare Tunnel

Hostname Tunnel bukan endpoint SOCKS atau HTTP langsung untuk browser. User remote perlu bridge lokal dengan `cloudflared access tcp`.

```text
Camoufox atau aplikasi proxy
  → listener lokal client
  → cloudflared access tcp
  → socks1.example.com atau http1.example.com
  → Cloudflare Tunnel
  → Xray port akun
```

Dua pilihan:

| Cara | Untuk siapa | Keterangan |
|---|---|---|
| Pan-VlessBank Client | User desktop | UI lokal mengimpor hostname dari token scope dan mengelola listener |
| `vlessbank tunnel-client` | Operator CLI | Membuka listener manual/fan-out tanpa UI client |

### Camoufox dan autentikasi proxy

Camoufox/Firefox tidak mendukung autentikasi SOCKS5 username/password. Pan-VlessBank Client menangani batas ini dengan relay lokal: client menyimpan **Auth Proxy** pada config lokal permission `0600`, lalu menyuntikkan auth hanya antara relay loopback dan bridge cloudflared.

Camoufox memakai SOCKS tanpa credential:

```python
proxy = {
    "server": "socks5://127.0.0.1:7001",
}
```

Untuk aplikasi lain yang mendukung SOCKS5 auth, credential dapat dikirim langsung. HTTP bridge tetap tersedia bila aplikasi memerlukannya.
> [!IMPORTANT]
> Client versi relay memerlukan **Auth Proxy** yang sama dengan auth Xray. Client lama tanpa field ini masih dapat membuka bridge HTTP, tetapi tidak dapat membuat SOCKS Camoufox tanpa auth.

Lihat README repository **Pan-VlessBank Client** untuk setup relay dan port internal.

## Token client read-only

Jangan memberi Basic auth admin panel ke client. Buat token client dari panel atau API admin:

```bash
curl -u ADMIN:PASSWORD -X POST https://panel.example.com/api/client-tokens \
  -d 'label=camoufox-laptop&hours=720&accounts=1,2'
```

- `accounts=1,2` membatasi token ke akun 1 dan 2.
- Scope kosong berarti semua akun.
- Nilai token hanya muncul sekali. Server hanya menyimpan hash token.
- Cabut token perangkat hilang atau tidak dipakai.

```bash
curl -u ADMIN:PASSWORD -X POST https://panel.example.com/api/client-tokens/revoke \
  -d 'id=TOKEN_ID'
```

Token client hanya boleh membaca endpoint berikut:

| Endpoint | Isi |
|---|---|
| `GET /api/client/proxies` | `idx`, nama, hostname SOCKS publik, hostname HTTP publik sesuai scope |
| `GET /api/cf/tunnel-diagnostics` | connector, service Tunnel, ingress, dan validasi Tunnel |

Token tidak dapat membaca VLESS URI, upstream, IP LAN, port Xray, credential proxy, token Cloudflare, daftar token, atau operasi admin.

## Verifikasi

### Verifikasi server lokal

```bash
# Setelah Xray berjalan, pakai credential proxy aktif
curl -sS --proxy-user USER_PROXY:PASSWORD_PROXY \
  --socks5-hostname 127.0.0.1:1081 \
  --max-time 25 'http://ip-api.com/json/?fields=query,countryCode'
```

### Verifikasi remote melalui client bridge

Setelah client membuka listener akun 2 pada `:7102`:

```bash
curl -sS --proxy-user USER_PROXY:PASSWORD_PROXY \
  --socks5-hostname 127.0.0.1:7102 \
  --max-time 25 'http://ip-api.com/json/?fields=query,countryCode'
```

Output harus memuat IP dan kode negara. Bandingkan setiap port bila akun harus memiliki exit IP berbeda.

### Development checks

```bash
cd go
gofmt -w ./cmd ./internal
go test ./... -count=1
go vet ./...
go build ./...
```

Test hermetic. Tidak membutuhkan jaringan atau akun VLESS nyata.

## Troubleshooting

| Gejala | Penyebab umum | Perbaikan |
|---|---|---|
| Semua akun memiliki exit IP sama | Hostname Tunnel menuju port Xray sama, atau upstream VLESS memang exit sama | Periksa ingress per hostname dan `vlessbank check` |
| `websocket: bad handshake` pada client | Xray mati atau connector gagal mencapai origin | Cek status Xray dahulu, lalu service proxy tunnel |
| Client diagnostics gagal | Token dicabut, service Tunnel mati, connector nol, atau ingress invalid | Buat token baru bila perlu, cek diagnostics panel |
| `No authentication method was acceptable` | Credential proxy belum diberikan ke aplikasi | Masukkan `USER_PROXY:PASSWORD_PROXY` pada konfigurasi SOCKS/HTTP Camoufox |
| `User was rejected by SOCKS5 server` | Credential proxy salah | Gunakan credential auth aktif di Pengaturan dan regenerate/restart Xray bila baru diubah |
| `no such host` atau TLS handshake gagal | DNS tidak proxied atau hostname lebih dari satu level | Gunakan CNAME proxied dan `socksN.domain.tld` |
| Listener lokal sudah dipakai | Sisa proses `cloudflared access tcp` | Stop client secara normal, lalu hentikan hanya proses listener yang memakai port tersebut |

## File state

| File | Isi | Rahasia |
|---|---|---:|
| `accounts.txt` | Link VLESS | Ya |
| `config.json` | Config Xray dan auth proxy | Ya |
| `config.tunnel.yml` | Config Tunnel dan path credential | Ya |
| `webui.settings.json` | Pengaturan panel dan proxy | Ya |
| `webui.login` | Hash login panel | Ya |
| `webui.sessions` | Sesi panel | Ya |
| `webui.client-tokens.json` | Hash dan metadata token client | Ya |
| `xray.log`, `cloudflared.log` | Log proses | Bisa berisi data sensitif |

Semua state berada di `--dir`, defaultnya folder binary.