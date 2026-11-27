// Package engine discovers, verifies and installs fingerprint-chromium releases.
package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Official adryfish/fingerprint-chromium repository endpoints.
const (
	UpstreamRepoURL      = "https://github.com/adryfish/fingerprint-chromium"
	UpstreamReleasesPage = "https://github.com/adryfish/fingerprint-chromium/releases"
	UpstreamLatestAPI    = "https://api.github.com/repos/adryfish/fingerprint-chromium/releases/latest"
	UpstreamReleasesAPI  = "https://api.github.com/repos/adryfish/fingerprint-chromium/releases?per_page=20"

	EnvChromiumPath = "FINGERPRINT_CHROMIUM_PATH"
)

// BinaryKind classifies a discovered browser executable.
type BinaryKind string

const (
	KindFingerprintChromium BinaryKind = "fingerprint-chromium"
	KindChromium            BinaryKind = "chromium"
	KindChrome              BinaryKind = "chrome"
	KindUnknown             BinaryKind = "unknown"
)

// Label returns a human-readable Chinese display label for UI badges.
func (k BinaryKind) Label() string {
	switch k {
	case KindFingerprintChromium:
		return "fingerprint-chromium"
	case KindChromium:
		return "Chromium"
	case KindChrome:
		return "Google Chrome（回退）"
	default:
		return "未知内核"
	}
}

// KernelCapabilities describes verified runtime capabilities of a discovered browser binary.
type KernelCapabilities struct {
	MajorVersion            int      `json:"major_version,omitempty"`
	FingerprintPatched      bool     `json:"fingerprint_patched"`        // verified --fingerprint-platform patch or fingerprint-chromium build
	BinaryVerified          bool     `json:"binary_verified,omitempty"`  // true when C++ switch literals were directly found in binary
	SupportsDisableSpoofing bool     `json:"supports_disable_spoofing"`  // Chrome 144+ --disable-spoofing=<csv>
	SupportsDeviceMemory148 bool     `json:"supports_device_memory_148"` // Chrome 148+ randomized deviceMemory & realistic GPU profiles
	IsFallbackChrome        bool     `json:"is_fallback_chrome"`         // stock Chrome/unpatched binary (ignores --fingerprint-*)
	Quarantined             bool     `json:"quarantined,omitempty"`      // macOS com.apple.quarantine detected
	Notes                   []string `json:"notes,omitempty"`
}

// BinaryProbeResult holds static binary inspection results for a browser executable.
type BinaryProbeResult struct {
	Scanned                  bool
	HasFingerprintSwitch     bool
	HasDisableSpoofingSwitch bool
	Quarantined              bool
}

// KernelInfo describes a discovered or configured browser binary.
type KernelInfo struct {
	Path         string             `json:"path"`
	Version      string             `json:"version"`
	Kind         BinaryKind         `json:"kind"`
	Source       string             `json:"source"` // "configured" | "env" | "managed" | "system"
	Exists       bool               `json:"exists"`
	Capabilities KernelCapabilities `json:"capabilities"`
}

// KernelManager handles discovery, probing, and release queries for
// adryfish/fingerprint-chromium. All external I/O boundaries are injectable for tests.
type KernelManager struct {
	mu             sync.RWMutex
	DataDir        string
	ConfiguredPath string
	GOOS           string
	GOARCH         string
	HomeDir        string
	PathEnv        string                // optional host-specific binary override, before FINGERPRINT_CHROMIUM_PATH
	ArchiveSuffix  string                // host-required archive format, e.g. .tar.xz when AppImage is unavailable
	AcceptRelease  func(tag string) bool // optional host compatibility policy
	Getenv         func(string) string
	Stat           func(string) (os.FileInfo, error)
	RunVersion     func(ctx context.Context, bin string) (string, error)
	ProbeBinary    func(bin string) BinaryProbeResult
	HTTPClient     *http.Client
	LatestAPIURL   string
	ReleasesAPIURL string
}

// SetConfiguredPath updates ConfiguredPath safely for concurrent readers.
func (m *KernelManager) SetConfiguredPath(path string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.ConfiguredPath = path
	m.mu.Unlock()
}

// GetConfiguredPath reads ConfiguredPath safely for concurrent writers.
func (m *KernelManager) GetConfiguredPath() string {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ConfiguredPath
}

