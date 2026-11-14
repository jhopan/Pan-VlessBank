# Server Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Pan-VlessBank a revocable read-only client token API, truthful tunnel health, bounded lifecycle control, audit records, and redacted backup export.

**Architecture:** Keep file-backed state and Go stdlib only. Add `internal/settings/tokens.go` for hashed client-token records and `internal/ops` for audit, backup, health, and tunnel validation. `web.Server` authorizes `/api/client/proxies` only through Bearer client tokens; admin session or Basic auth remains required for every management endpoint.

**Tech Stack:** Go 1.26, standard library, file-backed JSON, existing `net/http`, `xray.Manager`, `cf` package.

---

### Task 1: Client-token persistence

**Files:**
- Create: `go/internal/settings/tokens.go`
- Create: `go/internal/settings/tokens_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestClientTokenAuthorizesAndRevokes(t *testing.T) {
    store := NewClientTokens(filepath.Join(t.TempDir(), "webui.client-tokens.json"))
    raw, record, err := store.Create("camoufox", 24*time.Hour)
    if err != nil { t.Fatal(err) }
    if !store.Check(raw) { t.Fatal("created token rejected") }
    if err := store.Revoke(record.ID); err != nil { t.Fatal(err) }
    if store.Check(raw) { t.Fatal("revoked token accepted") }
}
```

- [ ] **Step 2: Run RED**

Run: `cd go && go test ./internal/settings -run TestClientTokenAuthorizesAndRevokes -count=1`

Expected: FAIL because `NewClientTokens` does not exist.

- [ ] **Step 3: Implement token store**

Use `crypto/rand` for 32 bytes. Return raw base64url token exactly once. Persist only SHA-256 token hash, ID, label, creation, expiry, and revocation. File mode `0600`. Use constant-time compare in `Check`.

- [ ] **Step 4: Run GREEN**

Run: `cd go && go test ./internal/settings -run TestClientTokenAuthorizesAndRevokes -count=1`

Expected: PASS.

### Task 2: Restricted client proxy API

**Files:**
- Modify: `go/internal/web/web.go`
- Modify: `go/internal/web/web_routes.go`
- Modify: `go/internal/web/helpers.go`
- Modify: `go/internal/web/web_test.go`

- [ ] **Step 1: Write failing handler test**

```go
func TestClientProxyAPIOnlyReturnsPublicEndpoints(t *testing.T) {
    // bearer token authenticates /api/client/proxies
    // response must not contain upstream, LAN host, proxy auth, or tunnel command
}
```

- [ ] **Step 2: Run RED**

Run: `cd go && go test ./internal/web -run TestClientProxyAPIOnlyReturnsPublicEndpoints -count=1`

Expected: FAIL with 404 or unauthorized.

- [ ] **Step 3: Implement restricted response and Bearer auth**

Add `GET /api/client/proxies`. Response contains only `idx`, `name`, `socks_host`, `http_host`. Reject missing, expired, and revoked Bearer tokens with JSON 401. Keep `/api/proxies` admin-only and unchanged for compatibility.

- [ ] **Step 4: Run GREEN**

Run: `cd go && go test ./internal/web -run TestClientProxyAPIOnlyReturnsPublicEndpoints -count=1`

Expected: PASS.

### Task 3: Health, validation, audit, backup

**Files:**
- Create: `go/internal/ops/ops.go`
- Create: `go/internal/ops/ops_test.go`
- Modify: `go/internal/web/web.go`
- Modify: `go/internal/web/web_routes.go`
- Modify: `go/cmd/vlessbank/main.go`

- [ ] **Step 1: Write failing tests**

```go
func TestProxyTunnelValidationDetectsCollapsedPorts(t *testing.T) {
    got := ValidateProxyTunnel("ingress:\n- hostname: socks1.example.com\n  service: tcp://127.0.0.1:1081\n- hostname: socks2.example.com\n  service: tcp://127.0.0.1:1081\n")
    if got.OK { t.Fatal("collapsed ingress accepted") }
}
func TestBackupRedactsSecrets(t *testing.T) {
    b, err := RedactedBackup(dir)
    if err != nil { t.Fatal(err) }
    if bytes.Contains(b, []byte("vless://")) || bytes.Contains(b, []byte("secret")) { t.Fatal("secret leaked") }
}
```

