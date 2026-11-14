# Operations Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make deployed Pan-VlessBank restart-safe, service-owned for proxy tunnels, scoped per client, observable, and recoverable without exposing credentials.

**Architecture:** File-backed state remains stdlib-only. `webui.runtime.json` records desired Xray state. `systemctl` is abstracted behind a small command dependency for proxy tunnel service status/actions. Client tokens contain a hash and account scope only. Health snapshots compare current process/service state and append transition-only audit events.

**Tech Stack:** Go 1.26 standard library, systemd on deployed Linux, existing Cloudflare Tunnel and HTTP panel.

---

### Task 1: Persist Xray desired state

**Files:**
- Create: `go/internal/settings/runtime.go`
- Create: `go/internal/settings/runtime_test.go`
- Modify: `go/cmd/vlessbank/main.go`
- Modify: `go/internal/web/web.go`
- Modify: `go/internal/web/web_routes.go`

- [ ] Write a failing test that `RuntimeStore.SetXrayEnabled(true)` survives reload and a missing state file defaults false.
- [ ] Run `cd go && go test ./internal/settings -run TestRuntime -count=1`; expect undefined symbols.
- [ ] Implement JSON `{"xray_enabled": bool}` mode `0600`; startup starts Xray only when enabled and config exists. Start marks enabled only after successful start. Stop marks disabled before stop. Restart preserves enabled.
- [ ] Re-run test; expect PASS.

### Task 2: Single proxy-tunnel owner

**Files:**
- Create: `go/internal/service/service.go`
- Create: `go/internal/service/service_test.go`
- Modify: `go/internal/web/web.go`
- Modify: `go/internal/web/web_routes.go`

- [ ] Write failing service parser test for `systemctl is-active` and action command construction.
- [ ] Implement `Manager` with injected runner, status `active`, and `Start`/`Stop`/`Restart`; no shell interpolation.
- [ ] Replace proxy `cf.Start`/`cf.Stop` routes with `pan-vlessbank-proxy-tunnel.service` actions when configured; preserve panel child mode only on unsupported hosts.
- [ ] Test unit and web routes.

### Task 3: Scoped client tokens and secret redaction

**Files:**
- Modify: `go/internal/settings/tokens.go`
- Modify: `go/internal/settings/tokens_test.go`
- Modify: `go/internal/web/helpers.go`
- Modify: `go/internal/web/web_routes.go`
- Modify: `go/internal/web/pages_dash.go`
- Modify: `go/internal/web/web_test.go`

- [ ] Write failing test for scope `[2]`: Bearer API returns only account 2.
- [ ] Add `Accounts []int` to token record, token create validation against current account IDs, lookup returns authorized scope, empty scope means all for backward compatibility.
- [ ] Make client API filter proxy hostname list by scope. Never expose scope hash or raw token.
- [ ] Replace Dashboard proxy auth value with `aktif` or `tanpa auth` only.
- [ ] Run web/settings tests.

### Task 4: Audit, redacted restore preview, health transition events

**Files:**
- Modify: `go/internal/ops/ops.go`
- Modify: `go/internal/ops/ops_test.go`
- Modify: `go/internal/web/web.go`
- Modify: `go/internal/web/web_routes.go`

- [ ] Write failing test: audit event contains actor/action/result and no supplied secret. Write failing test: redacted ZIP preview rejects unknown format and returns only allowed settings.
- [ ] Add actor `admin` from login/session, result `ok|error`, and safe summary. Add `POST /api/backup/preview`; accepts ZIP up to 1 MiB and returns settings diff/account count without writes.
- [ ] Add health monitor snapshot. Emit audit only when xray/proxy tunnel/web tunnel/config/tunnel validation changes.
- [ ] Test all edges.

### Task 5: Panel and docs

**Files:**
- Modify: `go/internal/web/pages_settings.go`
- Modify: `go/internal/web/layout.go`
- Modify: `README.md`

- [ ] Add account selector for client-token scope, Xray auto-start state, systemd proxy tunnel status, backup preview upload. Follow `DESIGN.md`: existing dark panel, labelled values, no credentials rendered, keyboard focus, mobile layout.
- [ ] Document Xray persistence, service ownership, scoped tokens, restore preview, health transitions, and Access policy requirement.

### Task 6: Access policy and deployment

**Files:**
- Modify: deployed `/etc/systemd/system/pan-vlessbank.service`
- Modify: deployed `/etc/systemd/system/pan-vlessbank-proxy-tunnel.service`

- [ ] Audit Cloudflare Access application/policy through authenticated dashboard. Ensure all four proxy hostnames, or a wildcard covering them, require Access login.
- [ ] Add `Requires=pan-vlessbank.service` and `After=pan-vlessbank.service` only if desired shared lifecycle is correct; proxy tunnel must remain persistent independently across panel restarts.
- [ ] Deploy release with checksum verification. Restart panel. Verify automatic Xray resume, systemd proxy tunnel persistence, scoped token API, health response, and SOCKS egress from a fresh client session.
