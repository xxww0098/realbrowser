// Package profile models, edits and persists browser fingerprints.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
)

// Platform is the operating system spoofed by --fingerprint-platform.
type Platform string

const (
	PlatformMacOS   Platform = "macos"
	PlatformWindows Platform = "windows"
	PlatformLinux   Platform = "linux"
)

// AllPlatforms lists supported --fingerprint-platform values in UI display order.
var AllPlatforms = []Platform{PlatformMacOS, PlatformWindows, PlatformLinux}

// Brand is the browser brand spoofed by --fingerprint-brand.
type Brand string

const (
	BrandChrome   Brand = "Chrome"
	BrandEdge     Brand = "Edge"
	BrandOpera    Brand = "Opera"
	BrandVivaldi  Brand = "Vivaldi"
	BrandChromium Brand = "Chromium" // default when --fingerprint-brand is omitted
)

// AllBrands lists supported --fingerprint-brand values in UI display order.
var AllBrands = []Brand{BrandChrome, BrandEdge, BrandOpera, BrandVivaldi, BrandChromium}

// SpoofModule names a granular spoofing vector controllable via Chrome 144+
// --disable-spoofing=<comma-separated>.
type SpoofModule string

const (
	SpoofFont        SpoofModule = "font"
	SpoofAudio       SpoofModule = "audio"
	SpoofCanvas      SpoofModule = "canvas"
	SpoofClientRects SpoofModule = "clientrects"
	SpoofGPU         SpoofModule = "gpu"
)

// AllSpoofModules lists all supported --disable-spoofing targets in canonical order.
var AllSpoofModules = []SpoofModule{
	SpoofAudio,
	SpoofCanvas,
	SpoofClientRects,
	SpoofFont,
	SpoofGPU,
}

// WebRTCPolicy maps directly to Chromium's real C++ switch
// --force-webrtc-ip-handling-policy and Preferences webrtc.ip_handling_policy.
type WebRTCPolicy string

const (
	WebRTCDisableNonProxiedUDP WebRTCPolicy = "disable_non_proxied_udp"
	WebRTCPublicInterfaceOnly  WebRTCPolicy = "default_public_interface_only"
	WebRTCPublicAndPrivate     WebRTCPolicy = "default_public_and_private_interfaces"
	WebRTCDefault              WebRTCPolicy = "default"
)

// AllWebRTCPolicies lists supported WebRTC IP handling policies from strictest to permissive.
var AllWebRTCPolicies = []WebRTCPolicy{
	WebRTCDisableNonProxiedUDP,
	WebRTCPublicInterfaceOnly,
	WebRTCPublicAndPrivate,
	WebRTCDefault,
}

// DNSLeakMode controls Chromium --host-resolver-rules + DNS prefetch suppression
// when outbound proxy routing is enabled.
type DNSLeakMode string

const (
	DNSLeakAuto   DNSLeakMode = "auto"   // default: block local DNS when proxy is active
	DNSLeakSystem DNSLeakMode = "system" // allow OS/local DNS resolver
)

// QUICMode controls Chromium's UDP QUIC / HTTP3 transport (--disable-quic),
// preventing UDP proxy bypass leaks when routing through HTTP/SOCKS5 proxies.
type QUICMode string

const (
	QUICAuto    QUICMode = "auto"    // default: emit --disable-quic when outbound proxy is active
	QUICDisable QUICMode = "disable" // always emit --disable-quic (even direct)
	QUICAllow   QUICMode = "allow"   // never emit --disable-quic (allow native HTTP/3 UDP)
)

// AutomationMode controls Blink's AutomationControlled runtime feature
// (--disable-blink-features=AutomationControlled).
type AutomationMode string

const (
	AutomationHide   AutomationMode = "hide"   // default: suppress navigator.webdriver / AutomationControlled
	AutomationNative AutomationMode = "native" // omit flag (native Chromium behavior)
)

// DoNotTrackMode controls Chromium Preferences enable_do_not_track (navigator.doNotTrack & DNT header).
type DoNotTrackMode string

const (
	DoNotTrackOff DoNotTrackMode = "off" // recommended realistic default
	DoNotTrackOn  DoNotTrackMode = "on"  // send DNT: 1
)

