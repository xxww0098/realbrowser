package fingerprintkernel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// LaunchOptions collects everything required to compile an adryfish/fingerprint-chromium
// command-line invocation for a single browser profile.
type LaunchOptions struct {
	ProfileID           string
	UserDataDir         string
	RemoteDebuggingPort int    // 0 => --remote-debugging-port=0 (ephemeral port printed to stderr)
	ProxyServerOverride string // set when local ProxyBridge wraps an authenticated upstream proxy
	Fingerprint         FingerprintSpec
	Proxy               ProxySpec
	StartURLs           []string
	ExtraArgs           []string
}

// BuildArgs compiles LaunchOptions into validated adryfish/fingerprint-chromium
// command-line arguments.
func BuildArgs(opts LaunchOptions) ([]string, error) {
	if strings.TrimSpace(opts.UserDataDir) == "" {
		return nil, errors.New("fpchromium: user_data_dir is required")
	}
	if err := opts.Fingerprint.Validate(); err != nil {
		return nil, err
	}
	if err := opts.Proxy.Validate(); err != nil {
		return nil, err
	}
	if opts.RemoteDebuggingPort < 0 || opts.RemoteDebuggingPort > 65535 {
		return nil, fmt.Errorf("fpchromium: remote debugging port %d out of range", opts.RemoteDebuggingPort)
	}

	fp := opts.Fingerprint
	seed := fp.Seed
	if seed == 0 && opts.ProfileID != "" {
		seed = DeriveSeed(opts.ProfileID)
	}

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", opts.RemoteDebuggingPort),
		"--user-data-dir=" + opts.UserDataDir,
		"--no-first-run",
		"--no-default-browser-check",
	}
	if seed > 0 {
		args = append(args, fmt.Sprintf("--fingerprint=%d", seed))
	}
	if plat := NormalizePlatform(string(fp.Platform)); plat != "" {
		args = append(args, "--fingerprint-platform="+string(plat))
		if ver := strings.TrimSpace(fp.PlatformVersion); ver != "" {
			args = append(args, "--fingerprint-platform-version="+ver)
		}
	}
	if brand := NormalizeBrand(string(fp.Brand)); brand != "" && brand != BrandChromium {
		args = append(args, "--fingerprint-brand="+string(brand))
		if ver := strings.TrimSpace(fp.BrandVersion); ver != "" {
			args = append(args, "--fingerprint-brand-version="+ver)
		}
	}
	if fp.HardwareConcurrency > 0 {
		args = append(args, fmt.Sprintf("--fingerprint-hardware-concurrency=%d", fp.HardwareConcurrency))
	}
	if lang := strings.TrimSpace(fp.Lang); lang != "" {
		args = append(args, "--lang="+lang)
	}
	if al := strings.TrimSpace(fp.AcceptLang); al != "" {
		args = append(args, "--accept-lang="+al)
	}
	if tz := strings.TrimSpace(fp.Timezone); tz != "" {
		args = append(args, "--timezone="+tz)
	}
	if disabled := CanonicalSpoofModules(fp.DisableSpoofing); len(disabled) > 0 {
		names := make([]string, len(disabled))
		for i, m := range disabled {
			names[i] = string(m)
		}
		args = append(args, "--disable-spoofing="+strings.Join(names, ","))
	}

	// WebRTC IP handling: emit Chromium's CommandLinePrefStore switch
	// --webrtc-ip-handling-policy (verified in modern Chromium 144–158+) alongside
	// --force-webrtc-ip-handling-policy (legacy Chromium compat) and adryfish
	// --disable-non-proxied-udp.
	switch webrtcPol := fp.EffectiveWebRTCPolicy(); webrtcPol {
	case WebRTCDisableNonProxiedUDP:
		args = append(args,
			"--disable-non-proxied-udp",
			"--webrtc-ip-handling-policy=disable_non_proxied_udp",
			"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
			"--enforce-webrtc-ip-permission-check",
		)
	case WebRTCPublicInterfaceOnly:
		args = append(args,
			"--webrtc-ip-handling-policy=default_public_interface_only",
			"--force-webrtc-ip-handling-policy=default_public_interface_only",
			"--enforce-webrtc-ip-permission-check",
		)
	case WebRTCPublicAndPrivate:
		args = append(args,
			"--webrtc-ip-handling-policy=default_public_and_private_interfaces",
			"--force-webrtc-ip-handling-policy=default_public_and_private_interfaces",
		)
	}

	proxyServer := strings.TrimSpace(opts.ProxyServerOverride)
	if proxyServer == "" && !opts.Proxy.IsDirect() {
		proxyServer = opts.Proxy.ServerURL()
	}
	if proxyServer != "" {
		args = append(args, "--proxy-server="+proxyServer)
		var cleanBypass []string
		if len(opts.Proxy.BypassList) > 0 {
			cleanBypass = make([]string, 0, len(opts.Proxy.BypassList))
			for _, b := range opts.Proxy.BypassList {
				if t := strings.TrimSpace(b); t != "" {
					cleanBypass = append(cleanBypass, t)
				}
			}
			if len(cleanBypass) > 0 {
				args = append(args, "--proxy-bypass-list="+strings.Join(cleanBypass, ","))
			}
		}
		if fp.EffectiveDNSLeakMode() == DNSLeakAuto {
			if rules := buildHostResolverRules(proxyServer, cleanBypass); rules != "" {
				args = append(args, "--host-resolver-rules="+rules)
			}
		}
	}

	// Disable UDP QUIC / HTTP3 transport when proxied (default auto) or explicitly disabled
	// so traffic cannot bypass HTTP/SOCKS5 proxies over UDP.
	switch fp.EffectiveQUICMode() {
	case QUICDisable:
		args = append(args, "--disable-quic")
	case QUICAuto:
		if proxyServer != "" {
			args = append(args, "--disable-quic")
		}
	}

	switch fp.EffectiveGPUMode() {
	case GPUAngleMetal:
		args = append(args, "--use-angle=metal")
	case GPUAngleD3D11:
		args = append(args, "--use-angle=d3d11")
	case GPUAngleVulkan:
		args = append(args, "--use-angle=vulkan")
	case GPUAngleGL:
		args = append(args, "--use-angle=gl")
	case GPUSwiftShader:
		args = append(args, "--use-gl=angle", "--use-angle=swiftshader-webgl", "--enable-unsafe-swiftshader")
	case GPUDisable3D:
		args = append(args, "--disable-3d-apis")
	}

	switch fp.EffectiveColorScheme() {
	case ColorSchemeDark:
		args = append(args, "--force-dark-mode")
	case ColorSchemeLight:
		args = append(args, "--blink-settings=preferredColorScheme=1")
	}

	if fp.DisablePings {
		args = append(args, "--no-pings")
	}
	if fp.MuteAudio {
		args = append(args, "--mute-audio")
	}
	if fp.DisablePasswordPopup {
		args = append(args, "--password-store=basic")
	}

	if ua := strings.TrimSpace(fp.UserAgent); ua != "" {
		args = append(args, "--user-agent="+ua)
	}
	if fp.WindowWidth > 0 && fp.WindowHeight > 0 {
		args = append(args, fmt.Sprintf("--window-size=%d,%d", fp.WindowWidth, fp.WindowHeight))
	}
	if fp.WindowX != nil && fp.WindowY != nil {
		args = append(args, fmt.Sprintf("--window-position=%d,%d", *fp.WindowX, *fp.WindowY))
	}
	if fp.DeviceScaleFactor > 0 {
		args = append(args, "--force-device-scale-factor="+strconv.FormatFloat(fp.DeviceScaleFactor, 'f', -1, 64))
	}
	if fp.Headless {
		args = append(args, "--headless=new")
	}

	// Merge user extra args (later key overrides earlier key for --flag=val pairs).
	args = mergeExtraArgs(args, opts.ExtraArgs)

	// Positional startup URLs must follow switches.
	for _, u := range opts.StartURLs {
		if t := strings.TrimSpace(u); t != "" {
			args = append(args, t)
		}
	}
	return args, nil
}

