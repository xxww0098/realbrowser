#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export REALBROWSER_DEV_DATA_ROOT="${REALBROWSER_DEV_DATA_ROOT:-$ROOT/.dev/data}"
mkdir -p "$REALBROWSER_DEV_DATA_ROOT"

if [[ "${1:-}" == "status" ]]; then
  printf 'shell: gpui-kit\n'
  printf 'data: %s\n' "$REALBROWSER_DEV_DATA_ROOT"
  printf 'identity: run ./dev.sh to open the RealBrowser window\n'
  exit 0
fi

exec cargo run -p realbrowser-desktop --manifest-path "$ROOT/Cargo.toml" --features shell -- "$@"
