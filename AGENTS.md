# Repository Guidelines

## Project Overview

`vlessbank` (upstream name `vless-bank`) turns a list of `vless://` links into one Xray config where **each account gets two local proxy inbounds, SOCKS5 and HTTP, both routed to the same outbound**, so every account has a stable exit IP.

The whole runtime is one static Go binary (module `vlessbank`, Go 1.26, standard library only: no third-party dependency, no manifest beyond `go.mod`). It has five subcommands: `serve` (default; panel plus JSON API), `gen` (writes `config.json` and optionally `config.tunnel.yml`), `check` (per-account egress IP against the IP embedded in the WS path), `tunnel-client` (client-side `cloudflared access tcp` fan-out), and `unzip` (used by `install.sh` when the `unzip` utility is missing). The same binary builds for Windows (`vlessbank.exe`) and for STB/Linux (`GOOS=linux GOARCH=arm64`).

Primary users are operators on Linux VPS or Windows hosts who need per-account proxy ports, optionally exposed remotely through a Cloudflare Tunnel that the web panel manages end to end.

## Architecture & Data Flow

```
accounts.txt ──► vlessbank gen ──► config.json (+ config.tunnel.yml) ──► xray run -c config.json
                   ▲                                                        │
                   │ same code path (config.BuildConfig)                    ├─► 127.0.0.1:1081 SOCKS5 ─┐ acc 1
              panel /api/regen ──────────────────────────────────────────── ├─► 127.0.0.1:2081 HTTP   ─┘
                   │                                                        ├─► 127.0.0.1:1082 SOCKS5 ─┐ acc 2
                   └──► internal/cf ──► cloudflared tunnel run --config config.tunnel.yml
```