// buildHostResolverRules constructs Chromium's --host-resolver-rules string to
// prevent local ISP DNS leaks when routing through a proxy server. All DNS lookups
// resolve to ~NOTFOUND locally except the proxy endpoint host(s), loopback, and
// explicit bypass entries.
func buildHostResolverRules(proxyServer string, bypassList []string) string {
	seen := make(map[string]bool, 4+len(bypassList))
	excludes := make([]string, 0, 4+len(bypassList))
	addExclude := func(raw string) {
		h := strings.TrimSpace(raw)
		if h == "" || strings.HasPrefix(h, "<") || strings.ContainsAny(h, ",; \t\r\n") {
			return
		}
		// Strip scheme or port if present in proxyServer / bypass entries.
		if strings.Contains(h, "://") {
			if u, err := url.Parse(h); err == nil && u.Hostname() != "" {
				h = u.Hostname()
			} else {
				// Fallback for placeholder ports such as http://127.0.0.1:<local-auth-bridge>.
				after := h[strings.Index(h, "://")+3:]
				if slash := strings.IndexByte(after, '/'); slash >= 0 {
					after = after[:slash]
				}
				if host, _, err := net.SplitHostPort(after); err == nil && host != "" {
					h = host
				} else {
					return
				}
			}
		} else if host, _, err := net.SplitHostPort(h); err == nil && host != "" {
			h = host
		}
		h = strings.Trim(h, "[]")
		// Chromium --proxy-bypass-list allows leading-dot suffix rules like ".corp.local";
		// MappedHostResolver EXCLUDE expects wildcard syntax "*.corp.local".
		if strings.HasPrefix(h, ".") && len(h) > 1 {
			h = "*" + h
		}
		if h == "" || strings.ContainsAny(h, "<>/") {
			return
		}
		key := strings.ToLower(h)
		if seen[key] {
			return
		}
		seen[key] = true
		excludes = append(excludes, h)
	}

	// Support both single-URL proxy servers (socks5://host:port) and Chromium
	// multi-scheme semicolon specs (http=proxy1:8080;https=proxy2:8443).
	for _, seg := range strings.Split(proxyServer, ";") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if !strings.Contains(seg, "://") {
			if eq := strings.IndexByte(seg, '='); eq > 0 {
				seg = strings.TrimSpace(seg[eq+1:])
			}
		}
		if u, err := url.Parse(seg); err == nil && u.Hostname() != "" {
			addExclude(u.Hostname())
		} else {
			addExclude(seg)
		}
	}
	addExclude("127.0.0.1")
	addExclude("localhost")
	addExclude("::1")
	for _, b := range bypassList {
		// Chromium --proxy-bypass-list entries may also contain semicolon-delimited items.
		for _, item := range strings.Split(b, ";") {
			addExclude(item)
		}
	}
	if len(excludes) == 0 {
		return ""
	}
	parts := make([]string, 0, 1+len(excludes))
	parts = append(parts, "MAP * ~NOTFOUND")
	for _, ex := range excludes {
		parts = append(parts, "EXCLUDE "+ex)
	}
	return strings.Join(parts, " , ")
}

