#!/usr/bin/env bash
# Pan-VlessBank release installer. Linux, macOS, Termux, and Git Bash Windows.
set -euo pipefail

REPO="${VLESSBANK_REPO:-jhopan/Pan-VlessBank}"
RELEASE="${VLESSBANK_RELEASE:-latest}"
APP_DIR="${VLESSBANK_DIR:-$HOME/pan-vlessbank}"
WEBUI_PORT="${VLESSBANK_WEBUI_PORT:-}"
BASE_PORT="${VLESSBANK_BASE_PORT:-1081}"
BASE_HTTP="${VLESSBANK_BASE_HTTP:-0}"
LISTEN="${VLESSBANK_LISTEN:-0.0.0.0}"
AUTH="${VLESSBANK_AUTH:-}"
START="${VLESSBANK_START:-1}"

say() { printf '%s\n' "$*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; return 1; }

release_asset_for() {
  local os=$1 arch=$2 family=
  if [ "$os" = Termux ]; then family=linux
  else
    case "$os" in
      Linux) family=linux ;;
      Darwin) family=darwin ;;
      MINGW*|MSYS*|CYGWIN*) family=windows ;;
      *) die "OS '$os' tidak didukung"; return 1 ;;
    esac
  fi
  case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) die "arsitektur '$arch' tidak didukung"; return 1 ;;
  esac
  case "$family/$arch" in
    linux/amd64|linux/arm64|darwin/amd64|darwin/arm64) printf 'pan-vlessbank_%s_%s\n' "$family" "$arch" ;;
    windows/amd64) printf 'pan-vlessbank_windows_amd64.exe\n' ;;
    *) die "$family/$arch belum tersedia"; return 1 ;;
  esac
}

if [ "${VLESSBANK_LIB_ONLY:-0}" = 1 ]; then return 0; fi

command -v curl >/dev/null || { die 'curl belum terpasang'; exit 1; }
OS=$(uname -s)
[ "${PREFIX:-}" != "" ] && case "$PREFIX" in /data/data/com.termux/*) OS=Termux ;; esac
ASSET=$(release_asset_for "$OS" "$(uname -m)") || exit 1
case "$ASSET" in *.exe) BIN_NAME=pan-vlessbank.exe ;; *) BIN_NAME=pan-vlessbank ;; esac

if [ -z "$WEBUI_PORT" ]; then
  if [ -t 0 ]; then read -r -p 'Port panel [9000]: ' WEBUI_PORT; fi
  WEBUI_PORT=${WEBUI_PORT:-9000}
fi
case "$WEBUI_PORT" in ''|*[!0-9]*) die 'port harus angka 1 sampai 65535'; exit 1 ;; esac
[ "$WEBUI_PORT" -ge 1 ] && [ "$WEBUI_PORT" -le 65535 ] || { die 'port harus 1 sampai 65535'; exit 1; }

if [ "$LISTEN" = 0.0.0.0 ] && [ -z "$AUTH" ]; then
  if [ -t 0 ]; then read -r -p 'Auth proxy user:pass (wajib untuk bind LAN): ' AUTH; fi
  [ -n "$AUTH" ] || { die 'bind LAN 0.0.0.0 perlu VLESSBANK_AUTH=user:pass'; exit 1; }
fi

mkdir -p "$APP_DIR"
cd "$APP_DIR"
BASE_URL="https://github.com/$REPO/releases/download/$RELEASE"
curl -fsSL --retry 3 -o "$BIN_NAME" "$BASE_URL/$ASSET" || { rm -f "$BIN_NAME"; die "download gagal: $BASE_URL/$ASSET"; exit 1; }
curl -fsSL --retry 3 -o checksums.txt "$BASE_URL/checksums.txt" || { rm -f "$BIN_NAME"; die 'download checksums gagal'; exit 1; }
if command -v sha256sum >/dev/null; then
  grep "  $ASSET$" checksums.txt | sha256sum -c -
elif command -v shasum >/dev/null; then
  expected=$(awk -v n="$ASSET" '$2==n {print $1}' checksums.txt)
  [ -n "$expected" ] && [ "$expected" = "$(shasum -a 256 "$BIN_NAME" | awk '{print $1}')" ]
else
  die 'butuh sha256sum atau shasum untuk verifikasi release'; exit 1
fi || { rm -f "$BIN_NAME"; die 'checksum binary tidak cocok'; exit 1; }
chmod +x "$BIN_NAME" 2>/dev/null || true

cat > webui.settings.json <<EOF
{
  "xray": "",
  "base": $BASE_PORT,
  "base_http": $BASE_HTTP,
  "listen": "$LISTEN",
  "auth": "$AUTH",
  "tunnel": "",
  "tunnel_id": "",
  "xray_path": "xray",
  "insecure": true
}
EOF

say "Binary verified: $BIN_NAME"
say "Panel bind: http://$LISTEN:$WEBUI_PORT/"
say 'Proxy bind: 0.0.0.0 with auth required.'
say 'Install xray and cloudflared in this folder or make both available on PATH.'
if [ "$START" = 1 ]; then
  nohup "./$BIN_NAME" --port "$WEBUI_PORT" --listen "$LISTEN" --dir "$APP_DIR" > webui.log 2>&1 &
  echo $! > webui.pid
  say "Panel started, pid $(cat webui.pid)."
fi
