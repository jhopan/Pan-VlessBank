#!/usr/bin/env bash
set -euo pipefail

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
export VLESSBANK_LIB_ONLY=1
# shellcheck source=../install.sh
. "$ROOT/install.sh"

check() {
  local os=$1 arch=$2 expected=$3 actual
  actual=$(release_asset_for "$os" "$arch")
  [ "$actual" = "$expected" ] || {
    printf 'FAIL %s/%s: want %s, got %s\n' "$os" "$arch" "$expected" "$actual" >&2
    exit 1
  }
}

check Linux x86_64 pan-vlessbank_linux_amd64
check Linux aarch64 pan-vlessbank_linux_arm64
check Termux aarch64 pan-vlessbank_linux_arm64
check MINGW64_NT-10.0 x86_64 pan-vlessbank_windows_amd64.exe

if release_asset_for FreeBSD x86_64 >/dev/null 2>&1; then
  printf 'FAIL FreeBSD must be unsupported\n' >&2
  exit 1
fi

printf 'PASS installer platform mapping\n'
