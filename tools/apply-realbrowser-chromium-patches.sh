#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHROMIUM_ROOT="${1:-}"
EXPECTED_TAG="151.0.7922.138"
EXPECTED_COMMIT="41fa82442390a4d4456c78f2d69a832d5720cb27"
PATCH="$ROOT/chromium/patches/0001-realbrowser-k0-k1.patch"

[[ "$CHROMIUM_ROOT" = /* ]] || { printf 'usage: %s /absolute/path/to/chromium/src\n' "$0" >&2; exit 2; }
[[ -d "$CHROMIUM_ROOT/.git" && -f "$PATCH" ]] || { printf 'error: Chromium checkout or patch missing\n' >&2; exit 1; }

ACTUAL_COMMIT="$(git -C "$CHROMIUM_ROOT" rev-parse HEAD)"
[[ "$ACTUAL_COMMIT" == "$EXPECTED_COMMIT" ]] || {
  printf 'error: Chromium HEAD must be %s (%s), got %s\n' "$EXPECTED_TAG" "$EXPECTED_COMMIT" "$ACTUAL_COMMIT" >&2
  exit 1
}

if rg -q 'RealBrowser requires --realbrowser-persona-file' \
  "$CHROMIUM_ROOT/chrome/app/chrome_main_delegate.cc"; then
  git -C "$CHROMIUM_ROOT" apply --check --reverse "$PATCH"
else
  git -C "$CHROMIUM_ROOT" apply --check "$PATCH"
  git -C "$CHROMIUM_ROOT" apply "$PATCH"
fi

"$ROOT/tools/chromium-brand-realbrowser.sh" "$CHROMIUM_ROOT"
printf 'RealBrowser Chromium K0+K1 applied to %s\n' "$EXPECTED_TAG"
