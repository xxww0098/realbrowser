package fingerprintkernel

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Common choices for fingerprint editors.
var (
	CommonTimezones = []string{
		"America/New_York",
		"America/Los_Angeles",
		"America/Chicago",
		"America/Denver",
		"America/Toronto",
		"America/Sao_Paulo",
		"Europe/London",
		"Europe/Berlin",
		"Europe/Paris",
		"Europe/Amsterdam",
		"Europe/Madrid",
		"Europe/Zurich",
		"Europe/Warsaw",
		"Asia/Shanghai",
		"Asia/Hong_Kong",
		"Asia/Taipei",
		"Asia/Tokyo",
		"Asia/Seoul",
		"Asia/Singapore",
		"Asia/Bangkok",
		"Asia/Dubai",
		"Asia/Kolkata",
		"Australia/Sydney",
		"UTC",
	}
	CommonLanguages = []string{
		"en-US",
		"en-GB",
		"en-CA",
		"en-AU",
		"zh-CN",
		"zh-TW",
		"zh-HK",
		"ja-JP",
		"ko-KR",
		"de-DE",
		"fr-FR",
		"es-ES",
		"it-IT",
		"nl-NL",
		"pt-BR",
		"ru-RU",
	}
	CommonResolutions = []struct {
		Label         string
		Width, Height int
	}{
		{"1280 × 800", 1280, 800},
		{"1366 × 768", 1366, 768},
		{"1440 × 900", 1440, 900},
		{"1512 × 982", 1512, 982},
		{"1680 × 1050", 1680, 1050},
		{"1920 × 1080", 1920, 1080},
		{"2560 × 1440", 2560, 1440},
	}
	CommonConcurrency  = []int{0, 2, 4, 6, 8, 12, 16, 24, 32}
	CommonScaleFactors = []float64{0, 1, 1.25, 1.5, 2, 3}
)

// FingerprintPreset is a curated one-click profile template combining coherent
// platform, OS version, brand, hardware, display, timezone, and locale settings.
type FingerprintPreset struct {
	ID                  string
	Label               string
	Platform            Platform
	PlatformVersion     string
	Brand               Brand
	HardwareConcurrency int
	WindowWidth         int
	WindowHeight        int
	DeviceScaleFactor   float64
	Timezone            string
	Lang                string
}

// Fingerprint creates a fresh editable profile from this preset.
func (pr FingerprintPreset) Fingerprint() FingerprintSpec {
	return FingerprintSpec{
		Seed:                 RandomSeed(),
		Platform:             pr.Platform,
		PlatformVersion:      pr.PlatformVersion,
		Brand:                pr.Brand,
		HardwareConcurrency:  pr.HardwareConcurrency,
		WindowWidth:          pr.WindowWidth,
		WindowHeight:         pr.WindowHeight,
		DeviceScaleFactor:    pr.DeviceScaleFactor,
		Timezone:             pr.Timezone,
		Lang:                 pr.Lang,
		AcceptLang:           DefaultAcceptLang(pr.Lang),
		DisableSpoofing:      []SpoofModule{SpoofCanvas},
		DisableNonProxiedUDP: true,
		WebRTCPolicy:         WebRTCDisableNonProxiedUDP,
		DNSLeakMode:          DNSLeakAuto,
		QUICMode:             QUICAuto,
		AutomationMode:       AutomationHide,
		DoNotTrack:           DoNotTrackOff,
		GPUMode:              GPUAuto,
		ColorScheme:          ColorSchemeSystem,
		Permissions:          PermissionsSpec{HardwareGuard: PermissionBlock},
	}
}

// RegionProfile captures a geographically coherent tuple of country, timezone,
// locale, Accept-Language, and representative coordinates for geo-harmony alignment.
type RegionProfile struct {
	Country    string
	Label      string
	Timezone   string
	Lang       string
	AcceptLang string
	Latitude   float64
	Longitude  float64
	UTCOffsetH float64 // approximate standard UTC offset in hours for geo sanity checks
}

