package fingerprintkernel

import (
	"strings"
)

// strippedHostEnvKeys are host environment variables that must NEVER leak into
// spawned fingerprint-chromium browser processes:
//   - host proxy variables would hijack direct profiles or conflict with per-profile proxy routing;
//   - automation/Google API keys/Electron vars leak host identity or trigger automation paths.
var strippedHostEnvKeys = map[string]bool{
	"AUTH":                             true,
	"ELECTRON_RUN_AS_NODE":             true,
	"HTTP_PROXY":                       true,
	"HTTPS_PROXY":                      true,
	"ALL_PROXY":                        true,
	"NO_PROXY":                         true,
	"FTP_PROXY":                        true,
	"SOCKS_PROXY":                      true,
	"SOCKS5_PROXY":                     true,
	"CGI_HTTP_PROXY":                   true,
	"CHROME_HEADLESS":                  true,
	"PUPPETEER_EXECUTABLE_PATH":        true,
	"PUPPETEER_CACHE_DIR":              true,
	"PLAYWRIGHT_BROWSERS_PATH":         true,
	"PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD": true,
	"SELENIUM_MANAGER_PATH":            true,
	"CHROMEDRIVER_PATH":                true,
	"GOOGLE_API_KEY":                   true,
	"GOOGLE_DEFAULT_CLIENT_ID":         true,
	"GOOGLE_DEFAULT_CLIENT_SECRET":     true,
	"CHROME_LOG_FILE":                  true,
	// Legacy non-prefixed feature switches (must never leak into fingerprint-chromium).
	"CLOSE_KEEP_COOKIES":       true,
	"IS_CHECK_PROXY_BEFORE":    true,
	"IS_BROWSER_RESIDUE":       true,
	"IS_REALTIME_TZ_GEO":       true,
	"IS_DISABLE_CONSOLE":       true,
	"IS_DISABLE_SAVE_PWD":      true,
	"IS_CLIENT_RECT":           true,
	"IS_BROWSER_DEVICE_NAME":   true,
	"IS_BROWSER_PASSWORD_LIST": true,
	"IS_BROWSER_WHITE_LIST":    true,
	"IS_CLIENT_HINTS":          true,
	"IS_MAC_ADDRESS":           true,
	"IS_MULTI_KERNEL":          true,
	"IS_COLIST":                true,
	"CUSTOM_COMPONENTS":        true,
	"IS_SHARP":                 true,
	"EXTENSION_UPDATE":         true,
	"SYS_PROXY":                true,
	"IS_BROWSER_CREATE_ICON":   true,
}

// BuildProcessEnv builds a clean, complete OS environment map for launching
// fingerprint-chromium from hostEnv (typically os.Environ()):
//   - preserves essential OS runtime variables (PATH, HOME, USER, DISPLAY, XDG_*, SYSTEMROOT, etc.);
//   - strips host proxy, auth, telemetry, and automation leak variables (case-insensitive);
//   - aligns TZ with Fingerprint.Timezone so OS/libc timezone APIs agree with --timezone;
//   - aligns LANG and LC_TIME with Fingerprint.Lang so OS locale does not leak host language.
func BuildProcessEnv(hostEnv []string, opts LaunchOptions) map[string]string {
	out := make(map[string]string, len(hostEnv)+4)
	for _, entry := range hostEnv {
		eq := strings.IndexByte(entry, '=')
		if eq <= 0 {
			continue
		}
		k := entry[:eq]
		v := entry[eq+1:]
		uk := strings.ToUpper(k)
		if strippedHostEnvKeys[uk] || strings.HasPrefix(uk, "IS_SUNFLOWER_BROWSER") {
			continue
		}
		out[k] = v
	}

	deleteFold := func(target string) {
		for k := range out {
			if strings.EqualFold(k, target) {
				delete(out, k)
			}
		}
	}

	if tz := strings.TrimSpace(opts.Fingerprint.Timezone); tz != "" {
		deleteFold("TZ")
		out["TZ"] = tz
	}
	if locale := ToPOSIXLocale(opts.Fingerprint.Lang); locale != "" {
		deleteFold("LANG")
		deleteFold("LC_TIME")
		deleteFold("LC_MESSAGES")
		// Clear host LC_* override categories so host locale never bleeds past aligned LANG/LC_TIME.
		deleteFold("LC_ALL")
		deleteFold("LC_CTYPE")
		deleteFold("LANGUAGE")
		deleteFold("LC_NUMERIC")
		deleteFold("LC_MONETARY")
		deleteFold("LC_COLLATE")
		out["LANG"] = locale
		out["LC_TIME"] = locale
		out["LC_MESSAGES"] = locale
	}
	return out
}

// ToPOSIXLocale converts a BCP-47 language tag (e.g. "en-US", "zh-CN", "ja-JP")
// into a UTF-8 POSIX locale string (e.g. "en_US.UTF-8"). Returns "" if empty or invalid.
func ToPOSIXLocale(bcp47 string) string {
	s := strings.TrimSpace(bcp47)
	if s == "" {
		return ""
	}
	// Strip any encoding suffix if already present.
	if i := strings.IndexByte(s, '.'); i > 0 {
		s = s[:i]
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' })
	if len(parts) == 0 {
		return ""
	}
	lang := strings.ToLower(parts[0])
	if len(lang) < 2 || len(lang) > 3 {
		return ""
	}
	if len(parts) >= 2 && len(parts[1]) == 2 {
		return lang + "_" + strings.ToUpper(parts[1]) + ".UTF-8"
	}
	// Expand single-subtag common languages to standard POSIX locale.
	defaults := map[string]string{
		"en": "en_US",
		"zh": "zh_CN",
		"ja": "ja_JP",
		"ko": "ko_KR",
		"de": "de_DE",
		"fr": "fr_FR",
		"es": "es_ES",
		"it": "it_IT",
		"nl": "nl_NL",
		"pt": "pt_BR",
		"ru": "ru_RU",
		"pl": "pl_PL",
		"th": "th_TH",
	}
	if full, ok := defaults[lang]; ok {
		return full + ".UTF-8"
	}
	return lang + ".UTF-8"
}
