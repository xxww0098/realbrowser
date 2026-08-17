#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CALLER_CWD="$(pwd)"
RUN_DIR="$ROOT/.dev"
MACOS_DEV_APP_NAME="RealBrowser Dev"
MACOS_DEV_BUNDLE_ID="com.realbrowser.desktop.tauri.dev"
MACOS_DEV_APP="$RUN_DIR/macos/${MACOS_DEV_APP_NAME}.app"
MACOS_DEV_EXECUTABLE="$MACOS_DEV_APP/Contents/MacOS/realbrowser-desktop"
DEV_DATA_ROOT="$RUN_DIR/data"
DEV_KERNEL_ROOT="${REALBROWSER_KERNEL_DIR:-$RUN_DIR/kernel}"
DEV_URL="http://127.0.0.1:1431"

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "missing command: $1"
}

write_macos_dev_info_plist() {
  local destination="$1"
  local temporary="${destination}.tmp.$$"
  cat >"$temporary" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key><string>English</string>
  <key>CFBundleDisplayName</key><string>${MACOS_DEV_APP_NAME}</string>
  <key>CFBundleExecutable</key><string>realbrowser-desktop</string>
  <key>CFBundleIconFile</key><string>icon.icns</string>
  <key>CFBundleIdentifier</key><string>${MACOS_DEV_BUNDLE_ID}</string>
  <key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
  <key>CFBundleName</key><string>${MACOS_DEV_APP_NAME}</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>0.0.0</string>
  <key>CFBundleVersion</key><string>0.0.0</string>
  <key>LSApplicationCategoryType</key><string>public.app-category.business</string>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
EOF
  mv "$temporary" "$destination"
}

run_macos_desktop_app() {
  [[ "$(uname -s)" == "Darwin" ]] || die "the macOS dev app runner requires macOS"
  [[ $# -gt 0 ]] || die "the macOS dev app runner requires a Cargo executable"
  need codesign

  local source_executable="$1"
  shift
  if [[ "$source_executable" != /* ]]; then
    source_executable="$CALLER_CWD/$source_executable"
  fi
  [[ -x "$source_executable" ]] || die "Cargo executable is not runnable: $source_executable"

  local contents="$MACOS_DEV_APP/Contents"
  local executable_tmp="${MACOS_DEV_EXECUTABLE}.tmp.$$"
  mkdir -p "$contents/MacOS" "$contents/Resources"
  write_macos_dev_info_plist "$contents/Info.plist"
  cp "$ROOT/apps/desktop/src-tauri/icons/icon.icns" "$contents/Resources/icon.icns"
  cp "$source_executable" "$executable_tmp"
  chmod +x "$executable_tmp"
  mv "$executable_tmp" "$MACOS_DEV_EXECUTABLE"
  codesign --force --sign - "$MACOS_DEV_APP" >/dev/null

  printf 'RealBrowser dev app ready\n  app: %s\n  bundle: %s\n' "$MACOS_DEV_APP" "$MACOS_DEV_BUNDLE_ID"
  cd "$CALLER_CWD"
  exec "$MACOS_DEV_EXECUTABLE" "$@"
}

macos_dev_desktop_pid() {
  [[ "$(uname -s)" == "Darwin" ]] || return 1
  need lsappinfo
  need pgrep
  local pid info
  for pid in $(pgrep -f 'realbrowser-desktop' 2>/dev/null || true); do
    info="$(lsappinfo info -only bundleID -only bundlepath -only executablepath -app "$pid" 2>/dev/null || true)"
    if [[ "$info" == *"\"CFBundleIdentifier\"=\"${MACOS_DEV_BUNDLE_ID}\""* \
      && "$info" == *"\"LSBundlePath\"=\"${MACOS_DEV_APP}\""* \
      && "$info" == *"\"CFBundleExecutablePath\"=\"${MACOS_DEV_EXECUTABLE}\""* ]]; then
      printf '%s\n' "$pid"
      return 0
    fi
  done
  return 1
}

show_status() {
  local pid
  pid="$(macos_dev_desktop_pid || true)"
  printf 'frontend: %s\n' "$DEV_URL"
  printf 'desktop app: %s\n' "$MACOS_DEV_APP"
  printf 'bundle: %s\n' "$MACOS_DEV_BUNDLE_ID"
  printf 'data: %s\n' "$DEV_DATA_ROOT"
  printf 'kernel: %s\n' "$DEV_KERNEL_ROOT"
  if [[ -n "$pid" ]]; then
    printf 'identity: ready (pid=%s)\n' "$pid"
  else
    printf 'identity: not ready\n'
    return 1
  fi
}

check_macos_desktop() {
  show_status
  plutil -lint "$MACOS_DEV_APP/Contents/Info.plist"
  codesign --verify --deep --strict --verbose=2 "$MACOS_DEV_APP"
}

run_dev() {
  need pnpm
  need cargo
  need rustc
  [[ "$(uname -s)" == "Darwin" ]] || die "./dev.sh currently provides the stable Computer Use shell on macOS"
  local host runner_key runner_value
  host="$(rustc -vV | sed -n 's/^host: //p')"
  [[ "$host" == *-apple-darwin ]] || die "unexpected Rust host: $host"
  runner_key="CARGO_TARGET_$(printf '%s' "$host" | tr '[:lower:]-' '[:upper:]_')_RUNNER"
  runner_value="$ROOT/dev.sh __run-macos-desktop"
  export "$runner_key=$runner_value"
  export REALBROWSER_DEV_DATA_ROOT="$DEV_DATA_ROOT"
  export REALBROWSER_KERNEL_DIR="$DEV_KERNEL_ROOT"
  cd "$ROOT"
  exec pnpm --filter @realbrowser/desktop tauri dev "$@"
}

case "${1:-start}" in
  start)
    shift || true
    run_dev "$@"
    ;;
  status)
    show_status
    ;;
  __check-macos-desktop)
    check_macos_desktop
    ;;
  __run-macos-desktop)
    shift
    run_macos_desktop_app "$@"
    ;;
  help|-h|--help)
    printf 'Usage: ./dev.sh [start|status|__check-macos-desktop]\n'
    ;;
  *)
    die "unknown command: $1"
    ;;
esac
