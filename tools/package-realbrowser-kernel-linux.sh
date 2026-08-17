#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="${1:-}"
DESTINATION="${2:-$ROOT/.dev/kernel}"

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

[[ "$(uname -s)" == "Linux" ]] || die "this packager requires Linux"
[[ -n "$SOURCE" ]] || die "usage: $0 /absolute/path/to/out/RealBrowser [destination]"
[[ "$SOURCE" = /* ]] || die "source directory must be an absolute path"

if [[ -d "$SOURCE" && -x "$SOURCE/chrome" ]]; then
  OUT_DIR="$SOURCE"
elif [[ -f "$SOURCE" && -x "$SOURCE" && "$(basename "$SOURCE")" == "chrome" ]]; then
  OUT_DIR="$(dirname "$SOURCE")"
else
  die "RealBrowser chrome binary not found: $SOURCE"
fi

SOURCE_EXECUTABLE="$OUT_DIR/chrome"
[[ -x "$SOURCE_EXECUTABLE" ]] || die "RealBrowser executable not found: $SOURCE_EXECUTABLE"

VERSION_OUTPUT="$("$SOURCE_EXECUTABLE" --version 2>/dev/null | head -n1 || true)"
if [[ -z "$VERSION_OUTPUT" ]]; then
  VERSION_OUTPUT="$("$SOURCE_EXECUTABLE" --version 2>&1 | head -n1 || true)"
fi
[[ "$VERSION_OUTPUT" == RealBrowser\ * ]] || die "unexpected product version: $VERSION_OUTPUT"
VERSION="${VERSION_OUTPUT#RealBrowser }"
VERSION="${VERSION%% *}"
ENGINE_MAJOR="${VERSION%%.*}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "invalid version: $VERSION"
[[ "$ENGINE_MAJOR" =~ ^[0-9]+$ ]] || die "invalid engine major: $ENGINE_MAJOR"

DESTINATION_PARENT="$(dirname "$DESTINATION")"
mkdir -p "$DESTINATION_PARENT"
STAGING="$(mktemp -d "$DESTINATION_PARENT/.realbrowser-kernel-stage.XXXXXX")"
trap 'rm -rf "$STAGING"' EXIT

copy_file() {
  local src="$1"
  local dest="$2"
  mkdir -p "$(dirname "$dest")"
  cp -a "$src" "$dest"
}

if [[ -f "$OUT_DIR/chrome.runtime_deps" ]]; then
  while IFS= read -r rel || [[ -n "${rel:-}" ]]; do
    [[ -z "${rel:-}" || "$rel" == \#* ]] && continue
    rel="${rel#./}"
    src="$OUT_DIR/$rel"
    if [[ "$rel" == *".."* ]]; then
      resolved="$(realpath -m "$src")"
      [[ -e "$resolved" ]] || continue
      copy_file "$resolved" "$STAGING/$(basename "$resolved")"
      continue
    fi
    if [[ -f "$src" ]]; then
      copy_file "$src" "$STAGING/$rel"
    elif [[ -d "$src" ]]; then
      mkdir -p "$STAGING/$rel"
      cp -a "$src/." "$STAGING/$rel/"
    fi
  done < "$OUT_DIR/chrome.runtime_deps"
fi

copy_file "$SOURCE_EXECUTABLE" "$STAGING/chrome"

PACKAGED_EXECUTABLE="$STAGING/chrome"
[[ -x "$PACKAGED_EXECUTABLE" ]] || die "packaged chrome executable missing"
EXECUTABLE_SHA256="$(sha256sum "$PACKAGED_EXECUTABLE" | awk '{print $1}')"

cat >"$STAGING/realbrowser-kernel.json" <<EOF
{
  "schema_version": 1,
  "product_id": "com.realbrowser.browser",
  "version": "$VERSION",
  "engine_major": $ENGINE_MAJOR,
  "executable": "chrome",
  "executable_sha256": "$EXECUTABLE_SHA256"
}
EOF

if [[ -e "$DESTINATION" ]]; then
  BACKUP="$DESTINATION.previous.$(date +%Y%m%d%H%M%S)"
  mv "$DESTINATION" "$BACKUP"
  printf 'previous kernel moved to: %s\n' "$BACKUP"
fi
mv "$STAGING" "$DESTINATION"
trap - EXIT

printf 'RealBrowser Linux kernel packaged (compile-only engineering artifact)\n  root: %s\n  version: %s\n  executable: %s\n' \
  "$DESTINATION" "$VERSION" "$DESTINATION/chrome"