- **Port math is the core invariant**: SOCKS = `base + i`, HTTP = `base_http + i` (`base_http == 0` means `base + 1000`). Index `i` is the position among *valid, deduped* accounts, not the line number in `accounts.txt`.
- `internal/config` is the single source of config truth: `ParseFile`, `DedupFile`, `ValidLink`, `BuildConfig` (one outbound plus both inbounds plus the routing rule binding both inbound tags to that outbound), `WriteJSON`, `TunnelYML`, `XrayMajor`. It is pure and cheap to unit test.
- Nothing else builds a config document: `app.regen()` (CLI and the panel's `/api/regen`) and `app.binds()` (the account to port table used by the UI and the tunnel yml generator) both call `config.BuildConfig`, so the ports shown always match the JSON written.
- `internal/xray.Manager` is the single owner of the xray child process: start/stop/restart are serialized so repeated Start does not stack processes, and a process scan lets a restarted panel still see and kill strays.
- `internal/cf` owns the whole cloudflared lifecycle: binary download, `tunnel login` (always fresh), tunnel create/list/use/delete, `tunnel run --config config.tunnel.yml`, plus Cloudflare REST calls for zones and the wildcard CNAME, and the tunnel yml generator.
- `internal/web` is the panel: one `http.Server` with a single `ServeHTTP` switch over routes, HTML from Go string templates in `pages_*.go` and the dark tokens from `DESIGN.md` in `css.go`. It owns no child process itself: it calls into `internal/xray`, `internal/cf`, `internal/config`, and `internal/settings`, and receives the IP-test callbacks (which use `internal/netcheck`) plus the regen/add/delete hooks through `web.Deps`, wired in `cmd/vlessbank`.
- State is on-disk, not in a database: `accounts.txt`, `config.json`, `config.tunnel.yml`, `webui.settings.json`, `webui.login`, `webui.sessions`, `xray.log`, `cloudflared.log`, all under the working directory selected by `--dir`. In-memory state is just the xray process handle, the cloudflared process globals, and download progress.

## Key Directories

| Path | Purpose |
|---|---|
| `go/cmd/vlessbank/main.go` | CLI entry: subcommand dispatch, `serve`/`gen`/`check`/`tunnel-client`/`unzip`, flag parsing, `regen`, `addAccounts`, `deleteAccount`, `tunnelCreds` |
| `go/internal/config` | vless parsing, dedup, port math, `config.json` and tunnel yml emit, xray major version probe |
| `go/internal/settings` | `webui.settings.json`, `webui.login` (sha256 + salt), `webui.sessions` (30-day cookie tokens) |
| `go/internal/xray` | xray child process manager (start/stop/restart, running/PID detection, stray cleanup, log tail) |
| `go/internal/netcheck` | SOCKS5 dialing plus IP probe helpers used by `/api/ip`, IP tests, and `check` |
| `go/internal/cf` | cloudflared manager plus Cloudflare API client (`zones`, wildcard CNAME) and tunnel yml generator |
| `go/internal/web` | panel pages, JSON API, auth, CSS tokens |
| `go/internal/diag` | `check` probes: egress IP per account versus `/<ip>-<port>` in the WS path |
| `go/internal/tunclient` | client-side `cloudflared access tcp` fan-out, one listener per hostname |
| `install.sh` | one-shot installer (Linux / Windows git-bash); refuses macOS. Builds the Go binary when it is absent, downloads xray + cloudflared, writes settings, starts `./vlessbank --port ... --listen ... --dir ...` |
| `.env.example` | Installer env overrides (`VLESSBANK_*`); read only by `install.sh` |
| `DESIGN.md` | Panel design direction (dark tokens, layout, states, no em dash in UI text); source for `css.go` |
| `README.md` | Authoritative architecture/usage doc, in Indonesian |
| `accounts.example.txt` | Sample `vless://` lines and comments format |
| Runtime artifacts (gitignored) | `config.json`, `config.tunnel.yml`, `webui.settings.json`, `webui.login`, `webui.sessions`, `*.log`, `accounts.txt`, `vlessbank(.exe)`, `xray(.exe)`, `cloudflared(.exe)` |

## Development Commands

```bash
cd go
go build ./... && go vet ./... && go test ./... -count=1        # build + vet + hermetic tests
go build -o vlessbank.exe ./cmd/vlessbank                       # Windows
GOOS=linux GOARCH=arm64 go build -o vlessbank ./cmd/vlessbank   # STB Armbian

./vlessbank.exe                                                 # serve: panel on :9000 (folder = binary dir)
./vlessbank.exe --port 9000 --listen 0.0.0.0 --dir /root/vlessbank
./vlessbank.exe gen accounts.txt --base 1081                    # config.json (+ tunnel yml)
./vlessbank.exe check accounts.txt --base 1320                  # egress IP per akun vs IP di path WS
./vlessbank.exe tunnel-client --domain example.com --accounts 3 --dry
./vlessbank.exe unzip file.zip folder
./vlessbank.exe help
```

Flag reference (confirmed against each `-h`):
`serve`: `--port`, `--listen`, `--xray`, `--dir`.
`gen`: `-o`/`--out`, `--base`, `--base-http`, `--listen`, `--auth`, `--tunnel`, `--tunnel-id`, `--insecure`.
`check`: `--base`, `--timeout`, `--workers`, `--xray`.
`tunnel-client`: `--domain`, `--accounts`, `--sub`, `--socks-base`, `--http-base`, `--no-http`, `--auth`, `--cloudflared`, `--dry`.

`gen` refuses `--listen 0.0.0.0` without `--auth` (open proxy) and rejects an `--auth` value without a colon. `check` exits 1 when any account is `MISMATCH` or `FAIL`.

Full install on a fresh host (clones, builds or fetches the binary, downloads xray + cloudflared, writes settings, starts the panel):

```bash
curl -fsSL https://raw.githubusercontent.com/jhopan/vless-bank/master/install.sh | bash
```

Installer env: `VLESSBANK_DIR`, `VLESSBANK_REPO`, `VLESSBANK_WEBUI_PORT`, `VLESSBANK_BASE_PORT`, `VLESSBANK_BASE_HTTP`, `VLESSBANK_LISTEN`, `VLESSBANK_AUTH`, `VLESSBANK_FORCE`, `VLESSBANK_SKIP_BINS`, `VLESSBANK_START` (see `.env.example`); an `.env` in the app dir is sourced.

There is **no formatter, linter, or CI beyond `gofmt` and `go vet`**. Do not invent tooling; if you add any, keep it optional and keep the stdlib-only property intact.

## Code Conventions & Common Patterns

- **Go, standard library only.** Adding a third-party module breaks the project contract.
- **Layering**: `cmd/vlessbank` wires everything and owns the `app` struct; `internal/*` never imports `cmd`. `internal/web` depends on `config`, `settings`, `xray`, `cf`, and gets everything else (IP tests, regen, add/delete) as callbacks in `web.Deps`; `internal/diag` builds on `config` and `netcheck`; `internal/tunclient` and `internal/netcheck` have no internal dependencies.
- **Error handling is stringly-typed at the boundary.** `cf` returns `{"ok": false, "error": "..."}` maps; CLI helpers and panel actions return human-readable Indonesian strings that get printed or flashed; HTTP handlers answer JSON `{"error": ...}` with 400/401/500. Nothing panics on bad input: functions return early with a message.
- **Concurrency**: plain goroutines plus `sync.Mutex` (or `sync.RWMutex` where reads dominate), no worker-pool library. `xray.Manager` serializes start/stop/restart, `settings.Sessions` guards the session file, `cf` guards process globals and download progress. Waits are explicit (`time.Sleep` between `cloudflared access` starts so the port is bound); there is no background job queue.
- **No logging framework.** Startup uses `fmt.Printf`; child process output is redirected to `xray.log` / `cloudflared.log` and read back through tail helpers.
- **Path discipline**: every state file is derived from the working directory (`--dir`, defaulting to the binary directory, falling back to cwd when there is no `accounts.txt` there). Never hardcode relative state paths.
- **Panel contract**: `/` is a public status page; `/admin*` and `/api/*` need auth, either HTTP Basic per request (scripts/bots) or the `vb_session` cookie backed by `webui.sessions` (30-day tokens). An authenticated visit to `/` or `/login` redirects to `/admin`. Legacy `/add`, `/del`, `/log`, `/settings` redirect to their `/admin` equivalents so old bookmarks keep working. `/logout` drops the token and clears the cookie.
- **UI strings, comments, and user-facing text are Indonesian** (`"xray mati"`, `"gagal (dial upstream gagal)"`); identifiers, flag names, and package names stay English.
- **Commit style**: Conventional Commits (`fix: xray arm64 asset name (v8a)`), single `master` branch.

Traps worth knowing before editing:

- `accounts.txt` may contain comments and blanks; adding accounts appends then rewrites the file deduped in place. The delete API takes a 0-based **line** index (not the account display index), so keep that mapping straight when touching `/api/delete` or `deleteAccount`.
- **Regeneration must never clobber tunnel credentials.** `tunnelCreds()` prefers `~/.cloudflared/<tunnel-id>.json`, then the path already written in `config.tunnel.yml`; the `<PATH>/<TUNNEL-UUID>.json` placeholder is only emitted when no credentials exist at all. Overwriting a live credentials path silently breaks `/api/regen` for a running tunnel.
- xray and cloudflared are children of the panel process; restarting or killing the panel takes them down, so start them again afterwards. A connector that cannot reach xray makes `cloudflared access` fail with `websocket: bad handshake` rather than a clear origin error, so check the xray manager first when debugging tunnel clients.
- Adding or deleting an account auto-restarts xray if it is running; `/api/regen` alone does not.
- The Cloudflare API token is client-supplied per request (browser `sessionStorage`), never persisted server-side.
- **Tunnel mode: the exit IP is decided by which local port the connection lands on, not by cloudflared.** Every hostname must map to a distinct local port; pointing several `cloudflared access --url` flags at the same port silently sends all accounts through account 1 (one exit IP for everything). Verify with per-port `curl --socks5-hostname`.
- **Cloudflare Tunnel DNS must be proxied (orange) and one level deep.** DNS-only records make the hostname unresolvable (`no such host`) for `cloudflared access`; `socksN.socks.domain.com` fails with `tls: handshake failure` unless `socks.domain.com` is its own zone (Universal SSL covers one level only), which is why the wildcard record created by the Cloudflare page is `*.<zone>`.
- Fresh installs set `"insecure": true` in `webui.settings.json`, disabling TLS verification for generated outbounds.
- **Xray 26 removed `allowInsecure`** (replacement: `pinnedPeerCertSha256`). A config still carrying it fails to load (`exit 23`), which looks like "Start does nothing". `config.XrayMajor` probes the local binary with `xray version`, and when the major is 26 or newer the flag is skipped and the summary says `allowInsecure dilewati`.
- `accounts.txt` holds credentials and is gitignored: never copy its contents into responses, fixtures, logs, commits, or test data.

## Important Files

- `go/cmd/vlessbank/main.go`: subcommand dispatch and the flag sets; `regen`, `addAccounts`, `deleteAccount`, `tunnelCreds`, `uniqEndpoints`.
- `go/internal/config/config.go`: port allocation and outbound construction are the two things to read before touching anything proxy-related.
- `go/internal/web/web_routes.go`: the route table (`/api/status`, `/api/accounts`, `/api/ip`, `/api/ipall`, `/api/proxies`, `/api/add`, `/api/delete`, `/api/xray`, `/api/regen`, `/api/cf/*`); keep route order and JSON/text shapes stable for scripts.
- `go/internal/cf/`: `cf.go` (binary resolution order: settings path, then `dir/cloudflared(.exe)`, then PATH), `tunnel.go`, `api.go` (zones plus wildcard CNAME), `yml.go` (tunnel yml generator and preview).
- `webui.settings.json`: live config keys: `xray`, `base`, `base_http`, `listen`, `auth`, `tunnel`, `tunnel_id`, `xray_path`, `insecure`.
- `install.sh`: installer contract (`VLESSBANK_*` vars, `webui.pid`, `webui.log`); keep it in sync when adding settings keys.
- `DESIGN.md`: the panel's visual contract: tokens in `:root`, full-width layout, status states, no em dash in UI text (R-02).
- `README.md`: authoritative architecture/usage doc, in Indonesian.

## Runtime/Tooling Preferences

- **Go 1.26** (`go/go.mod`). The binary is static, cross-compiles without cgo, and downloads no third-party code; keep it that way.
- **No package manager for the app itself**: no vendored dependencies, no `go.sum` entries.
- **External binaries** `xray` and `cloudflared` are not committed; they are downloaded by `install.sh` or by the panel, and resolved from the working directory or PATH (`xray.exe` / `cloudflared.exe` on Windows). The panel auto-detects them next to the binary and `xray_path` in Pengaturan can override the path.
- **`install.sh` supports Linux and Windows git-bash only** and exits on macOS. It needs `curl` and `git`, and `go` only when the `vlessbank` binary is not present yet.
- Windows is a first-class build target even though the installer is bash.
- Long-running processes (`xray`, `cloudflared tunnel run`, `cloudflared access`) are owned by the panel or by the CLI command that spawned them; stop them cleanly (Ctrl+C closes the HTTP server and stops xray) and never leave orphans behind while testing.
- A pure `go test ./...` run must stay hermetic: no network access and no real `vless` account.

## Testing & QA

- **`go test ./...` is the suite**, and `go vet ./...` must stay clean. Packages with tests: `config`, `settings`, `xray`, `netcheck`, `cf`, `web`.
- Tests are table-driven and use `t.TempDir()` for state files plus `t.Setenv` to fake `PATH` / `HOME`; nothing touches the user's real state files. `xray` and `cf` reuse the test binary as a stub executable through `VB_XRAY_STUB` / `VB_CF_STUB` (the stub sleeps, exits with a chosen code, or prints canned output) so process handling and command output are covered without the real binaries. `web` drives the handler through `httptest` with fake `Deps`.
- Manual verification (the accepted QA path, mirroring the panel's own IP test):
  1. `./vlessbank gen accounts.example.txt -o /tmp/test.json`, confirm the printed `SOCKS :1081 | HTTP :2081 <- name` table.
  2. `xray run -c /tmp/test.json`, then `curl -s --socks5-hostname 127.0.0.1:1081 http://ip-api.com/json/` (needs network and `curl` on PATH).
  3. Panel smoke test: `./vlessbank --port 9000`, log in, start/stop xray, add and delete an account, run "Test Semua IP".
  4. `./vlessbank check accounts.txt --base 1320` for per-account egress verification (exit 1 on `MISMATCH`/`FAIL`).
- If you add tests, keep them stdlib-only (`testing`), out of the runtime artifact paths, and skip anything that needs live network or a real `vless` account by default.
- Because `accounts.txt` holds credentials and is gitignored, never copy its contents into fixtures, logs, commits, or your responses; use `accounts.example.txt` shapes (`vless://uuid@host:443?...#name`) instead.

<!-- antislop:start -->
## antislop
For UI, copy, people, mobile layout, or code comments work, read the antislop core first: `skill://antislop` (file: `C:\Users\ACER\.agents\skills\antislop\SKILL.md`).
No extra skills are installed; when one is added, list its line here:
- UI / visual: `skills/antislop-ui/SKILL.md`
- Copy & text: `skills/antislop-copywriting/SKILL.md`
- People: `skills/antislop-human/SKILL.md`
- Mobile / responsive: `skills/antislop-layoutmobile/SKILL.md`
- Code comments: `skills/antislop-code/SKILL.md`
Before starting, ask the user when antislop applies: during the work, or after it is done.
<!-- antislop:end -->
