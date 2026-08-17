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
host="$(uname -s)"

test -f "$chromium_root/DEPS"
test -d "$theme_root"
test -f "$source_png"

resize_png() {
  local size="$1"
  local dest="$2"

  if [[ "$host" == "Darwin" ]]; then
    command -v sips >/dev/null || {
      echo "error: macOS icon branding requires sips" >&2
      exit 1
    }
    sips -s format png -z "$size" "$size" "$source_png" --out "$dest" >/dev/null
    return 0
  fi

  if command -v magick >/dev/null; then
    magick "$source_png" -filter Lanczos -resize "${size}x${size}!" "$dest"
    return 0
  fi

  if command -v convert >/dev/null; then
    convert "$source_png" -filter Lanczos -resize "${size}x${size}!" "$dest"
    return 0
  fi

  python3 - "$source_png" "$size" "$dest" <<'PY'
import sys

source, size_text, dest = sys.argv[1], sys.argv[2], sys.argv[3]
size = int(size_text)
try:
    from PIL import Image
except ImportError:
    sys.stderr.write(
        "error: Linux icon branding requires ImageMagick (magick/convert) or Python Pillow\n"
    )
    sys.exit(1)

image = Image.open(source).convert("RGBA")
image = image.resize((size, size), Image.Resampling.LANCZOS)
image.save(dest, format="PNG")
PY
}

if [[ "$host" == "Darwin" ]]; then
  cp "$repo_root/apps/desktop/src-tauri/icons/icon.icns" \
    "$theme_root/mac/app.icns"
fi
cp "$repo_root/apps/desktop/src-tauri/icons/icon.ico" \
  "$theme_root/win/chromium.ico"
cp "$repo_root/apps/desktop/src/assets/logo-brand.svg" \
  "$theme_root/product_logo.svg"

for size in 16 24 48 64 128 256; do
  resize_png "$size" "$theme_root/product_logo_${size}.png"
done

mkdir -p "$theme_root/linux"
for size in 24 48 64 128 256; do
  resize_png "$size" "$theme_root/linux/product_logo_${size}.png"
done

if [[ "$host" == "Darwin" ]]; then
  for size in 16 32 64 128 256 512 1024; do
    resize_png "$size" \
      "$theme_root/mac/Assets.xcassets/AppIcon.appiconset/appicon_${size}.png"
  done
fi

echo "RealBrowser Chromium icons updated in $theme_root"