// GPUMode selects an explicit Chromium GPU / ANGLE backend or disables 3D APIs.
type GPUMode string

const (
	GPUAuto        GPUMode = "auto"        // default native GPU acceleration
	GPUAngleMetal  GPUMode = "angle_metal" // --use-angle=metal (macOS)
	GPUAngleD3D11  GPUMode = "angle_d3d11" // --use-angle=d3d11 (Windows)
	GPUAngleVulkan GPUMode = "angle_vulkan"
	GPUAngleGL     GPUMode = "angle_gl"
	GPUSwiftShader GPUMode = "swiftshader" // software rasterizer (detectable!)
	GPUDisable3D   GPUMode = "disable_3d"  // --disable-3d-apis (detectable!)
)

// AllGPUModes lists supported GPU backend choices in UI order.
var AllGPUModes = []GPUMode{
	GPUAuto,
	GPUAngleMetal,
	GPUAngleD3D11,
	GPUAngleVulkan,
	GPUAngleGL,
	GPUSwiftShader,
	GPUDisable3D,
}

// ColorSchemeMode controls preferred CSS color scheme (prefers-color-scheme).
type ColorSchemeMode string

const (
	ColorSchemeSystem ColorSchemeMode = "system"
	ColorSchemeLight  ColorSchemeMode = "light"
	ColorSchemeDark   ColorSchemeMode = "dark"
)

// PermissionMode controls default Chromium content settings (0=ask, 1=allow, 2=block).
type PermissionMode string

const (
	PermissionPrompt PermissionMode = "prompt"
	PermissionAllow  PermissionMode = "allow"
	PermissionBlock  PermissionMode = "block"
)

// GeolocationSpec configures geolocation content permission and optional native
// C++ coordinates override (applied post-launch via Runtime-free CDP when Mode=="allow").
type GeolocationSpec struct {
	Mode      PermissionMode `json:"mode,omitempty"` // "" | "prompt" | "allow" | "block"
	Latitude  *float64       `json:"latitude,omitempty"`
	Longitude *float64       `json:"longitude,omitempty"`
	Accuracy  float64        `json:"accuracy,omitempty"` // meters; <=0 defaults to 50
}

// EffectiveMode returns canonical geolocation permission mode.
func (g GeolocationSpec) EffectiveMode() PermissionMode {
	switch NormalizePermissionMode(string(g.Mode)) {
	case PermissionAllow:
		return PermissionAllow
	case PermissionBlock:
		return PermissionBlock
	default:
		return PermissionPrompt
	}
}

// HasCoordinates reports whether both latitude and longitude are explicitly set.
func (g GeolocationSpec) HasCoordinates() bool {
	return g.Latitude != nil && g.Longitude != nil
}

// EffectiveAccuracy returns accuracy in meters (defaults to 50m when unset).
func (g GeolocationSpec) EffectiveAccuracy() float64 {
	if g.Accuracy > 0 {
		return g.Accuracy
	}
	return 50
}

// PermissionsSpec controls leak-prone browser content permissions natively via Preferences.
type PermissionsSpec struct {
	Notifications PermissionMode `json:"notifications,omitempty"`  // ""|"prompt"|"allow"|"block"
	MediaDevices  PermissionMode `json:"media_devices,omitempty"`  // ""|"prompt"|"block" (blocks unauthorized mic/cam enum)
	Sensors       PermissionMode `json:"sensors,omitempty"`        // ""|"prompt"|"block" (blocks accelerometer/gyroscope)
	HardwareGuard PermissionMode `json:"hardware_guard,omitempty"` // ""|"prompt"|"block" (blocks WebUSB/WebHID/WebBluetooth/WebSerial via *_guard)
}

// ScreenSpec describes the virtual desktop display and default window geometry
// assigned to a profile. User-edited WindowWidth/WindowHeight override the
// default window size at launch while preserving the screen preset.
type ScreenSpec struct {
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	AvailTop     int     `json:"avail_top"`
	AvailBottom  int     `json:"avail_bottom"`
	DPR          float64 `json:"dpr"`
	WindowWidth  int     `json:"window_width"`
	WindowHeight int     `json:"window_height"`
	WindowX      *int    `json:"window_x,omitempty"`
	WindowY      *int    `json:"window_y,omitempty"`
}