// NewKernelManager constructs a production KernelManager.
func NewKernelManager(dataDir, configuredPath string) *KernelManager {
	return &KernelManager{
		DataDir:        dataDir,
		ConfiguredPath: configuredPath,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		Getenv:         os.Getenv,
		Stat:           os.Stat,
		RunVersion:     defaultRunVersion,
		ProbeBinary:    defaultProbeBinary,
		HTTPClient:     &http.Client{Timeout: 20 * time.Second},
		LatestAPIURL:   UpstreamLatestAPI,
		ReleasesAPIURL: UpstreamReleasesAPI,
	}
}

func (m *KernelManager) goos() string {
	if m.GOOS != "" {
		return m.GOOS
	}
	return runtime.GOOS
}

func (m *KernelManager) goarch() string {
	if m.GOARCH != "" {
		return m.GOARCH
	}
	return runtime.GOARCH
}

func (m *KernelManager) getenv(k string) string {
	if m.Getenv != nil {
		return m.Getenv(k)
	}
	return os.Getenv(k)
}

func (m *KernelManager) home() string {
	if m.HomeDir != "" {
		return m.HomeDir
	}
	h, _ := os.UserHomeDir()
	return h
}

func (m *KernelManager) existsFile(p string) bool {
	if p == "" {
		return false
	}
	stFn := m.Stat
	if stFn == nil {
		stFn = os.Stat
	}
	fi, err := stFn(p)
	return err == nil && !fi.IsDir()
}

// ManagedKernelDir returns <DataDir>/kernels/fingerprint-chromium.
func (m *KernelManager) ManagedKernelDir() string {
	if m.DataDir == "" {
		return ""
	}
	return filepath.Join(m.DataDir, "kernels", "fingerprint-chromium")
}

// NormalizeBinaryPath resolves a macOS .app bundle directory (selected via file
// dialog) into the actual executable inside Contents/MacOS/.
func NormalizeBinaryPath(goos, raw string) string {
	p := strings.TrimSpace(raw)
	if p == "" {
		return ""
	}
	if goos == "darwin" && strings.HasSuffix(strings.ToLower(p), ".app") {
		base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		candidate := filepath.Join(p, "Contents", "MacOS", base)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		// Fallback common names inside Chromium/Chrome bundles.
		for _, name := range []string{"Chromium", "Google Chrome", "Chrome"} {
			alt := filepath.Join(p, "Contents", "MacOS", name)
			if _, err := os.Stat(alt); err == nil {
				return alt
			}
		}
		return candidate
	}
	return p
}

