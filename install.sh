#!/usr/bin/env bash
set -euo pipefail

REPO="${VLESSBANK_REPO:-jhopan/vless-bank}"
RELEASE="${VLESSBANK_RELEASE:-latest}"
APP_DIR="${VLESSBANK_DIR:-$HOME/pan-vlessbank}"
START="${VLESSBANK_START:-0}"

say() { printf '%s\n' "$*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

platform() {
  case "$(uname -s)/$(uname -m)" in
    Linux/x86_64|Linux/amd64) printf 'linux_amd64' ;;
    Linux/aarch64|Linux/arm64) printf 'linux_arm64' ;;
    Darwin/x86_64|Darwin/amd64) printf 'darwin_amd64' ;;
    Darwin/arm64) printf 'darwin_arm64' ;;
    *) die "platform tidak didukung: $(uname -s) $(uname -m)" ;;
  esac
}

command -v curl >/dev/null || die 'curl belum terpasang'
ASSET="vlessbank_$(platform)"
BASE_URL="https://github.com/$REPO/releases/download/$RELEASE"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -fsSL --retry 3 -o "$TMP_DIR/$ASSET" "$BASE_URL/$ASSET" || die "download gagal: $BASE_URL/$ASSET"
curl -fsSL --retry 3 -o "$TMP_DIR/checksums.txt" "$BASE_URL/checksums.txt" || die 'download checksums gagal'
EXPECTED=$(awk -v asset="$ASSET" '$2==asset {print $1}' "$TMP_DIR/checksums.txt")
[ -n "$EXPECTED" ] || die "checksum $ASSET tidak ditemukan"
if command -v sha256sum >/dev/null; then ACTUAL=$(sha256sum "$TMP_DIR/$ASSET" | awk '{print $1}'); else ACTUAL=$(shasum -a 256 "$TMP_DIR/$ASSET" | awk '{print $1}'); fi
[ "$EXPECTED" = "$ACTUAL" ] || die 'checksum binary tidak cocok'

mkdir -p "$APP_DIR"
install -m 0755 "$TMP_DIR/$ASSET" "$APP_DIR/vlessbank"
say "Installed: $APP_DIR/vlessbank"
say "Release: $RELEASE"
say 'State lama dipertahankan: accounts.txt, config.json, settings, token, dan log tidak diubah.'
say "Jalankan: cd $APP_DIR && ./vlessbank"
if [ "$START" = 1 ]; then
  cd "$APP_DIR"
  nohup ./vlessbank > webui.log 2>&1 &
  say "Panel started: http://127.0.0.1:9000"
fi