// Validate checks ScreenSpec dimensions, scale factor, and optional window position.
func (s ScreenSpec) Validate() error {
	if s.Width < 1024 || s.Width > 7680 || s.Height < 720 || s.Height > 4320 {
		return fmt.Errorf("fpchromium: screen dimensions %dx%d out of range", s.Width, s.Height)
	}
	if s.AvailTop < 0 || s.AvailBottom < 0 || s.AvailTop+s.AvailBottom >= s.Height {
		return fmt.Errorf("fpchromium: invalid screen work area insets (%d, %d) for height %d", s.AvailTop, s.AvailBottom, s.Height)
	}
	switch s.DPR {
	case 1, 1.25, 1.5, 2:
	default:
		return fmt.Errorf("fpchromium: unsupported screen dpr %v", s.DPR)
	}
	if s.WindowWidth < 1024 || s.WindowWidth > s.Width {
		return fmt.Errorf("fpchromium: screen window_width %d out of range [1024, %d]", s.WindowWidth, s.Width)
	}
	maxWinH := s.Height - s.AvailTop - s.AvailBottom
	if s.WindowHeight < 600 || s.WindowHeight > maxWinH {
		return fmt.Errorf("fpchromium: screen window_height %d out of range [600, %d]", s.WindowHeight, maxWinH)
	}
	if (s.WindowX != nil) != (s.WindowY != nil) {
		return errors.New("fpchromium: screen window_x and window_y must both be set")
	}
	if s.WindowX != nil {
		maxX := s.Width - s.WindowWidth
		maxY := s.Height - s.AvailBottom - s.WindowHeight
		if *s.WindowX < 0 || *s.WindowX > maxX || *s.WindowY < s.AvailTop || *s.WindowY > maxY {
			return fmt.Errorf("fpchromium: screen window position (%d, %d) out of work area", *s.WindowX, *s.WindowY)
		}
	}
	return nil
}

// FingerprintSpec is the editable JSON representation of a browser profile.
type FingerprintSpec struct {
	Seed                 uint32          `json:"seed"`
	Platform             Platform        `json:"platform"`
	PlatformVersion      string          `json:"platform_version,omitempty"`
	Brand                Brand           `json:"brand"`
	BrandVersion         string          `json:"brand_version,omitempty"`
	HardwareConcurrency  int             `json:"hardware_concurrency,omitempty"`
	Screen               *ScreenSpec     `json:"screen,omitempty"`
	Lang                 string          `json:"lang,omitempty"`
	AcceptLang           string          `json:"accept_lang,omitempty"`
	Timezone             string          `json:"timezone,omitempty"`
	DisableSpoofing      []SpoofModule   `json:"disable_spoofing,omitempty"`
	DisableNonProxiedUDP bool            `json:"disable_non_proxied_udp"`
	WindowWidth          int             `json:"window_width,omitempty"`
	WindowHeight         int             `json:"window_height,omitempty"`
	WindowX              *int            `json:"window_x,omitempty"`
	WindowY              *int            `json:"window_y,omitempty"`
	DeviceScaleFactor    float64         `json:"device_scale_factor,omitempty"`
	Headless             bool            `json:"headless,omitempty"`
	UserAgent            string          `json:"user_agent,omitempty"`
	WebRTCPolicy         WebRTCPolicy    `json:"webrtc_policy,omitempty"`
	DNSLeakMode          DNSLeakMode     `json:"dns_leak_mode,omitempty"`
	QUICMode             QUICMode        `json:"quic_mode,omitempty"`
	AutomationMode       AutomationMode  `json:"automation_mode,omitempty"`
	DoNotTrack           DoNotTrackMode  `json:"do_not_track,omitempty"`
	GPUMode              GPUMode         `json:"gpu_mode,omitempty"`
	ColorScheme          ColorSchemeMode `json:"color_scheme,omitempty"`
	Geolocation          GeolocationSpec `json:"geolocation,omitempty"`
	Permissions          PermissionsSpec `json:"permissions,omitempty"`
	DisablePings         bool            `json:"disable_pings,omitempty"`
	MuteAudio            bool            `json:"mute_audio,omitempty"`
	DisablePasswordPopup bool            `json:"disable_password_popup,omitempty"`
}