// MergeExtraArgs appends extra flags, replacing any existing flag with the same
// --name prefix so user overrides take precedence cleanly.
func MergeExtraArgs(base, extra []string) []string {
	return mergeExtraArgs(base, extra)
}

// mergeExtraArgs appends extra flags, replacing any existing flag with the same
// --name prefix so user overrides take precedence cleanly, while union-merging
// comma-separated Chromium feature lists (such as --disable-blink-features) to keep configured features from both sources.
func mergeExtraArgs(base, extra []string) []string {
	out := append([]string(nil), base...)
	for i := 0; i < len(extra); i++ {
		arg := strings.TrimSpace(extra[i])
		if arg == "" {
			continue
		}
		key := flagKey(arg)
		if key == "" {
			out = append(out, arg)
			continue
		}
		// Normalize two-token comma-separated feature flags (e.g. ["--disable-blink-features", "Foo"])
		// into "--disable-blink-features=Foo" so the feature value merges cleanly instead of turning
		// into a stray positional URL argument.
		if isCommaFeatureListFlag(key) && !strings.Contains(arg, "=") && i+1 < len(extra) {
			next := strings.TrimSpace(extra[i+1])
			if next == "" || !strings.HasPrefix(next, "-") {
				arg = key + "=" + next
				i++
			}
		}
		replaced := false
		for j, existing := range out {
			if strings.EqualFold(flagKey(existing), key) {
				if isCommaFeatureListFlag(key) {
					out[j] = mergeCommaListFlag(existing, arg)
				} else {
					out[j] = arg
				}
				replaced = true
				break
			}
		}
		if !replaced {
			if isCommaFeatureListFlag(key) {
				merged := mergeCommaListFlag(key+"=", arg)
				if merged == key+"=" {
					continue
				}
				out = append(out, merged)
				continue
			}
			out = append(out, arg)
		}
	}
	return out
}

