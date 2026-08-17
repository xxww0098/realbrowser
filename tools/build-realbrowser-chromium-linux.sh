#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EXPECTED_TAG="151.0.7922.138"
EXPECTED_COMMIT="41fa82442390a4d4456c78f2d69a832d5720cb27"
OUTPUT="${REALBROWSER_CHROMIUM_OUT:-RealBrowser}"
CHROMIUM_GIT_URL="${REALBROWSER_CHROMIUM_GIT_URL:-https://chromium.googlesource.com/chromium/src.git}"
DEPOT_TOOLS_GIT_URL="${REALBROWSER_DEPOT_TOOLS_GIT_URL:-https://chromium.googlesource.com/chromium/tools/depot_tools.git}"

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

pick_writable_path() {
  local preferred="$1"
  local fallback="$2"
  local parent
  parent="$(dirname "$preferred")"
  if mkdir -p "$parent" 2>/dev/null && [[ -w "$parent" ]]; then
    printf '%s\n' "$preferred"
    return 0
  fi
  mkdir -p "$(dirname "$fallback")"
  printf '%s\n' "$fallback"
}

run_as_root() {
  if [[ "$(id -u)" -eq 0 ]]; then
    env DEBIAN_FRONTEND=noninteractive "$@"
  else
    sudo DEBIAN_FRONTEND=noninteractive "$@"
  fi
}

refuse_macho() {
  local path="$1"
  local label="$2"
  file -b "$path" | grep -qi 'mach-o' && die "$label is a macOS Mach-O binary: $path"
}

[[ "$(uname -s)" == "Linux" ]] || die "this build lane requires Linux; refuse Darwin and the macOS product kernel"

if [[ -n "${REALBROWSER_CHROMIUM_SRC:-}" ]]; then
  CHROMIUM_SRC="$REALBROWSER_CHROMIUM_SRC"
else
  CHROMIUM_SRC="$(pick_writable_path \
    "$ROOT/../../BUILD/realbrowser-chromium/src" \
    "$ROOT/.dev/chromium/src")"
fi
if [[ -z "${DEPOT_TOOLS:-}" ]]; then
  DEPOT_TOOLS="$(pick_writable_path \
    "$ROOT/../depot_tools" \
    "$ROOT/.dev/depot_tools")"
fi

printf 'RealBrowser Linux Chromium compile lane (compile-only; not the macOS product kernel)\n'
printf '  chromium src: %s\n' "$CHROMIUM_SRC"
printf '  depot_tools:  %s\n' "$DEPOT_TOOLS"
uname -a
nproc
free -h
df -h

SRC_PARENT="$(dirname "$CHROMIUM_SRC")"
mkdir -p "$SRC_PARENT"
AVAIL_KIB="$(df -Pk "$SRC_PARENT" | awk 'NR==2 {print $4}')"
AVAIL_GIB=$((AVAIL_KIB / 1024 / 1024))
MEM_KIB="$(awk '/MemTotal:/ {print $2}' /proc/meminfo)"
MEM_GIB=$((MEM_KIB / 1024 / 1024))
printf 'resource probe: disk_avail=%sGiB ram_total=%sGiB\n' "$AVAIL_GIB" "$MEM_GIB"
[[ "$AVAIL_GIB" -ge 80 ]] || die "free disk is ${AVAIL_GIB} GiB; need at least 80 GiB before gclient sync or ninja"
[[ "$MEM_GIB" -ge 8 ]] || die "RAM is ${MEM_GIB} GiB; need at least 8 GiB before gclient sync or ninja"

case "$(uname -m)" in
  x86_64) TARGET_CPU="x64" ;;
  aarch64|arm64) TARGET_CPU="arm64" ;;
  *) die "unsupported Linux architecture: $(uname -m)" ;;
esac

python3 -c 'import sys; raise SystemExit(sys.version_info < (3, 10))' \
  || die "Chromium requires Python 3.10 or newer"

run_as_root apt-get update
run_as_root apt-get install -y lsb-release curl python3 git file ca-certificates

if [[ ! -x "$DEPOT_TOOLS/gclient" ]]; then
  git clone --depth=1 "$DEPOT_TOOLS_GIT_URL" "$DEPOT_TOOLS"
fi
[[ -x "$DEPOT_TOOLS/gclient" ]] || die "depot_tools not found: $DEPOT_TOOLS"
export PATH="$DEPOT_TOOLS:$PATH"

if [[ -f "$DEPOT_TOOLS/python3_bin_reldir.txt" ]]; then
  PYTHON_BIN_DIR="$DEPOT_TOOLS/$(sed -n '1p' "$DEPOT_TOOLS/python3_bin_reldir.txt")"
  if [[ -x "$PYTHON_BIN_DIR/python3" ]]; then
    export PATH="$PYTHON_BIN_DIR:$PATH"
  fi
fi

GCLIENT_ROOT="$(dirname "$CHROMIUM_SRC")"
SRC_NAME="$(basename "$CHROMIUM_SRC")"
mkdir -p "$GCLIENT_ROOT"

if [[ ! -f "$GCLIENT_ROOT/.gclient" ]]; then
  cat >"$GCLIENT_ROOT/.gclient" <<EOF
solutions = [
  {
    "name": "$SRC_NAME",
    "url": "$CHROMIUM_GIT_URL",
    "managed": False,
  },
]
target_os = ["linux"]
EOF
fi