// NormalizePlatform maps legacy/free-form OS strings to adryfish/fingerprint-chromium
// values ("windows", "macos", "linux"). Returns "" if unrecognized.
func NormalizePlatform(raw string) Platform {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case s == "":
		return ""
	case s == "macos" || s == "darwin" || s == "mac" || s == "macintel" || strings.Contains(s, "mac"):
		return PlatformMacOS
	case s == "windows" || s == "win" || s == "win32" || s == "win64" || strings.Contains(s, "win"):
		return PlatformWindows
	case s == "linux" || strings.Contains(s, "linux") || strings.Contains(s, "ubuntu") || strings.Contains(s, "x11"):
		return PlatformLinux
	default:
		return ""
	}
}

// NormalizeBrand maps free-form browser brand names to adryfish/fingerprint-chromium
// canonical values. Returns "" if unrecognized.
func NormalizeBrand(raw string) Brand {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "chrome", "google chrome":
		return BrandChrome
	case "edge", "msedge", "microsoft edge":
		return BrandEdge
	case "opera":
		return BrandOpera
	case "vivaldi":
		return BrandVivaldi
	case "chromium":
		return BrandChromium
	default:
		return ""
	}
}

// NormalizeWebRTCPolicy canonicalizes WebRTC policy strings (returns "" if invalid).
func NormalizeWebRTCPolicy(raw string) WebRTCPolicy {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "disable_non_proxied_udp", "proxy", "disabled", "strict":
		return WebRTCDisableNonProxiedUDP
	case "default_public_interface_only", "public_only", "forward":
		return WebRTCPublicInterfaceOnly
	case "default_public_and_private_interfaces", "public_and_private":
		return WebRTCPublicAndPrivate
	case "default", "real", "allow":
		return WebRTCDefault
	default:
		return ""
	}
}

// NormalizeDNSLeakMode canonicalizes DNS leak protection mode.
func NormalizeDNSLeakMode(raw string) DNSLeakMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto", "block", "proxy", "remote":
		return DNSLeakAuto
	case "system", "local", "off", "disabled":
		return DNSLeakSystem
	default:
		return ""
	}
}

// NormalizeQUICMode canonicalizes QUIC / HTTP3 UDP transport policy.
func NormalizeQUICMode(raw string) QUICMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto", "proxy":
		return QUICAuto
	case "disable", "disabled", "off", "block", "true", "1":
		return QUICDisable
	case "allow", "enable", "enabled", "on", "native", "false", "0":
		return QUICAllow
	default:
		return ""
	}
}

// NormalizeAutomationMode canonicalizes AutomationControlled suppression mode.
func NormalizeAutomationMode(raw string) AutomationMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "hide", "stealth", "disable", "on":
		return AutomationHide
	case "native", "off", "default":
		return AutomationNative
	default:
		return ""
	}
}

// NormalizeDoNotTrack canonicalizes Do Not Track preference mode.
func NormalizeDoNotTrack(raw string) DoNotTrackMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "off", "0", "false", "default":
		return DoNotTrackOff
	case "on", "1", "true":
		return DoNotTrackOn
	default:
		return ""
	}
}

// NormalizeGPUMode canonicalizes GPU backend selection.
func NormalizeGPUMode(raw string) GPUMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto", "default":
		return GPUAuto
	case "angle_metal", "metal":
		return GPUAngleMetal
	case "angle_d3d11", "d3d11":
		return GPUAngleD3D11
	case "angle_vulkan", "vulkan":
		return GPUAngleVulkan
	case "angle_gl", "gl", "opengl":
		return GPUAngleGL
	case "swiftshader", "software":
		return GPUSwiftShader
	case "disable_3d", "disabled", "off":
		return GPUDisable3D
	default:
		return ""
	}
}

// NormalizeColorScheme canonicalizes CSS prefers-color-scheme mode.
func NormalizeColorScheme(raw string) ColorSchemeMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "system", "auto", "default":
		return ColorSchemeSystem
	case "light":
		return ColorSchemeLight
	case "dark":
		return ColorSchemeDark
	default:
		return ""
	}
}