// DetectAll scans configured path, env vars, managed directory, and system paths,
// returning deduplicated candidates in strict priority order.
func (m *KernelManager) DetectAll() []KernelInfo {
	goos := m.goos()
	type cand struct {
		path   string
		source string
	}
	var raw []cand
	add := func(path, source string) {
		p := NormalizeBinaryPath(goos, path)
		if p != "" {
			raw = append(raw, cand{path: p, source: source})
		}
	}

	if cfgPath := m.GetConfiguredPath(); cfgPath != "" {
		add(cfgPath, "configured")
	}
	if m.PathEnv != "" {
		add(m.getenv(m.PathEnv), "env")
	}
	if v := m.getenv(EnvChromiumPath); v != "" {
		add(v, "env")
	}

	if md := m.ManagedKernelDir(); md != "" {
		if active, ok := ActiveRelease(md); ok {
			add(active.Executable, "managed")
		}
		addManagedInDir := func(dir string) {
			switch goos {
			case "darwin":
				add(filepath.Join(dir, "Chromium.app", "Contents", "MacOS", "Chromium"), "managed")
				add(filepath.Join(dir, "Fingerprint Chromium.app", "Contents", "MacOS", "Fingerprint Chromium"), "managed")
				add(filepath.Join(dir, "Fingerprint Chromium.app", "Contents", "MacOS", "Chromium"), "managed")
				if ents, err := os.ReadDir(dir); err == nil {
					for _, e := range ents {
						if e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".app") {
							add(filepath.Join(dir, e.Name()), "managed")
						}
					}
				}
			case "windows":
				add(filepath.Join(dir, "chrome.exe"), "managed")
				add(filepath.Join(dir, "chromium.exe"), "managed")
			default:
				add(filepath.Join(dir, "chrome"), "managed")
				add(filepath.Join(dir, "chromium"), "managed")
				add(filepath.Join(dir, "fingerprint-chromium"), "managed")
			}
		}
		addManagedInDir(md)
		// Also check 1-level extracted release subdirectories (e.g. 148.0.7778.215_windows_x64/chrome.exe)
		// and managed Linux .AppImage binaries dropped directly into ManagedKernelDir, ordered newest version first.
		if entries, err := os.ReadDir(md); err == nil {
			sortDirEntriesByVersionDesc(entries)
			for _, ent := range entries {
				if ent.IsDir() {
					if goos == "darwin" && strings.HasSuffix(strings.ToLower(ent.Name()), ".app") {
						add(filepath.Join(md, ent.Name()), "managed")
						continue
					}
					addManagedInDir(filepath.Join(md, ent.Name()))
				} else if goos == "linux" && strings.HasSuffix(strings.ToLower(ent.Name()), ".appimage") {
					add(filepath.Join(md, ent.Name()), "managed")
				}
			}
		}
	}

	home := m.home()
	switch goos {
	case "darwin":
		add("/Applications/Fingerprint Chromium.app/Contents/MacOS/Chromium", "system")
		add("/Applications/Chromium.app/Contents/MacOS/Chromium", "system")
		if home != "" {
			add(filepath.Join(home, "Applications", "Chromium.app", "Contents", "MacOS", "Chromium"), "system")
		}
		add("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "system")
		if home != "" {
			add(filepath.Join(home, "Applications", "Google Chrome.app", "Contents", "MacOS", "Google Chrome"), "system")
		}
	case "windows":
		add(`C:\Program Files\fingerprint-chromium\chrome.exe`, "system")
		add(`C:\Program Files\Chromium\Application\chrome.exe`, "system")
		add(`C:\Program Files (x86)\Chromium\Application\chrome.exe`, "system")
		add(`C:\Program Files\Google\Chrome\Application\chrome.exe`, "system")
		if la := m.getenv("LOCALAPPDATA"); la != "" {
			add(la+`\Chromium\Application\chrome.exe`, "system")
			add(la+`\Google\Chrome\Application\chrome.exe`, "system")
		}
	default:
		add("/usr/bin/fingerprint-chromium", "system")
		add("/usr/local/bin/fingerprint-chromium", "system")
		add("/usr/bin/ungoogled-chromium", "system")
		add("/usr/bin/chromium", "system")
		add("/usr/bin/chromium-browser", "system")
		add("/usr/bin/google-chrome", "system")
		add("/usr/bin/google-chrome-stable", "system")
	}

	seen := map[string]bool{}
	out := make([]KernelInfo, 0, len(raw))
	for _, c := range raw {
		if seen[c.path] {
			continue
		}
		seen[c.path] = true
		if !m.existsFile(c.path) {
			continue
		}
		info := m.Probe(c.path)
		info.Source = c.source
		out = append(out, info)
	}
	return out
}

// SelectPreferredKernel chooses the active browser binary from a DetectAll slice.
// When the top auto-discovered candidate is not a verified fingerprint-chromium build
// (and was not explicitly user-configured), SelectPreferredKernel promotes the first
// verified fingerprint-chromium candidate so stock Chrome/Chromium cannot shadow it.
func SelectPreferredKernel(all []KernelInfo) (KernelInfo, bool) {
	if len(all) == 0 {
		return KernelInfo{}, false
	}
	if all[0].Source != "configured" && (!all[0].Capabilities.FingerprintPatched || all[0].Capabilities.IsFallbackChrome) {
		for _, cand := range all[1:] {
			if cand.Capabilities.FingerprintPatched && !cand.Capabilities.IsFallbackChrome {
				return cand, true
			}
		}
	}
	return all[0], true
}

// ResolvePreferred returns the highest-priority existing browser binary using
// SelectPreferredKernel promotion rules.
func (m *KernelManager) ResolvePreferred() (KernelInfo, error) {
	best, ok := SelectPreferredKernel(m.DetectAll())
	if !ok {
		return KernelInfo{}, errors.New("fpchromium: 未找到可用的 fingerprint-chromium 或 Chromium 内核，请在「内核管理」中下载或选择本地路径")
	}
	return best, nil
}