func isCommaFeatureListFlag(key string) bool {
	switch strings.ToLower(key) {
	case "--disable-blink-features", "--enable-blink-features", "--disable-features", "--enable-features":
		return true
	default:
		return false
	}
}

func mergeCommaListFlag(existing, incoming string) string {
	key := flagKey(existing)
	if key == "" {
		key = flagKey(incoming)
	}
	seen := map[string]bool{}
	tokens := make([]string, 0, 4)
	addFrom := func(arg string) {
		eq := strings.IndexByte(arg, '=')
		if eq < 0 || eq+1 >= len(arg) {
			return
		}
		for _, raw := range strings.Split(arg[eq+1:], ",") {
			tok := strings.TrimSpace(raw)
			if tok == "" {
				continue
			}
			lk := strings.ToLower(tok)
			if seen[lk] {
				continue
			}
			seen[lk] = true
			tokens = append(tokens, tok)
		}
	}
	addFrom(existing)
	addFrom(incoming)
	if len(tokens) == 0 {
		return existing
	}
	return key + "=" + strings.Join(tokens, ",")
}

func flagKey(arg string) string {
	if !strings.HasPrefix(arg, "--") {
		return ""
	}
	if i := strings.IndexByte(arg, '='); i > 2 {
		return arg[:i]
	}
	return arg
}

// FormatCommandPreview renders a shell-quoted preview of the launch command for UI display.
func FormatCommandPreview(binaryPath string, opts LaunchOptions) string {
	bin := strings.TrimSpace(binaryPath)
	if bin == "" {
		bin = "fingerprint-chromium"
	}
	if opts.UserDataDir == "" {
		id := opts.ProfileID
		if id == "" {
			id = "preview"
		}
		opts.UserDataDir = filepath.Join("<dataDir>", "profiles", id)
	}
	previewOpts := opts
	if previewOpts.Proxy.NeedsLocalBridge() && previewOpts.ProxyServerOverride == "" {
		scheme := "http"
		if previewOpts.Proxy.NormalizedMode() == "socks5" {
			scheme = "socks5"
		}
		previewOpts.ProxyServerOverride = scheme + "://127.0.0.1:<local-auth-bridge>"
	}
	args, err := BuildArgs(previewOpts)
	if err != nil {
		return fmt.Sprintf("# 配置校验失败: %v", err)
	}
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(bin))
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " \\\n  ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '.' || r == '/' || r == ':' || r == '=' || r == ',') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Prepare validates a launch, writes the native Chromium preferences and builds
// its command arguments and environment. A nil hostEnv inherits os.Environ().
func (o LaunchOptions) Prepare(ctx context.Context, hostEnv []string) ([]string, map[string]string, error) {
	args, err := BuildArgs(o)
	if err != nil {
		return nil, nil, err
	}
	proxied := strings.TrimSpace(o.ProxyServerOverride) != "" || !o.Proxy.IsDirect()
	if err := o.Fingerprint.WriteChromiumPreferences(ctx, o.UserDataDir, proxied); err != nil {
		return nil, nil, err
	}
	if hostEnv == nil {
		hostEnv = os.Environ()
	}
	return args, BuildProcessEnv(hostEnv, o), nil
}
