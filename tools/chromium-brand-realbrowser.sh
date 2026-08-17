#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHROMIUM_ROOT="${1:-}"
BRANDING="$CHROMIUM_ROOT/chrome/app/theme/chromium/BRANDING"
STRINGS="$CHROMIUM_ROOT/chrome/app/chromium_strings.grd"

[[ "$CHROMIUM_ROOT" = /* ]] || { printf 'usage: %s /absolute/path/to/chromium/src\n' "$0" >&2; exit 2; }
[[ -f "$BRANDING" && -f "$STRINGS" ]] || { printf 'error: Chromium branding files not found\n' >&2; exit 1; }

perl -pi -e 's/^COMPANY_FULLNAME=.*/COMPANY_FULLNAME=RealBrowser/; s/^COMPANY_SHORTNAME=.*/COMPANY_SHORTNAME=RealBrowser/; s/^PRODUCT_FULLNAME=.*/PRODUCT_FULLNAME=RealBrowser/; s/^PRODUCT_SHORTNAME=.*/PRODUCT_SHORTNAME=RealBrowser/; s/^PRODUCT_INSTALLER_FULLNAME=.*/PRODUCT_INSTALLER_FULLNAME=RealBrowser Installer/; s/^PRODUCT_INSTALLER_SHORTNAME=.*/PRODUCT_INSTALLER_SHORTNAME=RealBrowser Installer/; s/^COPYRIGHT=.*/COPYRIGHT=Copyright \@LASTCHANGE_YEAR\@ RealBrowser. All rights reserved./; s/^MAC_BUNDLE_ID=.*/MAC_BUNDLE_ID=com.realbrowser.browser/; s/^MAC_CREATOR_CODE=.*/MAC_CREATOR_CODE=RBrw/' "$BRANDING"
# Translated strings can inflect the product name (for example "Chromiuma").
# Replace the product-name stem as well as the standalone English spelling so
# no locale falls back to presenting the upstream product name.
perl -pi -e 's/Chromium/RealBrowser/g' "$STRINGS"
find "$CHROMIUM_ROOT/chrome/app/resources" -type f \
  -name 'chromium_strings_*.xtb' -print0 \
  | xargs -0 perl -pi -e 's/Chromium/RealBrowser/g'

"$ROOT/tools/chromium-brand-icons.sh" "$CHROMIUM_ROOT"
printf 'RealBrowser Chromium name and icon resources updated\n'