// Probe inspects a single binary path, classifying its kind, version, and
// verified adryfish/fingerprint-chromium C++ switch capabilities.
func (m *KernelManager) Probe(path string) KernelInfo {
	norm := NormalizeBinaryPath(m.goos(), path)
	info := KernelInfo{
		Path:   norm,
		Kind:   ClassifyBinaryKind(norm, ""),
		Exists: m.existsFile(norm),
	}
	if !info.Exists {
		return info
	}
	run := m.RunVersion
	if run == nil {
		run = defaultRunVersion
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if ver, err := run(ctx, norm); err == nil {
		info.Version = strings.TrimSpace(ver)
		info.Kind = ClassifyBinaryKind(norm, info.Version)
	}

	probeFn := m.ProbeBinary
	if probeFn == nil && m.Stat == nil {
		// Default static binary probe only runs when using real filesystem (not synthetic mocked Stat).
		probeFn = defaultProbeBinary
	}
	var binRes BinaryProbeResult
	if probeFn != nil {
		binRes = probeFn(norm)
	}
	if binRes.HasFingerprintSwitch {
		info.Kind = KindFingerprintChromium
	}
	info.Capabilities = buildKernelCapabilities(info.Kind, info.Version, binRes)
	return info
}

var chromiumVersionRe = regexp.MustCompile(`\b(\d{2,3})\.\d+\.\d+\.\d+\b`)

// ReleaseVersion returns the dotted Chromium version reported by the installed binary.
func (k KernelInfo) ReleaseVersion() string {
	if !k.Exists || k.Kind != KindFingerprintChromium {
		return ""
	}
	return chromiumVersionRe.FindString(k.Version)
}

// ParseChromiumMajorVersion extracts the Chromium milestone major version (e.g. 148)
// from a --version output string such as "Chromium 148.0.7778.215" or "Google Chrome 139.0.7258.66".
func ParseChromiumMajorVersion(version string) int {
	if m := chromiumVersionRe.FindStringSubmatch(version); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return n
		}
	}
	// Fallback: bare integer token following "Chromium " or "Chrome ".
	for _, field := range strings.Fields(version) {
		if n, err := strconv.Atoi(field); err == nil && n >= 70 && n < 500 {
			return n
		}
	}
	return 0
}

func buildKernelCapabilities(kind BinaryKind, version string, probe BinaryProbeResult) KernelCapabilities {
	major := ParseChromiumMajorVersion(version)
	patched := kind == KindFingerprintChromium || probe.HasFingerprintSwitch
	// If binary was directly scanned on disk and did NOT contain fingerprint-platform
	// and is generic Chromium/Chrome, classify as unpatched fallback.
	isFallback := kind == KindChrome || (probe.Scanned && !probe.HasFingerprintSwitch && kind != KindFingerprintChromium)

	supportsDisableSpoofing := patched && (probe.HasDisableSpoofingSwitch || major == 0 || major >= 144)
	supportsDeviceMemory148 := patched && major >= 148

	var notes []string
	if isFallback {
		notes = append(notes, "未检测到 fingerprint-platform 源码级补丁，--fingerprint-* 与 --disable-spoofing 参数不会生效")
	} else if patched {
		if probe.HasFingerprintSwitch {
			notes = append(notes, "已通过二进制符号校验，确认包含 adryfish/fingerprint-chromium 源码级伪装补丁")
		}
		if major > 0 && major < 144 {
			notes = append(notes, fmt.Sprintf("Chromium %d 低于 144，不支持细粒度 --disable-spoofing", major))
		} else if supportsDeviceMemory148 {
			notes = append(notes, fmt.Sprintf("Chromium %d 支持 Chrome 148+ 种子衍生内存与真实厂商 GPU 渲染配置", major))
		} else if supportsDisableSpoofing {
			notes = append(notes, "支持 Chrome 144+ 细粒度 --disable-spoofing 五模块开关")
		}
	}
	if probe.Quarantined {
		notes = append(notes, "检测到 macOS Gatekeeper 隔离属性 (com.apple.quarantine)，首次启动可能被系统拦截")
	}

	return KernelCapabilities{
		MajorVersion:            major,
		FingerprintPatched:      patched,
		BinaryVerified:          probe.HasFingerprintSwitch,
		SupportsDisableSpoofing: supportsDisableSpoofing,
		SupportsDeviceMemory148: supportsDeviceMemory148,
		IsFallbackChrome:        isFallback,
		Quarantined:             probe.Quarantined,
		Notes:                   notes,
	}
}

type binaryScanCacheKey struct {
	path    string
	size    int64
	modNano int64
}

type binaryScanCacheVal struct {
	hasFP      bool
	hasDisable bool
	scanned    bool
}

var (
	binaryScanCacheMu sync.RWMutex
	binaryScanCache   = map[binaryScanCacheKey]binaryScanCacheVal{}
)

