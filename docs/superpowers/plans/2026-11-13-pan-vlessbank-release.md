# Pan-VlessBank Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish reproducible Go binaries for versioned and rolling latest releases, then install them on Linux, macOS, Termux, Git Bash Windows, and native Windows PowerShell.

**Architecture:** GitHub Actions cross-compiles `go/cmd/vlessbank` for Linux amd64/arm64, macOS amd64/arm64, and Windows amd64. A semver tag creates/updates both its immutable versioned release and the rolling `latest` release, using same assets. Installers detect platform, resolve matching release asset, prompt for web port and required proxy credentials, write LAN-bound settings, then start panel.

**Tech Stack:** Go 1.26, GitHub Actions, GitHub CLI release APIs, POSIX shell, PowerShell 5.1+.

---

### Task 1: Define release artifact contract

**Files:**
- Create: `.github/workflows/release.yml`
- Modify: `.gitignore`
- Modify: `README.md`

- [ ] Build exact names: `pan-vlessbank_linux_amd64`, `pan-vlessbank_linux_arm64`, `pan-vlessbank_darwin_amd64`, `pan-vlessbank_darwin_arm64`, `pan-vlessbank_windows_amd64.exe`.
- [ ] Trigger on `v*` tag and manual dispatch with `tag` input.
- [ ] Build with `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"` from `go/`.
- [ ] Upload SHA-256 checksums alongside assets.
- [ ] Publish immutable release at the exact tag and force-update rolling `latest` release assets from same build.

### Task 2: Add installer behavior tests

**Files:**
- Create: `tests/install_smoke.sh`

- [ ] Test shell installer’s platform mapping by sourcing only its pure mapping helpers.
- [ ] Assert Linux amd64/arm64, Termux arm64, Darwin arm64, and MINGW Windows amd64 map to expected binary filenames.
- [ ] Assert unsupported OS returns a failure code.
- [ ] Run test and verify it fails before helpers exist.

### Task 3: Implement Unix installer

**Files:**
- Modify: `install.sh`
- Modify: `.env.example`

- [ ] Detect `Linux`, `Darwin`, Termux (`PREFIX` path under `/data/data/com.termux`), and `MINGW*|MSYS*|CYGWIN*`.
- [ ] Map machine architecture to release asset. Reject unsupported architectures before network access.
- [ ] Fetch release asset and `checksums.txt` from `releases/latest/download/`, verify SHA-256 before execution.
- [ ] Never clone or require Go to install normal releases.
- [ ] Prompt interactively for panel port when `VLESSBANK_WEBUI_PORT` is unset; validate integer 1–65535.
- [ ] Bind panel and generated proxy config to `0.0.0.0` by default. Prompt for `VLESSBANK_AUTH` when unset and reject blank credential input.
- [ ] Preserve env overrides and noninteractive behavior.

### Task 4: Implement native Windows installer

**Files:**
- Create: `install.ps1`

- [ ] Detect Windows amd64/arm64; download `pan-vlessbank_windows_amd64.exe` or fail clear on unsupported architecture.
- [ ] Fetch and verify checksums using `Get-FileHash`.
- [ ] Prompt port and proxy credentials with same validation and LAN defaults as Unix installer.
- [ ] Write `webui.settings.json`, start detached panel process, and print LAN URL.

### Task 5: Document installation and releases

**Files:**
- Modify: `README.md`
- Modify: `.env.example`

- [ ] Add curl installer commands for Unix, Termux, Git Bash Windows, and PowerShell Windows.
- [ ] Explain release channels: exact `vX.Y.Z` versus rolling `latest`.
- [ ] State that LAN bind requires credentials and firewall opening remains operator responsibility.

### Task 6: Verify locally

**Files:**
- Test: `tests/install_smoke.sh`

- [ ] Run installer platform mapping smoke test.
- [ ] Run `go vet ./...` and `go test ./... -count=1` in `go/`.
- [ ] Execute cross-compile commands for all five target binaries.
- [ ] Inspect workflow YAML and generated checksums.

### Task 7: Publish public repository

**Files:**
- Publish: GitHub `jhopan/Pan-VlessBank`

- [ ] Before creating it, confirm `accounts.txt`, binary/runtime artifacts, settings, logs, and sessions are excluded.
- [ ] Ask for confirmation immediately before public repository creation and source push.
- [ ] Create public repository, push `master`, then create/push first `vX.Y.Z` tag.
- [ ] Watch GitHub Actions release run and verify exact tag release plus `latest` release assets.