- [ ] **Step 2: Run RED**

Run: `cd go && go test ./internal/ops -count=1`

Expected: FAIL because package does not exist.

- [ ] **Step 3: Implement operations helpers**

Health reports actual Xray process, proxy tunnel process, WebUI tunnel process, config presence, and per-account tunnel mapping validation. Validation parses generated YAML service lines, requires distinct SOCKS and HTTP local ports per account. Audit append-only JSON lines records action, UTC time, and safe summary. Backup ZIP contains redacted settings and account count only, never `accounts.txt`, credentials, token files, logs, or tunnel credentials.

- [ ] **Step 4: Run GREEN**

Run: `cd go && go test ./internal/ops -count=1`

Expected: PASS.

### Task 4: Token management and safe admin actions

**Files:**
- Modify: `go/internal/web/web_routes.go`
- Modify: `go/internal/web/web_test.go`

- [ ] **Step 1: Write failing authorization and audit tests**

```go
func TestTokenManagementRequiresAdmin(t *testing.T) { /* POST rejected without admin auth */ }
func TestAdminLifecycleActionWritesAudit(t *testing.T) { /* start/stop/regen writes safe audit record */ }
```

- [ ] **Step 2: Run RED**

Run: `cd go && go test ./internal/web -run 'Test(TokenManagementRequiresAdmin|AdminLifecycleActionWritesAudit)' -count=1`

Expected: FAIL because endpoints/audit hook do not exist.

- [ ] **Step 3: Implement admin-only routes**

Add create/list/revoke token, `GET /api/health`, `GET /api/audit`, and `GET /api/backup`. Put backup and audit behind current admin auth. Audit config changes, token changes, and Xray/tunnel start/stop/restart. Never record raw token, Basic password, VLESS URI, API token, or hostname command.

- [ ] **Step 4: Run GREEN**

Run: `cd go && go test ./internal/web -count=1`

Expected: PASS.

### Task 5: Client migration

**Files:**
- Modify: `../Pan-VlessBank-Client/internal/client/client.go`
- Modify: `../Pan-VlessBank-Client/internal/client/discovery_test.go`
- Modify: `../Pan-VlessBank-Client/cmd/pan-vlessbank-client/main.go`

- [ ] **Step 1: Write failing discovery test**

```go
func TestDiscoverUsesBearerClientToken(t *testing.T) {
    // server expects Authorization: Bearer client-token
    // only socks_host/http_host response fields are consumed
}
```

- [ ] **Step 2: Run RED**

Run: `cd ../Pan-VlessBank-Client && go test ./internal/client -run TestDiscoverUsesBearerClientToken -count=1`

Expected: FAIL because Basic auth and old JSON format remain.

- [ ] **Step 3: Implement migration**

Replace Username/Password with `ClientToken`; call `/api/client/proxies`; remove all Basic auth behavior. Keep client config permission `0600`. UI change waits for user antislop mode selection.

- [ ] **Step 4: Run GREEN**

Run: `cd ../Pan-VlessBank-Client && go test ./... -count=1 && go vet ./...`

Expected: PASS.

### Task 6: Documentation and verification

**Files:**
- Modify: `README.md`
- Modify: `../Pan-VlessBank-Client/README.md`

- [ ] **Step 1: Document token flow and operational checks**

Document: create token once, copy raw value once, client API scope, revocation, health conditions, backup contents, and tunnel validation failure meaning. Explicitly state proxy routing still relies on Cloudflare Access.

- [ ] **Step 2: Run full verification**

Run:

```bash
cd go && go test ./... -count=1 && go vet ./... && go build ./cmd/vlessbank
cd ../Pan-VlessBank-Client && go test ./... -count=1 && go vet ./... && go build ./cmd/pan-vlessbank-client
```

Expected: all commands exit 0.

- [ ] **Step 3: Smoke both binaries**

Start server and client on loopback, request health unauthenticated and token API with created token, verify unauthorized/authorized behavior, then stop processes cleanly.

- [ ] **Step 4: Release after UI completion**

Tag server and client only after backend and UI changes both pass automated verification and loopback smoke tests.