// NormalizePermissionMode canonicalizes content setting permission modes.
func NormalizePermissionMode(raw string) PermissionMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "prompt", "ask", "default", "0":
		return PermissionPrompt
	case "allow", "1":
		return PermissionAllow
	case "block", "deny", "forbid", "2":
		return PermissionBlock
	default:
		return ""
	}
}

func CanonicalSpoofModules(in []SpoofModule) []SpoofModule {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[SpoofModule]bool, len(in))
	out := make([]SpoofModule, 0, len(in))
	for _, m := range in {
		norm := SpoofModule(strings.ToLower(strings.TrimSpace(string(m))))
		if !isValidSpoofModule(norm) || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, norm)
	}
	slices.Sort(out)
	return out
}

func isValidSpoofModule(m SpoofModule) bool {
	return slices.Contains(AllSpoofModules, m)
}

// EffectiveWebRTCPolicy resolves the effective WebRTC IP handling policy,
// maintaining full backward compatibility with legacy DisableNonProxiedUDP.
func (f FingerprintSpec) EffectiveWebRTCPolicy() WebRTCPolicy {
	if p := NormalizeWebRTCPolicy(string(f.WebRTCPolicy)); p != "" {
		return p
	}
	if f.DisableNonProxiedUDP {
		return WebRTCDisableNonProxiedUDP
	}
	return WebRTCDefault
}