func defaultProbeBinary(binPath string) BinaryProbeResult {
	res := BinaryProbeResult{
		Quarantined: detectMacQuarantine(binPath),
	}
	target := resolveInspectableBinary(binPath)
	if target == "" {
		return res
	}
	fi, err := os.Stat(target)
	if err != nil || fi.IsDir() {
		return res
	}
	key := binaryScanCacheKey{
		path:    target,
		size:    fi.Size(),
		modNano: fi.ModTime().UnixNano(),
	}
	binaryScanCacheMu.RLock()
	cached, ok := binaryScanCache[key]
	binaryScanCacheMu.RUnlock()
	if ok {
		res.Scanned = cached.scanned
		res.HasFingerprintSwitch = cached.hasFP
		res.HasDisableSpoofingSwitch = cached.hasDisable
		return res
	}

	hasFP, hasDisable, scanned := scanBinaryForSwitches(target)
	binaryScanCacheMu.Lock()
	if len(binaryScanCache) > 64 {
		clear(binaryScanCache)
	}
	binaryScanCache[key] = binaryScanCacheVal{
		hasFP:      hasFP,
		hasDisable: hasDisable,
		scanned:    scanned,
	}
	binaryScanCacheMu.Unlock()

	res.Scanned = scanned
	res.HasFingerprintSwitch = hasFP
	res.HasDisableSpoofingSwitch = hasDisable
	return res
}

// resolveInspectableBinary locates the actual Chromium core binary containing
// compiled CLI switch strings (Chromium Framework on macOS .app, chrome.dll on Windows,
// or the executable itself on Linux).
func resolveInspectableBinary(binPath string) string {
	fi, err := os.Stat(binPath)
	if err != nil || fi.IsDir() {
		return ""
	}
	// Linux AppImage embeds a compressed SquashFS payload; raw byte scanning
	// cannot observe compiled switch literals inside compressed blocks.
	if strings.HasSuffix(strings.ToLower(binPath), ".appimage") {
		return ""
	}
	// macOS .app bundle: executable in Contents/MacOS/ is a tiny trampoline; C++ switches live in Frameworks/*.framework.
	// Always prefer Versions/Current first so upgraded bundles don't scan stale older Versions/<old> directories.
	macOSDir := filepath.Dir(binPath)
	contentsDir := filepath.Dir(macOSDir)
	if strings.EqualFold(filepath.Base(macOSDir), "macos") && strings.EqualFold(filepath.Base(contentsDir), "contents") {
		currentCandidates := []string{
			filepath.Join(contentsDir, "Frameworks", "Chromium Framework.framework", "Versions", "Current", "Chromium Framework"),
			filepath.Join(contentsDir, "Frameworks", "Google Chrome Framework.framework", "Versions", "Current", "Google Chrome Framework"),
		}
		for _, cur := range currentCandidates {
			if mfi, merr := os.Stat(cur); merr == nil && !mfi.IsDir() {
				return cur
			}
		}
		patterns := []string{
			filepath.Join(contentsDir, "Frameworks", "Chromium Framework.framework", "Versions", "*", "Chromium Framework"),
			filepath.Join(contentsDir, "Frameworks", "Google Chrome Framework.framework", "Versions", "*", "Google Chrome Framework"),
		}
		for _, pat := range patterns {
			if matches, err := filepath.Glob(pat); err == nil && len(matches) > 0 {
				sortPathsByVersionDesc(matches)
				for _, m := range matches {
					if mfi, merr := os.Stat(m); merr == nil && !mfi.IsDir() {
						return m
					}
				}
			}
		}
	}
	// Windows: chrome.exe delegates switches to sibling chrome.dll or newest versioned <ver>\chrome.dll.
	if strings.HasSuffix(strings.ToLower(binPath), ".exe") {
		dir := filepath.Dir(binPath)
		dll := filepath.Join(dir, "chrome.dll")
		if dfi, derr := os.Stat(dll); derr == nil && !dfi.IsDir() {
			return dll
		}
		if matches, err := filepath.Glob(filepath.Join(dir, "*", "chrome.dll")); err == nil && len(matches) > 0 {
			sortPathsByVersionDesc(matches)
			for _, m := range matches {
				if dfi, derr := os.Stat(m); derr == nil && !dfi.IsDir() {
					return m
				}
			}
		}
	}
	return binPath
}