if [[ ! -d "$CHROMIUM_SRC/.git" ]]; then
  git clone --depth=1 --branch "$EXPECTED_TAG" "$CHROMIUM_GIT_URL" "$CHROMIUM_SRC"
fi

[[ "$(git -C "$CHROMIUM_SRC" config --bool core.sparseCheckout 2>/dev/null || true)" != "true" ]] \
  || die "Chromium checkout must be complete; sparse checkout is not buildable"

ACTUAL_COMMIT="$(git -C "$CHROMIUM_SRC" rev-parse HEAD)"
if [[ "$ACTUAL_COMMIT" != "$EXPECTED_COMMIT" ]]; then
  git -C "$CHROMIUM_SRC" fetch --depth=1 origin "refs/tags/${EXPECTED_TAG}:refs/tags/${EXPECTED_TAG}"
  git -C "$CHROMIUM_SRC" checkout --detach "$EXPECTED_COMMIT"
  ACTUAL_COMMIT="$(git -C "$CHROMIUM_SRC" rev-parse HEAD)"
fi
[[ "$ACTUAL_COMMIT" == "$EXPECTED_COMMIT" ]] \
  || die "Chromium HEAD must be $EXPECTED_TAG ($EXPECTED_COMMIT), got $ACTUAL_COMMIT"

INSTALL_DEPS="$CHROMIUM_SRC/build/install-build-deps.sh"
[[ -f "$INSTALL_DEPS" ]] || die "Chromium install-build-deps.sh missing: $INSTALL_DEPS"
run_as_root bash "$INSTALL_DEPS" --no-prompt --no-chromeos-fonts
if ! command -v magick >/dev/null && ! command -v convert >/dev/null; then
  run_as_root apt-get update
  run_as_root apt-get install -y imagemagick python3-pil
fi

cd "$GCLIENT_ROOT"
gclient sync --no-history --nohooks --revision "$SRC_NAME@$EXPECTED_COMMIT"
ACTUAL_COMMIT="$(git -C "$CHROMIUM_SRC" rev-parse HEAD)"
[[ "$ACTUAL_COMMIT" == "$EXPECTED_COMMIT" ]] \
  || die "gclient sync moved Chromium HEAD; expected $EXPECTED_COMMIT, got $ACTUAL_COMMIT"
gclient runhooks
ACTUAL_COMMIT="$(git -C "$CHROMIUM_SRC" rev-parse HEAD)"
[[ "$ACTUAL_COMMIT" == "$EXPECTED_COMMIT" ]] \
  || die "gclient runhooks moved Chromium HEAD; expected $EXPECTED_COMMIT, got $ACTUAL_COMMIT"

"$ROOT/tools/apply-realbrowser-chromium-patches.sh" "$CHROMIUM_SRC"

resolve_linux_gn() {
  local src="$1"
  local path
  for path in \
    "$src/buildtools/linux64/gn" \
    "$src/third_party/depot_tools/linux64/gn" \
    "$src/buildtools/linux/gn"; do
    if [[ -x "$path" ]]; then
      refuse_macho "$path" "gn"
      printf '%s\n' "$path"
      return 0
    fi
  done
  if command -v gn >/dev/null; then
    path="$(command -v gn)"
    refuse_macho "$path" "gn"
    printf '%s\n' "$path"
    return 0
  fi
  die "Linux gn not found (refusing buildtools/mac/gn)"
}

resolve_linux_ninja() {
  local src="$1"
  local path
  for path in "$src/third_party/ninja/ninja" "$src/third_party/ninja/ninja-linux64"; do
    if [[ -x "$path" ]]; then
      refuse_macho "$path" "ninja"
      printf '%s\n' "$path"
      return 0
    fi
  done
  if command -v ninja >/dev/null; then
    path="$(command -v ninja)"
    refuse_macho "$path" "ninja"
    printf '%s\n' "$path"
    return 0
  fi
  die "Linux ninja not found"
}

cd "$CHROMIUM_SRC"
GN_BIN="$(resolve_linux_gn "$CHROMIUM_SRC")"
NINJA_BIN="$(resolve_linux_ninja "$CHROMIUM_SRC")"
[[ "$GN_BIN" != *"/buildtools/mac/"* ]] || die "refusing macOS gn: $GN_BIN"
printf 'using gn: %s\nusing ninja: %s\ntarget_cpu: %s\n' "$GN_BIN" "$NINJA_BIN" "$TARGET_CPU"

# concurrent_links=1 keeps the final chrome link from OOM-ing a 8–16 GiB cloud VM with no swap.
"$GN_BIN" gen "out/$OUTPUT" --args="is_debug=false is_component_build=false is_official_build=false is_chrome_branded=false target_cpu=\"${TARGET_CPU}\" symbol_level=0 blink_symbol_level=0 use_siso=false concurrent_links=1"
"$NINJA_BIN" -C "out/$OUTPUT" chrome

CHROME_BIN="$CHROMIUM_SRC/out/$OUTPUT/chrome"
[[ -x "$CHROME_BIN" ]] || die "build completed without chrome: $CHROME_BIN"
"$ROOT/tools/package-realbrowser-kernel-linux.sh" "$CHROMIUM_SRC/out/$OUTPUT" "$ROOT/.dev/kernel"

printf 'Linux RealBrowser Chromium packaged under %s\n' "$ROOT/.dev/kernel"
printf 'This is a compile-only engineering artifact, not the macOS product kernel.\n'
