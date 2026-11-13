# Pan-VlessBank Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a cross-platform Go client with a local web panel that discovers Pan-VlessBank proxy accounts, starts persistent Cloudflare Access TCP listeners, and exposes stable local SOCKS/HTTP endpoints for Camoufox and other applications.

**Architecture:** Add a protected server endpoint that returns Cloudflare Access hostnames for each account. A new `cmd/pan-vlessbank-client` owns config, Cloudflare child processes, restart state, and a loopback-only local panel. It uses `internal/tunclient` job semantics but introduces a long-lived manager with per-listener state, retry, and stop behavior.

**Tech Stack:** Go 1.26 standard library, existing `internal/tunclient`, `cloudflared access tcp`, GitHub Actions cross-compilation.

---

### Task 1: Define remote discovery contract

**Files:**
- Modify: `go/internal/web/web_routes.go`
- Modify: `go/internal/web/helpers.go`
- Test: `go/internal/web/web_test.go`

- [ ] Add `GET /api/tunnel-client-config` authenticated with existing Basic auth/session rules.
- [ ] Return JSON: `{domain, accounts, auth, socks_base, http_base, jobs:[{idx,kind,hostname,local_port}]}`.
- [ ] Require configured proxy tunnel domain. Return HTTP 400 and `{"error":"tunnel proxy belum diatur"}` without it.
- [ ] Do not return VLESS links, UUIDs, or Cloudflare API tokens.
- [ ] Write a failing `httptest` asserting account 1 maps to `socks1.<domain>` and `http1.<domain>` with local ports 7001 and 8001.

### Task 2: Add persistent listener manager

**Files:**
- Create: `go/internal/client/client.go`
- Create: `go/internal/client/client_test.go`
- Reuse: `go/internal/tunclient/tunclient.go`

- [ ] Define `Config { ServerURL, Username, Password, Cloudflared, SocksBase, HTTPBase }`; persist it as `client.settings.json` mode 0600.
- [ ] Define `Listener { Job, PID, State, Error, StartedAt }` and `Manager` with `Start`, `Stop`, `Refresh`, `Snapshot`.
- [ ] Discover jobs through `/api/tunnel-client-config` using Basic auth over HTTPS.
- [ ] For each job launch `cloudflared access tcp --hostname <hostname> --url 127.0.0.1:<local_port>`.
- [ ] Restart a failed child with capped exponential backoff, expose `reconnecting` state, and never restart after explicit `Stop`.
- [ ] Write failing tests with a test-binary cloudflared stub for: discovery mapping, start produces one child per job, stop kills all children, and failed child transitions to reconnecting.

### Task 3: Add loopback-only client panel

**Files:**
- Create: `go/internal/client/web.go`
- Create: `go/internal/client/web_test.go`
- Create: `go/cmd/pan-vlessbank-client/main.go`

- [ ] Default panel address `127.0.0.1:9900`; reject non-loopback `--listen` unless `--allow-lan-panel` is explicit.
- [ ] Render setup form for server URL, panel login, Cloudflared path, and local base ports.
- [ ] Render listener rows with account, SOCKS/HTTP local URI, hostname, state, error, and Start/Stop/Refresh actions.
- [ ] Implement visible loading/error/empty states and browser-safe copy actions.
- [ ] Keep passwords only in 0600 client settings and never echo them in panel/API responses.
- [ ] Write failing handler tests for unauthenticated access, save config, discovery error, Start action, and rendered local proxy URI.

### Task 4: Add Camoufox workflow surface

**Files:**
- Modify: `README.md`
- Create: `examples/camoufox-proxy.py`

- [ ] Document `pan-vlessbank-client --server https://panel.example.com --user <user>` first-run flow.
- [ ] Add valid Camoufox SOCKS and HTTP examples referencing only `127.0.0.1:7001` / `127.0.0.1:8001`; use placeholder password.
- [ ] Explain Cloudflare Access interactive browser login occurs on client machine and is renewed by Cloudflared.
- [ ] State that per-account exit IP is determined by upstream VLESS, not Cloudflare Access.

### Task 5: Release client binaries

**Files:**
- Modify: `.github/workflows/release.yml`
- Modify: `install.sh`
- Modify: `install.ps1`

- [ ] Build and publish `pan-vlessbank-client_linux_amd64`, `pan-vlessbank-client_linux_arm64`, `pan-vlessbank-client_darwin_amd64`, `pan-vlessbank-client_darwin_arm64`, and `pan-vlessbank-client_windows_amd64.exe` with matching checksums.
- [ ] Keep server installer behavior unchanged; add separate documented client download/install commands.
- [ ] Build client releases with the same `CGO_ENABLED=0`, `-trimpath`, `-s -w` contract.

### Task 6: Verify end to end

**Files:**
- Test: `go/internal/client/client_test.go`
- Test: `go/internal/client/web_test.go`

- [ ] Run `go test ./... -count=1` and `go vet ./...`.
- [ ] Cross-compile server and client for Linux amd64/arm64, macOS amd64/arm64, and Windows amd64.
- [ ] Run a local client panel against a stub discovery endpoint, Start listeners, verify local ports listen, Stop them, and verify children exit.
- [ ] With actual Cloudflare Access configured, run client against `https://panvlessbank.jhopan.my.id`, verify listener 7001, and compare Camoufox/browser IP against proxy curl output.