func sortDirEntriesByVersionDesc(entries []os.DirEntry) {
	parseLeadingVer := func(name string) []int {
		// Extract leading dotted version token (e.g. "149.0.7778.215_linux_x64" -> [149, 0, 7778, 215]).
		verTok := name
		if idx := strings.IndexAny(verTok, "_- "); idx > 0 {
			verTok = verTok[:idx]
		}
		parts := strings.Split(verTok, ".")
		out := make([]int, 0, len(parts))
		for _, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				break
			}
			out = append(out, n)
		}
		return out
	}
	sort.SliceStable(entries, func(i, j int) bool {
		pi, pj := parseLeadingVer(entries[i].Name()), parseLeadingVer(entries[j].Name())
		for k := 0; k < len(pi) && k < len(pj); k++ {
			if pi[k] != pj[k] {
				return pi[k] > pj[k]
			}
		}
		if len(pi) != len(pj) {
			return len(pi) > len(pj)
		}
		return entries[i].Name() > entries[j].Name()
	})
}

// sortPathsByVersionDesc orders versioned Chromium paths (e.g. .../148.0.7778.215/...)
// newest-version first so binary probing always inspects the active latest build.
func sortPathsByVersionDesc(paths []string) {
	parseParts := func(p string) []int {
		dirName := filepath.Base(filepath.Dir(p))
		raw := strings.Split(dirName, ".")
		out := make([]int, 0, len(raw))
		for _, part := range raw {
			n, err := strconv.Atoi(part)
			if err != nil {
				return nil
			}
			out = append(out, n)
		}
		return out
	}
	sort.SliceStable(paths, func(i, j int) bool {
		pi, pj := parseParts(paths[i]), parseParts(paths[j])
		for k := 0; k < len(pi) && k < len(pj); k++ {
			if pi[k] != pj[k] {
				return pi[k] > pj[k]
			}
		}
		if len(pi) != len(pj) {
			return len(pi) > len(pj)
		}
		return paths[i] > paths[j]
	})
}

var (
	tokenFingerprintPlatform = []byte("fingerprint-platform")
	tokenDisableSpoofing     = []byte("disable-spoofing")
)

// scanBinaryForSwitches streams path in 256 KiB chunks with overlap to detect
// compiled adryfish/fingerprint-chromium C++ CLI switch tokens without executing it.
func scanBinaryForSwitches(path string) (hasFP, hasDisable, scanned bool) {
	f, err := os.Open(path)
	if err != nil {
		return false, false, false
	}
	defer func() { _ = f.Close() }()

	// Skip tiny wrapper scripts (< 512 bytes) unless they literally mention switches,
	// so fake test shell scripts aren't falsely flagged as "scanned unpatched binaries".
	fi, err := f.Stat()
	if err != nil {
		return false, false, false
	}
	isSmallScript := fi.Size() < 4096

	const chunkSize = 256 * 1024
	const overlap = 64
	buf := make([]byte, overlap+chunkSize)
	carry := 0
	for {
		n, rerr := f.Read(buf[carry:])
		if n > 0 {
			window := buf[:carry+n]
			if !hasFP && bytes.Contains(window, tokenFingerprintPlatform) {
				hasFP = true
			}
			if !hasDisable && bytes.Contains(window, tokenDisableSpoofing) {
				hasDisable = true
			}
			if hasFP && hasDisable {
				return true, true, true
			}
			if len(window) >= overlap {
				copy(buf[:overlap], window[len(window)-overlap:])
				carry = overlap
			} else {
				carry = len(window)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return hasFP, hasDisable, false
		}
	}
	if isSmallScript && !hasFP && !hasDisable {
		return false, false, false
	}
	return hasFP, hasDisable, true
}

// ClassifyBinaryKind determines whether a binary is fingerprint-chromium,
// generic Chromium, or fallback Google Chrome.
func ClassifyBinaryKind(path, version string) BinaryKind {
	lp := strings.ToLower(path)
	lv := strings.ToLower(version)
	switch {
	case strings.Contains(lp, "fingerprint-chromium") || strings.Contains(lp, "fingerprint chromium") || strings.Contains(lv, "fingerprint"):
		return KindFingerprintChromium
	case strings.Contains(lp, "google chrome") || strings.Contains(lp, "google-chrome") || strings.HasPrefix(lv, "google chrome"):
		return KindChrome
	case strings.Contains(lp, "chromium") || strings.Contains(lp, "ungoogled") || strings.Contains(lv, "chromium"):
		return KindChromium
	default:
		if path != "" {
			return KindChromium
		}
		return KindUnknown
	}
}

func defaultRunVersion(ctx context.Context, bin string) (string, error) {
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