// CommonRegions lists coherent geo profiles keyed by major e-commerce / social regions.
var CommonRegions = []RegionProfile{
	{Country: "US", Label: "美国 · 纽约 (美东)", Timezone: "America/New_York", Lang: "en-US", AcceptLang: "en-US,en", Latitude: 40.7128, Longitude: -74.0060, UTCOffsetH: -5},
	{Country: "US", Label: "美国 · 洛杉矶 (美西)", Timezone: "America/Los_Angeles", Lang: "en-US", AcceptLang: "en-US,en", Latitude: 34.0522, Longitude: -118.2437, UTCOffsetH: -8},
	{Country: "US", Label: "美国 · 芝加哥 (美中)", Timezone: "America/Chicago", Lang: "en-US", AcceptLang: "en-US,en", Latitude: 41.8781, Longitude: -87.6298, UTCOffsetH: -6},
	{Country: "US", Label: "美国 · 丹佛", Timezone: "America/Denver", Lang: "en-US", AcceptLang: "en-US,en", Latitude: 39.7392, Longitude: -104.9903, UTCOffsetH: -7},
	{Country: "CA", Label: "加拿大 · 多伦多", Timezone: "America/Toronto", Lang: "en-CA", AcceptLang: "en-CA,en", Latitude: 43.6532, Longitude: -79.3832, UTCOffsetH: -5},
	{Country: "BR", Label: "巴西 · 圣保罗", Timezone: "America/Sao_Paulo", Lang: "pt-BR", AcceptLang: "pt-BR,pt,en-US,en", Latitude: -23.5505, Longitude: -46.6333, UTCOffsetH: -3},
	{Country: "GB", Label: "英国 · 伦敦", Timezone: "Europe/London", Lang: "en-GB", AcceptLang: "en-GB,en", Latitude: 51.5074, Longitude: -0.1278, UTCOffsetH: 0},
	{Country: "DE", Label: "德国 · 法兰克福", Timezone: "Europe/Berlin", Lang: "de-DE", AcceptLang: "de-DE,de,en-US,en", Latitude: 50.1109, Longitude: 8.6821, UTCOffsetH: 1},
	{Country: "FR", Label: "法国 · 巴黎", Timezone: "Europe/Paris", Lang: "fr-FR", AcceptLang: "fr-FR,fr,en-US,en", Latitude: 48.8566, Longitude: 2.3522, UTCOffsetH: 1},
	{Country: "NL", Label: "荷兰 · 阿姆斯特丹", Timezone: "Europe/Amsterdam", Lang: "nl-NL", AcceptLang: "nl-NL,nl,en-US,en", Latitude: 52.3676, Longitude: 4.9041, UTCOffsetH: 1},
	{Country: "ES", Label: "西班牙 · 马德里", Timezone: "Europe/Madrid", Lang: "es-ES", AcceptLang: "es-ES,es,en-US,en", Latitude: 40.4168, Longitude: -3.7038, UTCOffsetH: 1},
	{Country: "CH", Label: "瑞士 · 苏黎世", Timezone: "Europe/Zurich", Lang: "de-CH", AcceptLang: "de-CH,de,en-US,en", Latitude: 47.3769, Longitude: 8.5417, UTCOffsetH: 1},
	{Country: "PL", Label: "波兰 · 华沙", Timezone: "Europe/Warsaw", Lang: "pl-PL", AcceptLang: "pl-PL,pl,en-US,en", Latitude: 52.2297, Longitude: 21.0122, UTCOffsetH: 1},
	{Country: "IT", Label: "意大利 · 米兰", Timezone: "Europe/Rome", Lang: "it-IT", AcceptLang: "it-IT,it,en-US,en", Latitude: 45.4642, Longitude: 9.1900, UTCOffsetH: 1},
	{Country: "CN", Label: "中国 · 上海", Timezone: "Asia/Shanghai", Lang: "zh-CN", AcceptLang: "zh-CN,zh,en-US,en", Latitude: 31.2304, Longitude: 121.4737, UTCOffsetH: 8},
	{Country: "HK", Label: "中国香港", Timezone: "Asia/Hong_Kong", Lang: "zh-HK", AcceptLang: "zh-HK,zh,en-US,en", Latitude: 22.3193, Longitude: 114.1694, UTCOffsetH: 8},
	{Country: "TW", Label: "中国台湾 · 台北", Timezone: "Asia/Taipei", Lang: "zh-TW", AcceptLang: "zh-TW,zh,en-US,en", Latitude: 25.0330, Longitude: 121.5654, UTCOffsetH: 8},
	{Country: "JP", Label: "日本 · 东京", Timezone: "Asia/Tokyo", Lang: "ja-JP", AcceptLang: "ja-JP,ja,en-US,en", Latitude: 35.6762, Longitude: 139.6503, UTCOffsetH: 9},
	{Country: "KR", Label: "韩国 · 首尔", Timezone: "Asia/Seoul", Lang: "ko-KR", AcceptLang: "ko-KR,ko,en-US,en", Latitude: 37.5665, Longitude: 126.9780, UTCOffsetH: 9},
	{Country: "SG", Label: "新加坡", Timezone: "Asia/Singapore", Lang: "en-SG", AcceptLang: "en-SG,en", Latitude: 1.3521, Longitude: 103.8198, UTCOffsetH: 8},
	{Country: "TH", Label: "泰国 · 曼谷", Timezone: "Asia/Bangkok", Lang: "th-TH", AcceptLang: "th-TH,th,en-US,en", Latitude: 13.7563, Longitude: 100.5018, UTCOffsetH: 7},
	{Country: "AE", Label: "阿联酋 · 迪拜", Timezone: "Asia/Dubai", Lang: "en-AE", AcceptLang: "en-AE,en,ar", Latitude: 25.2048, Longitude: 55.2708, UTCOffsetH: 4},
	{Country: "IN", Label: "印度 · 孟买", Timezone: "Asia/Kolkata", Lang: "en-IN", AcceptLang: "en-IN,en", Latitude: 19.0760, Longitude: 72.8777, UTCOffsetH: 5.5},
	{Country: "AU", Label: "澳大利亚 · 悉尼", Timezone: "Australia/Sydney", Lang: "en-AU", AcceptLang: "en-AU,en", Latitude: -33.8688, Longitude: 151.2093, UTCOffsetH: 10},
}

