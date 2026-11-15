#!/usr/bin/env bash
set -euo pipefail
REPO="${PAN_VLESSBANK_REPO:-jhopan/Pan-VlessBank}"
RELEASE="${PAN_VLESSBANK_RELEASE:-latest}"
APP_DIR="${PAN_VLESSBANK_DIR:-$HOME/pan-vlessbank}"
START="${PAN_VLESSBANK_START:-0}"
say(){ printf '%s\n' "$*"; }; die(){ printf 'ERROR: %s\n' "$*" >&2; exit 1; }
case "$(uname -s)/$(uname -m)" in Linux/x86_64|Linux/amd64) P=linux_amd64;; Linux/aarch64|Linux/arm64) P=linux_arm64;; Darwin/x86_64|Darwin/amd64) P=darwin_amd64;; Darwin/arm64) P=darwin_arm64;; *) die 'platform tidak didukung';; esac
command -v curl >/dev/null || die 'curl belum terpasang'
ASSET="pan-vlessbank_$P"; BASE="https://github.com/$REPO/releases/download/$RELEASE"; TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
curl -fsSL --retry 3 -o "$TMP/$ASSET" "$BASE/$ASSET" || die 'download binary gagal'
curl -fsSL --retry 3 -o "$TMP/checksums.txt" "$BASE/checksums.txt" || die 'download checksum gagal'
EXPECTED=$(awk -v a="$ASSET" '$2==a {print $1}' "$TMP/checksums.txt"); [ -n "$EXPECTED" ] || die 'checksum asset tidak ditemukan'
if command -v sha256sum >/dev/null; then ACTUAL=$(sha256sum "$TMP/$ASSET"|awk '{print $1}'); else ACTUAL=$(shasum -a 256 "$TMP/$ASSET"|awk '{print $1}'); fi
[ "$EXPECTED" = "$ACTUAL" ] || die 'checksum binary tidak cocok'
mkdir -p "$APP_DIR"; install -m 0755 "$TMP/$ASSET" "$APP_DIR/pan-vlessbank"
say "Installed: $APP_DIR/pan-vlessbank"; say "Release: $RELEASE"; say 'State lama dipertahankan.'
if [ "$START" = 1 ]; then cd "$APP_DIR"; nohup ./pan-vlessbank > webui.log 2>&1 & fi