// Validate checks FingerprintSpec fields for invalid values.
func (f FingerprintSpec) Validate() error {
	if f.Platform != "" && NormalizePlatform(string(f.Platform)) == "" {
		return fmt.Errorf("fpchromium: unsupported platform %q (want windows|macos|linux)", f.Platform)
	}
	if f.Brand != "" && NormalizeBrand(string(f.Brand)) == "" {
		return fmt.Errorf("fpchromium: unsupported brand %q (want Chrome|Edge|Opera|Vivaldi|Chromium)", f.Brand)
	}
	if f.HardwareConcurrency < 0 || f.HardwareConcurrency > 128 {
		return fmt.Errorf("fpchromium: hardware_concurrency %d out of range [0, 128]", f.HardwareConcurrency)
	}
	if f.Screen != nil {
		if err := f.Screen.Validate(); err != nil {
			return err
		}
	}
	if f.WindowWidth < 0 || f.WindowHeight < 0 {
		return errors.New("fpchromium: window dimensions must be non-negative")
	}
	if math.IsNaN(f.DeviceScaleFactor) || math.IsInf(f.DeviceScaleFactor, 0) ||
		(f.DeviceScaleFactor != 0 && (f.DeviceScaleFactor < 0.5 || f.DeviceScaleFactor > 4.0)) {
		return fmt.Errorf("fpchromium: device_scale_factor %v out of range [0.5, 4.0]", f.DeviceScaleFactor)
	}
	for _, m := range f.DisableSpoofing {
		if !isValidSpoofModule(SpoofModule(strings.ToLower(strings.TrimSpace(string(m))))) {
			return fmt.Errorf("fpchromium: unknown disable-spoofing target %q", m)
		}
	}
	if f.WebRTCPolicy != "" && NormalizeWebRTCPolicy(string(f.WebRTCPolicy)) == "" {
		return fmt.Errorf("fpchromium: unsupported webrtc_policy %q", f.WebRTCPolicy)
	}
	if f.DNSLeakMode != "" && NormalizeDNSLeakMode(string(f.DNSLeakMode)) == "" {
		return fmt.Errorf("fpchromium: unsupported dns_leak_mode %q", f.DNSLeakMode)
	}
	if f.QUICMode != "" && NormalizeQUICMode(string(f.QUICMode)) == "" {
		return fmt.Errorf("fpchromium: unsupported quic_mode %q", f.QUICMode)
	}
	if f.AutomationMode != "" && NormalizeAutomationMode(string(f.AutomationMode)) == "" {
		return fmt.Errorf("fpchromium: unsupported automation_mode %q", f.AutomationMode)
	}
	if f.DoNotTrack != "" && NormalizeDoNotTrack(string(f.DoNotTrack)) == "" {
		return fmt.Errorf("fpchromium: unsupported do_not_track %q", f.DoNotTrack)
	}
	if f.GPUMode != "" && NormalizeGPUMode(string(f.GPUMode)) == "" {
		return fmt.Errorf("fpchromium: unsupported gpu_mode %q", f.GPUMode)
	}
	if f.ColorScheme != "" && NormalizeColorScheme(string(f.ColorScheme)) == "" {
		return fmt.Errorf("fpchromium: unsupported color_scheme %q", f.ColorScheme)
	}
	if f.Geolocation.Mode != "" && NormalizePermissionMode(string(f.Geolocation.Mode)) == "" {
		return fmt.Errorf("fpchromium: unsupported geolocation.mode %q", f.Geolocation.Mode)
	}
	if (f.Geolocation.Latitude != nil) != (f.Geolocation.Longitude != nil) {
		return errors.New("fpchromium: geolocation latitude and longitude must both be set")
	}
	if f.Geolocation.Latitude != nil {
		lat := *f.Geolocation.Latitude
		lon := *f.Geolocation.Longitude
		if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 {
			return fmt.Errorf("fpchromium: geolocation latitude %v out of range [-90, 90]", lat)
		}
		if math.IsNaN(lon) || math.IsInf(lon, 0) || lon < -180 || lon > 180 {
			return fmt.Errorf("fpchromium: geolocation longitude %v out of range [-180, 180]", lon)
		}
	}
	if math.IsNaN(f.Geolocation.Accuracy) || math.IsInf(f.Geolocation.Accuracy, 0) || f.Geolocation.Accuracy < 0 {
		return fmt.Errorf("fpchromium: geolocation accuracy %v must be non-negative", f.Geolocation.Accuracy)
	}
	if f.Permissions.Notifications != "" && NormalizePermissionMode(string(f.Permissions.Notifications)) == "" {
		return fmt.Errorf("fpchromium: unsupported permissions.notifications %q", f.Permissions.Notifications)
	}
	if f.Permissions.MediaDevices != "" && NormalizePermissionMode(string(f.Permissions.MediaDevices)) == "" {
		return fmt.Errorf("fpchromium: unsupported permissions.media_devices %q", f.Permissions.MediaDevices)
	}
	if f.Permissions.Sensors != "" && NormalizePermissionMode(string(f.Permissions.Sensors)) == "" {
		return fmt.Errorf("fpchromium: unsupported permissions.sensors %q", f.Permissions.Sensors)
	}
	if f.Permissions.HardwareGuard != "" {
		hw := NormalizePermissionMode(string(f.Permissions.HardwareGuard))
		if hw != PermissionPrompt && hw != PermissionBlock {
			return fmt.Errorf("fpchromium: unsupported permissions.hardware_guard %q (want prompt|block)", f.Permissions.HardwareGuard)
		}
	}
	return nil
}

// ExportFingerprintJSON serializes f as pretty-printed JSON for backup/sharing.
func ExportFingerprintJSON(f FingerprintSpec) (string, error) {
	if err := f.Validate(); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ImportFingerprintJSON parses and validates a JSON-encoded FingerprintSpec.
func ImportFingerprintJSON(raw string) (FingerprintSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return FingerprintSpec{}, errors.New("fpchromium: 指纹 JSON 不能为空")
	}
	var f FingerprintSpec
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		return FingerprintSpec{}, fmt.Errorf("fpchromium: 解析指纹 JSON 失败: %w", err)
	}
	if f.Platform != "" {
		if p := NormalizePlatform(string(f.Platform)); p != "" {
			f.Platform = p
		}
	}
	if f.Brand != "" {
		if b := NormalizeBrand(string(f.Brand)); b != "" {
			f.Brand = b
		}
	}
	f.DisableSpoofing = CanonicalSpoofModules(f.DisableSpoofing)
	if f.WebRTCPolicy != "" {
		f.DisableNonProxiedUDP = f.EffectiveWebRTCPolicy() == WebRTCDisableNonProxiedUDP
	}
	if err := f.Validate(); err != nil {
		return FingerprintSpec{}, err
	}
	return f, nil
}