// RegionForTimezone returns the matching RegionProfile for an IANA timezone.
func RegionForTimezone(tz string) (RegionProfile, bool) {
	t := strings.TrimSpace(tz)
	if t == "" {
		return RegionProfile{}, false
	}
	for _, r := range CommonRegions {
		if strings.EqualFold(r.Timezone, t) {
			return r, true
		}
	}
	return RegionProfile{}, false
}

// RegionForCountry resolves an ISO-2 country code (or common name like "UK", "United States")
// to a default coherent RegionProfile.
func RegionForCountry(country string) (RegionProfile, bool) {
	c := strings.ToUpper(strings.TrimSpace(country))
	if c == "" {
		return RegionProfile{}, false
	}
	aliases := map[string]string{
		"UK": "GB", "UNITED KINGDOM": "GB", "BRITAIN": "GB",
		"USA": "US", "UNITED STATES": "US",
		"GERMANY": "DE", "FRANCE": "FR", "JAPAN": "JP", "SOUTH KOREA": "KR", "KOREA": "KR",
		"SINGAPORE": "SG", "CHINA": "CN", "HONG KONG": "HK", "TAIWAN": "TW",
		"CANADA": "CA", "AUSTRALIA": "AU", "NETHERLANDS": "NL", "SPAIN": "ES",
		"SWITZERLAND": "CH", "POLAND": "PL", "ITALY": "IT", "BRAZIL": "BR",
		"THAILAND": "TH", "UAE": "AE", "INDIA": "IN",
	}
	if mapped, ok := aliases[c]; ok {
		c = mapped
	}
	for _, r := range CommonRegions {
		if r.Country == c {
			return r, true
		}
	}
	return RegionProfile{}, false
}

