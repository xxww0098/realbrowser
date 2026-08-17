#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 /absolute/path/to/chromium/src" >&2
  exit 2
fi

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
chromium_root="$1"
theme_root="$chromium_root/chrome/app/theme/chromium"
source_png="$repo_root/apps/desktop/src-tauri/icons/app-icon-1024.png"

test -f "$chromium_root/DEPS"
test -d "$theme_root"
test -f "$source_png"

cp "$repo_root/apps/desktop/src-tauri/icons/icon.icns" \
  "$theme_root/mac/app.icns"
cp "$repo_root/apps/desktop/src-tauri/icons/icon.ico" \
  "$theme_root/win/chromium.ico"
cp "$repo_root/apps/desktop/src/assets/logo-brand.svg" \
  "$theme_root/product_logo.svg"

for size in 16 24 48 64 128 256; do
  sips -s format png -z "$size" "$size" "$source_png" \
    --out "$theme_root/product_logo_${size}.png" >/dev/null
done

for size in 24 48 64 128 256; do
  sips -s format png -z "$size" "$size" "$source_png" \
    --out "$theme_root/linux/product_logo_${size}.png" >/dev/null
done

for size in 16 32 64 128 256 512 1024; do
  sips -s format png -z "$size" "$size" "$source_png" \
    --out "$theme_root/mac/Assets.xcassets/AppIcon.appiconset/appicon_${size}.png" >/dev/null
done

echo "RealBrowser Chromium icons updated in $theme_root"
