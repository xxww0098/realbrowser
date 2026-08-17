#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_APP="${1:-}"
DESTINATION="${2:-$ROOT/.dev/kernel}"

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

[[ "$(uname -s)" == "Darwin" ]] || die "this packager requires macOS"
[[ -n "$SOURCE_APP" ]] || die "usage: $0 /absolute/path/to/RealBrowser.app [destination]"
[[ "$SOURCE_APP" = /* ]] || die "source app must be an absolute path"
[[ -d "$SOURCE_APP" ]] || die "RealBrowser.app not found: $SOURCE_APP"

SOURCE_EXECUTABLE="$SOURCE_APP/Contents/MacOS/RealBrowser"
[[ -x "$SOURCE_EXECUTABLE" ]] || die "RealBrowser executable not found: $SOURCE_EXECUTABLE"

VERSION_OUTPUT="$($SOURCE_EXECUTABLE --version 2>&1)"
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

ditto "$SOURCE_APP" "$STAGING/RealBrowser.app"
PACKAGED_EXECUTABLE="$STAGING/RealBrowser.app/Contents/MacOS/RealBrowser"
EXECUTABLE_SHA256="$(shasum -a 256 "$PACKAGED_EXECUTABLE" | awk '{print $1}')"

cat >"$STAGING/realbrowser-kernel.json" <<EOF
{
  "schema_version": 1,
  "product_id": "com.realbrowser.browser",
  "version": "$VERSION",
  "engine_major": $ENGINE_MAJOR,
  "executable": "RealBrowser.app/Contents/MacOS/RealBrowser",
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

printf 'RealBrowser kernel packaged\n  root: %s\n  version: %s\n  executable: %s\n' \
  "$DESTINATION" "$VERSION" "$DESTINATION/RealBrowser.app/Contents/MacOS/RealBrowser"