// PlatformVersionOption represents a realistic Sec-CH-UA-Platform-Version preset.
type PlatformVersionOption struct {
	Version string
	Label   string
}

// PlatformVersionOptions returns realistic OS version presets for platform p.
func PlatformVersionOptions(p Platform) []PlatformVersionOption {
	switch NormalizePlatform(string(p)) {
	case PlatformWindows:
		return []PlatformVersionOption{
			{Version: "15.0.0", Label: "Win11 23H2/24H2 (15.0.0)"},
			{Version: "14.0.0", Label: "Win11 22H2 (14.0.0)"},
			{Version: "10.0.0", Label: "Win10 22H2 (10.0.0)"},
		}
	case PlatformLinux:
		return []PlatformVersionOption{
			{Version: "6.11.0", Label: "Linux 6.11"},
			{Version: "6.8.0", Label: "Linux 6.8 LTS"},
			{Version: "6.6.0", Label: "Linux 6.6 LTS"},
		}
	default:
		return []PlatformVersionOption{
			{Version: "15.3.0", Label: "macOS 15.3 Sequoia"},
			{Version: "15.2.0", Label: "macOS 15.2 Sequoia"},
			{Version: "14.7.0", Label: "macOS 14.7 Sonoma"},
		}
	}
}

// CommonFingerprintPresets lists ready-to-apply realistic desktop templates.
// All presets are verified cross-signal consistent (OS version, DPR, resolution,
// CPU cores, timezone, and locale) and score cleanly in Audit.
var CommonFingerprintPresets = []FingerprintPreset{
	{
		ID:                  "mac-us-chrome",
		Label:               "macOS Chrome · 美东",
		Platform:            PlatformMacOS,
		PlatformVersion:     "15.2.0",
		Brand:               BrandChrome,
		HardwareConcurrency: 8,
		WindowWidth:         1512,
		WindowHeight:        982,
		DeviceScaleFactor:   2,
		Timezone:            "America/New_York",
		Lang:                "en-US",
	},
	{
		ID:                  "win-us-chrome",
		Label:               "Win11 Chrome · 美西",
		Platform:            PlatformWindows,
		PlatformVersion:     "15.0.0",
		Brand:               BrandChrome,
		HardwareConcurrency: 8,
		WindowWidth:         1920,
		WindowHeight:        1080,
		DeviceScaleFactor:   1.25,
		Timezone:            "America/Los_Angeles",
		Lang:                "en-US",
	},
	{
		ID:                  "win-uk-edge",
		Label:               "Win11 Edge · 伦敦",
		Platform:            PlatformWindows,
		PlatformVersion:     "15.0.0",
		Brand:               BrandEdge,
		HardwareConcurrency: 12,
		WindowWidth:         1920,
		WindowHeight:        1080,
		DeviceScaleFactor:   1,
		Timezone:            "Europe/London",
		Lang:                "en-GB",
	},
	{
		ID:                  "mac-jp-chrome",
		Label:               "macOS Chrome · 东京",
		Platform:            PlatformMacOS,
		PlatformVersion:     "15.2.0",
		Brand:               BrandChrome,
		HardwareConcurrency: 12,
		WindowWidth:         1440,
		WindowHeight:        900,
		DeviceScaleFactor:   2,
		Timezone:            "Asia/Tokyo",
		Lang:                "ja-JP",
	},
	{
		ID:                  "win-sg-chrome",
		Label:               "Win11 Chrome · 新加坡",
		Platform:            PlatformWindows,
		PlatformVersion:     "15.0.0",
		Brand:               BrandChrome,
		HardwareConcurrency: 8,
		WindowWidth:         1920,
		WindowHeight:        1080,
		DeviceScaleFactor:   1.25,
		Timezone:            "Asia/Singapore",
		Lang:                "en-SG",
	},
	{
		ID:                  "linux-de-chrome",
		Label:               "Linux Chrome · 法兰克福",
		Platform:            PlatformLinux,
		PlatformVersion:     "6.8.0",
		Brand:               BrandChrome,
		HardwareConcurrency: 8,
		WindowWidth:         1440,
		WindowHeight:        900,
		DeviceScaleFactor:   1,
		Timezone:            "Europe/Berlin",
		Lang:                "de-DE",
	},
}

