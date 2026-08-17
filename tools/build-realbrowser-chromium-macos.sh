#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHROMIUM_SRC="${REALBROWSER_CHROMIUM_SRC:-$ROOT/../../BUILD/realbrowser-chromium/src}"
DEPOT_TOOLS="${DEPOT_TOOLS:-$ROOT/../depot_tools}"
OUTPUT="${REALBROWSER_CHROMIUM_OUT:-RealBrowser}"
OFFLINE_SOURCE_ARCHIVE="${REALBROWSER_CHROMIUM_OFFLINE_SOURCE_ARCHIVE:-0}"
XCODE_APP="${REALBROWSER_XCODE_APP:-/Applications/Xcode.app}"

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

[[ "$(uname -s)" == "Darwin" ]] || die "this build lane requires macOS"
[[ -d "$CHROMIUM_SRC/.git" ]] || die "Chromium checkout not found: $CHROMIUM_SRC"
[[ "$OFFLINE_SOURCE_ARCHIVE" == "0" || "$OFFLINE_SOURCE_ARCHIVE" == "1" ]] \
  || die "REALBROWSER_CHROMIUM_OFFLINE_SOURCE_ARCHIVE must be 0 or 1"
[[ "$(git -C "$CHROMIUM_SRC" config --bool core.sparseCheckout 2>/dev/null || true)" != "true" ]] \
  || die "Chromium checkout must be complete; sparse checkout is not buildable"

if [[ "$OFFLINE_SOURCE_ARCHIVE" == "0" ]]; then
  [[ -x "$DEPOT_TOOLS/gclient" ]] || die "depot_tools not found: $DEPOT_TOOLS"
else
  for tool in \
    buildtools/mac/gn \
    third_party/ninja/ninja \
    third_party/llvm-build/Release+Asserts/bin/clang++ \
    third_party/rust-toolchain/bin/rustc; do
    [[ -x "$CHROMIUM_SRC/$tool" ]] || die "offline source archive is missing host tool: $tool"
    file "$CHROMIUM_SRC/$tool" | grep -q 'Mach-O' \
      || die "offline source archive host tool is not a macOS executable: $tool"
  done
  (
    cd "$CHROMIUM_SRC"
    python3 tools/clang/scripts/update.py --print-revision >/dev/null
    python3 tools/rust/update_rust.py --print-revision=validate >/dev/null
  ) || die "offline source archive toolchain does not match the pinned Chromium revision"
fi

if [[ -f "$DEPOT_TOOLS/python3_bin_reldir.txt" ]]; then
  PYTHON_BIN_DIR="$DEPOT_TOOLS/$(sed -n '1p' "$DEPOT_TOOLS/python3_bin_reldir.txt")"
  [[ -x "$PYTHON_BIN_DIR/python3" ]] \
    || die "depot_tools Python not found: $PYTHON_BIN_DIR/python3"
  export PATH="$PYTHON_BIN_DIR:$PATH"
fi
python3 -c 'import sys; raise SystemExit(sys.version_info < (3, 10))' \
  || die "Chromium requires Python 3.10 or newer"

[[ -d "$XCODE_APP" ]] || die "full Xcode.app is required to build Chromium"
export DEVELOPER_DIR="${DEVELOPER_DIR:-$XCODE_APP/Contents/Developer}"
[[ -x "$DEVELOPER_DIR/usr/bin/xcodebuild" ]] \
  || die "Xcode developer tools not found: $DEVELOPER_DIR"

if [[ "$OFFLINE_SOURCE_ARCHIVE" == "0" ]]; then
  export PATH="$DEPOT_TOOLS:$PATH"
  GCLIENT_ROOT="$(dirname "$CHROMIUM_SRC")"
  cd "$GCLIENT_ROOT"
  if [[ ! -f .gclient ]]; then
    gclient config --name="$(basename "$CHROMIUM_SRC")" --unmanaged https://chromium.googlesource.com/chromium/src.git
  fi
  gclient sync --no-history
fi

"$ROOT/tools/apply-realbrowser-chromium-patches.sh" "$CHROMIUM_SRC"

cd "$CHROMIUM_SRC"
"$CHROMIUM_SRC/buildtools/mac/gn" gen "out/$OUTPUT" --args='is_debug=false is_component_build=false is_official_build=false is_chrome_branded=false target_cpu="arm64" symbol_level=0 blink_symbol_level=0 use_siso=false'
"$CHROMIUM_SRC/third_party/ninja/ninja" -C "out/$OUTPUT" chrome

APP="$CHROMIUM_SRC/out/$OUTPUT/RealBrowser.app"
[[ -d "$APP" ]] || die "build completed without RealBrowser.app: $APP"
"$ROOT/tools/package-realbrowser-kernel-macos.sh" "$APP" "$ROOT/.dev/kernel"