// DefaultAcceptLang builds a realistic Accept-Language value for a primary locale.
func DefaultAcceptLang(lang string) string {
	l := strings.TrimSpace(lang)
	if l == "" {
		return ""
	}
	base := l
	if i := strings.IndexAny(l, "-_"); i > 0 {
		base = l[:i]
	}
	if strings.EqualFold(base, "en") {
		if strings.EqualFold(l, "en") {
			return "en"
		}
		return l + ",en"
	}
	if strings.EqualFold(l, base) {
		return l + ",en-US,en"
	}
	return l + "," + base + ",en-US,en"
}

// FormatScaleFactor formats a DPI scale factor cleanly ("跟随系统" for <=0).
func FormatScaleFactor(v float64) string {
	if v <= 0 {
		return "跟随系统"
	}
	return strconv.FormatFloat(v, 'f', -1, 64) + "x"
}

// DerivedHardwareConcurrency returns the exact navigator.hardwareConcurrency value
// computed by adryfish/fingerprint-chromium (005-hardware-concurrency-fingerprint.patch)
// when --fingerprint-hardware-concurrency is omitted.
func DerivedHardwareConcurrency(seed uint32) int {
	if seed == 0 {
		return 8
	}
	return int(((seed % 13) + 4) * 2)
}

// DerivedDeviceMemoryGB returns the exact navigator.deviceMemory GB value
// computed by adryfish/fingerprint-chromium Chrome 148+ (021-device-memory-cap.patch).
func DerivedDeviceMemoryGB(seed uint32) int {
	vals := [3]int{8, 16, 32}
	return vals[seed%3]
}

// DefaultFingerprint returns a well-formed FingerprintSpec with a random seed.
func DefaultFingerprint(platform Platform, brand Brand) FingerprintSpec {
	p := NormalizePlatform(string(platform))
	if p == "" {
		p = PlatformMacOS
	}
	b := NormalizeBrand(string(brand))
	if b == "" {
		b = BrandChrome
	}
	return FingerprintSpec{
		Seed:                 RandomSeed(),
		Platform:             p,
		PlatformVersion:      DefaultPlatformVersion(p),
		Brand:                b,
		HardwareConcurrency:  8,
		Lang:                 "en-US",
		AcceptLang:           "en-US,en",
		Timezone:             "America/New_York",
		DisableSpoofing:      []SpoofModule{SpoofCanvas},
		DisableNonProxiedUDP: true,
		QUICMode:             QUICAuto,
		Permissions:          PermissionsSpec{HardwareGuard: PermissionBlock},
		WindowWidth:          1440,
		WindowHeight:         900,
	}
}

// DefaultPlatformVersion returns a realistic OS version for --fingerprint-platform-version
// (matching Chromium UA-CH Sec-CH-UA-Platform-Version: macOS 15.2.0, Windows 11 = 15.0.0, Linux 6.8.0).
func DefaultPlatformVersion(p Platform) string {
	switch p {
	case PlatformMacOS:
		return "15.2.0"
	case PlatformWindows:
		return "15.0.0"
	case PlatformLinux:
		return "6.8.0"
	default:
		return ""
	}
}

// RandomCoherentFingerprint builds a statistically realistic, cross-signal-consistent
// FingerprintSpec for the chosen platform, brand, and optional timezone.
func RandomCoherentFingerprint(platform Platform, brand Brand, timezone string) FingerprintSpec {
	p := NormalizePlatform(string(platform))
	if p == "" {
		p = PlatformMacOS
	}
	b := NormalizeBrand(string(brand))
	if b == "" || b == BrandChromium {
		b = BrandChrome
	}
	seed := RandomSeed()
	pick := func(n int) int {
		if n <= 0 {
			return 0
		}
		return int(seed % uint32(n))
	}

	var (
		platVer string
		w, h    int
		dpr     float64
	)
	switch p {
	case PlatformWindows:
		vers := []string{"15.0.0", "15.0.0", "14.0.0", "10.0.0"}
		platVer = vers[pick(len(vers))]
		type winDisp struct {
			w, h int
			dpr  float64
		}
		disps := []winDisp{
			{1920, 1080, 1.0},
			{1920, 1080, 1.25},
			{1536, 864, 1.25},
			{1366, 768, 1.0},
			{2560, 1440, 1.25},
			{2560, 1440, 1.5},
		}
		d := disps[int((seed>>3)%uint32(len(disps)))]
		w, h, dpr = d.w, d.h, d.dpr
	case PlatformLinux:
		vers := []string{"6.8.0", "6.11.0", "6.6.0"}
		platVer = vers[pick(len(vers))]
		type linDisp struct {
			w, h int
			dpr  float64
		}
		disps := []linDisp{
			{1920, 1080, 1.0},
			{1440, 900, 1.0},
			{1680, 1050, 1.0},
			{2560, 1440, 1.0},
		}
		d := disps[int((seed>>3)%uint32(len(disps)))]
		w, h, dpr = d.w, d.h, d.dpr
	default: // macOS
		vers := []string{"15.3.0", "15.2.0", "14.7.0"}
		platVer = vers[pick(len(vers))]
		type macDisp struct {
			w, h int
			dpr  float64
		}
		disps := []macDisp{
			{1512, 982, 2.0},
			{1440, 900, 2.0},
			{1680, 1050, 2.0},
			{1728, 1117, 2.0},
			{1920, 1080, 1.0},
		}
		d := disps[int((seed>>3)%uint32(len(disps)))]
		w, h, dpr = d.w, d.h, d.dpr
	}

	tz := strings.TrimSpace(timezone)
	lang := "en-US"
	acceptLang := "en-US,en"
	if reg, ok := RegionForTimezone(tz); ok {
		tz = reg.Timezone
		lang = reg.Lang
		acceptLang = reg.AcceptLang
	} else if tz == "" {
		reg := CommonRegions[int((seed>>7)%uint32(len(CommonRegions)))]
		tz = reg.Timezone
		lang = reg.Lang
		acceptLang = reg.AcceptLang
	}

	return FingerprintSpec{
		Seed:                 seed,
		Platform:             p,
		PlatformVersion:      platVer,
		Brand:                b,
		HardwareConcurrency:  0, // seed-derived even cores [8..32]
		Lang:                 lang,
		AcceptLang:           acceptLang,
		Timezone:             tz,
		DisableSpoofing:      []SpoofModule{SpoofCanvas},
		DisableNonProxiedUDP: true,
		WebRTCPolicy:         WebRTCDisableNonProxiedUDP,
		DNSLeakMode:          DNSLeakAuto,
		QUICMode:             QUICAuto,
		AutomationMode:       AutomationHide,
		DoNotTrack:           DoNotTrackOff,
		GPUMode:              GPUAuto,
		ColorScheme:          ColorSchemeSystem,
		Permissions:          PermissionsSpec{HardwareGuard: PermissionBlock},
		WindowWidth:          w,
		WindowHeight:         h,
		DeviceScaleFactor:    dpr,
	}
}

// AlignToRegion updates Timezone, Lang, AcceptLang (and optionally Geolocation)
// to match a coherent geographic region profile.
func (f *FingerprintSpec) AlignToRegion(reg RegionProfile, includeGeo bool) {
	if reg.Timezone != "" {
		f.Timezone = reg.Timezone
	}
	if reg.Lang != "" {
		f.Lang = reg.Lang
	}
	if reg.AcceptLang != "" {
		f.AcceptLang = reg.AcceptLang
	} else if reg.Lang != "" {
		f.AcceptLang = DefaultAcceptLang(reg.Lang)
	}
	if includeGeo {
		lat, lon := reg.Latitude, reg.Longitude
		f.Geolocation = GeolocationSpec{
			Mode:      PermissionAllow,
			Latitude:  &lat,
			Longitude: &lon,
			Accuracy:  50,
		}
	}
}

// SnapSeedForChromeDeviceMemory snaps a positive 31-bit seed to a multiple of 3
// so adryfish/fingerprint-chromium 148+ (021-device-memory-cap.patch: {8,16,32}[seed%3])
// always derives navigator.deviceMemory=8 (matching W3C/real Chrome's 8 GB cap instead
// of exposing spec-illegal 16/32 GB values). Because gcd(3, 13) == 1, CPU core derivation
// ((seed % 13) + 4) * 2 retains full uniform coverage across all 13 even values [8..32].
func SnapSeedForChromeDeviceMemory(seed uint32) uint32 {
	v := seed & 0x7fffffff
	if v == 0 {
		return 10002
	}
	rem := v % 3
	if rem == 0 {
		return v
	}
	if v <= 0x7fffffff-(3-rem) {
		return v + (3 - rem)
	}
	return v - rem
}

// DeriveSeed deterministically maps an arbitrary identifier to a non-zero
// 32-bit fingerprint seed in [3, 2147483646] divisible by 3 (fits signed 32-bit
// int and guarantees W3C-compliant navigator.deviceMemory=8 on Chrome 148+).
func DeriveSeed(id string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	v := h.Sum32() & 0x7fffffff
	if v == 0 {
		return 10002
	}
	return SnapSeedForChromeDeviceMemory(v)
}

// RandomSeed generates a cryptographically random non-zero positive 31-bit seed
// divisible by 3 (compatible with signed/unsigned 32-bit CLI flag parsers and
// guaranteeing W3C-compliant navigator.deviceMemory=8 on Chrome 148+).
func RandomSeed() uint32 {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return DeriveSeed(fmt.Sprintf("fallback-%d", time.Now().UnixNano()))
	}
	v := binary.BigEndian.Uint32(buf[:]) & 0x7fffffff
	if v == 0 {
		return 20260104
	}
	return SnapSeedForChromeDeviceMemory(v)
}

// InferBrandFromUA detects a brand hint from a User-Agent string.
func InferBrandFromUA(ua string) Brand {
	u := strings.ToLower(ua)
	switch {
	case strings.Contains(u, "edg/"):
		return BrandEdge
	case strings.Contains(u, "opr/") || strings.Contains(u, "opera"):
		return BrandOpera
	case strings.Contains(u, "vivaldi"):
		return BrandVivaldi
	case strings.Contains(u, "chrome/"):
		return BrandChrome
	default:
		return BrandChromium
	}
}

// InferPlatformFromUA detects a platform hint from a User-Agent string.
func InferPlatformFromUA(ua string) Platform {
	u := strings.ToLower(ua)
	switch {
	case strings.Contains(u, "macintosh") || strings.Contains(u, "mac os x"):
		return PlatformMacOS
	case strings.Contains(u, "windows"):
		return PlatformWindows
	case strings.Contains(u, "linux"):
		return PlatformLinux
	default:
		return ""
	}
}

// IsSpoofingDisabled reports whether mod is present in DisableSpoofing.
func (f FingerprintSpec) IsSpoofingDisabled(mod SpoofModule) bool {
	return slices.Contains(f.DisableSpoofing, mod)
}

// SetSpoofingEnabled enables (true) or disables (false) a single spoof module.
func (f *FingerprintSpec) SetSpoofingEnabled(mod SpoofModule, enabled bool) {
	if enabled {
		f.DisableSpoofing = slices.DeleteFunc(f.DisableSpoofing, func(m SpoofModule) bool { return m == mod })
		return
	}
	if !f.IsSpoofingDisabled(mod) {
		f.DisableSpoofing = append(f.DisableSpoofing, mod)
	}
	f.DisableSpoofing = CanonicalSpoofModules(f.DisableSpoofing)
}

// EffectiveConcurrency returns the explicit HardwareConcurrency or the seed-derived count.
func (f FingerprintSpec) EffectiveConcurrency() int {
	if f.HardwareConcurrency > 0 {
		return f.HardwareConcurrency
	}
	return DerivedHardwareConcurrency(f.Seed)
}

// EffectiveDeviceMemoryGB returns the navigator.deviceMemory GB derived from Seed.
func (f FingerprintSpec) EffectiveDeviceMemoryGB() int {
	return DerivedDeviceMemoryGB(f.Seed)
}

// ActiveSpoofCount returns how many of the 5 Chrome 144+ spoof modules remain enabled.
func (f FingerprintSpec) ActiveSpoofCount() int {
	return len(AllSpoofModules) - len(CanonicalSpoofModules(f.DisableSpoofing))
}

// SetWebRTCPolicy sets both WebRTCPolicy and legacy DisableNonProxiedUDP coherently.
func (f *FingerprintSpec) SetWebRTCPolicy(p WebRTCPolicy) {
	norm := NormalizeWebRTCPolicy(string(p))
	if norm == "" {
		norm = WebRTCDisableNonProxiedUDP
	}
	f.WebRTCPolicy = norm
	f.DisableNonProxiedUDP = norm == WebRTCDisableNonProxiedUDP
}

// EffectiveDNSLeakMode returns DNSLeakAuto unless explicitly opted out via "system".
func (f FingerprintSpec) EffectiveDNSLeakMode() DNSLeakMode {
	if m := NormalizeDNSLeakMode(string(f.DNSLeakMode)); m != "" {
		return m
	}
	return DNSLeakAuto
}

// EffectiveQUICMode returns QUICAuto unless explicitly set to disable or allow.
func (f FingerprintSpec) EffectiveQUICMode() QUICMode {
	if m := NormalizeQUICMode(string(f.QUICMode)); m != "" {
		return m
	}
	return QUICAuto
}

// EffectiveAutomationMode returns AutomationHide by default.
func (f FingerprintSpec) EffectiveAutomationMode() AutomationMode {
	if m := NormalizeAutomationMode(string(f.AutomationMode)); m != "" {
		return m
	}
	return AutomationHide
}

// EffectiveDoNotTrack returns DoNotTrackOff unless explicitly enabled.
func (f FingerprintSpec) EffectiveDoNotTrack() DoNotTrackMode {
	if m := NormalizeDoNotTrack(string(f.DoNotTrack)); m != "" {
		return m
	}
	return DoNotTrackOff
}

// EffectiveGPUMode returns GPUAuto unless explicitly overridden.
func (f FingerprintSpec) EffectiveGPUMode() GPUMode {
	if m := NormalizeGPUMode(string(f.GPUMode)); m != "" {
		return m
	}
	return GPUAuto
}

// EffectiveColorScheme returns ColorSchemeSystem unless explicitly set.
func (f FingerprintSpec) EffectiveColorScheme() ColorSchemeMode {
	if m := NormalizeColorScheme(string(f.ColorScheme)); m != "" {
		return m
	}
	return ColorSchemeSystem
}

// Summary returns a concise human-readable description for UI tables/badges.
func (f FingerprintSpec) Summary() string {
	parts := make([]string, 0, 5)
	if f.Seed > 0 {
		parts = append(parts, fmt.Sprintf("#%d", f.Seed))
	}
	if f.Platform != "" {
		parts = append(parts, string(f.Platform))
	}
	if f.Brand != "" {
		parts = append(parts, string(f.Brand))
	}
	if f.HardwareConcurrency > 0 {
		parts = append(parts, fmt.Sprintf("%dC", f.HardwareConcurrency))
	}
	if f.Timezone != "" {
		parts = append(parts, f.Timezone)
	}
	if len(parts) == 0 {
		return "默认指纹"
	}
	return strings.Join(parts, " · ")
}
