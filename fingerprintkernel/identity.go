package fingerprintkernel

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

const (
	minGeneratedSeed uint32 = 50000
	maxGeneratedSeed uint32 = 0x7fffffff
	genLockFile             = ".browserkernel-gen.lock"
)

var identityVersionRe = regexp.MustCompile(`^\d+(?:\.\d+){1,3}$`)

var identityGPUModulus = map[Platform]int{
	PlatformMacOS:   2,
	PlatformWindows: 10,
	PlatformLinux:   2,
}

var identityCoreChoices = map[Platform][]int{
	PlatformMacOS:   {8, 10, 12, 14, 16},
	PlatformWindows: {4, 6, 8, 12, 16, 20, 24},
	PlatformLinux:   {4, 6, 8, 12, 16, 24},
}

var macOSSequoiaVersions = []string{"15.0.1", "15.1.1", "15.2.0", "15.3.1", "15.3.2", "15.4.1", "15.5.0"}

var identityPlatformVersions = map[Platform][]string{
	PlatformMacOS:   append([]string{"14.5.0", "14.6.1", "14.7.1", "14.7.2", "14.7.4"}, macOSSequoiaVersions...),
	PlatformWindows: {"10.0.0", "13.0.0", "14.0.0", "15.0.0", "19.0.0"},
	PlatformLinux:   {"6.1.0", "6.5.0", "6.6.0", "6.8.0", "6.11.0", "6.12.0", "6.14.0"},
}

// On macOS, fingerprint-chromium selects Apple M4 for even seeds (seed % 2 == 0)
// and Apple M2 for odd seeds (seed % 2 == 1). Match authentic core counts and
// minimum shipping OS versions (Apple M4 launched with macOS 15 Sequoia).
var macOSM4Cores = []int{10, 14, 16}

// Apple M2 models by memory bucket, the bucket being {8, 16, 32}[seed % 3]:
// 8 GiB is the 8-core chip, 32 GiB only the 10/12-core Pro and Max chips.
var macOSM2MemoryCores = map[int][]int{
	0: {8},
	1: {8, 10, 12},
	2: {10, 12},
}

type screenPreset struct {
	width          int
	height         int
	availTop       int
	availBottom    int
	availBottomMin int
	availBottomMax int
	dpr            float64
	windowWidth    int
	windowHeight   int
}

var identityScreenPresets = map[Platform][]screenPreset{
	PlatformMacOS: {
		{width: 1440, height: 900, availTop: 25, availBottom: 60, availBottomMin: 56, availBottomMax: 68, dpr: 2, windowWidth: 1366, windowHeight: 800},
		{width: 1470, height: 956, availTop: 37, availBottom: 64, availBottomMin: 58, availBottomMax: 72, dpr: 2, windowWidth: 1380, windowHeight: 836},
		{width: 1512, height: 982, availTop: 37, availBottom: 70, availBottomMin: 60, availBottomMax: 76, dpr: 2, windowWidth: 1440, windowHeight: 860},
		{width: 1680, height: 1050, availTop: 25, availBottom: 65, availBottomMin: 58, availBottomMax: 74, dpr: 2, windowWidth: 1440, windowHeight: 900},
		{width: 1710, height: 1112, availTop: 37, availBottom: 68, availBottomMin: 60, availBottomMax: 76, dpr: 2, windowWidth: 1480, windowHeight: 920},
		{width: 1728, height: 1117, availTop: 37, availBottom: 70, availBottomMin: 60, availBottomMax: 76, dpr: 2, windowWidth: 1512, windowHeight: 940},
		{width: 1920, height: 1080, availTop: 25, availBottom: 64, availBottomMin: 56, availBottomMax: 72, dpr: 1, windowWidth: 1520, windowHeight: 920},
		{width: 2048, height: 1280, availTop: 25, availBottom: 68, availBottomMin: 60, availBottomMax: 76, dpr: 2, windowWidth: 1600, windowHeight: 980},
		{width: 2560, height: 1440, availTop: 25, availBottom: 70, availBottomMin: 60, availBottomMax: 78, dpr: 2, windowWidth: 1680, windowHeight: 1020},
	},
	PlatformWindows: {
		{width: 1600, height: 900, availTop: 0, availBottom: 40, dpr: 1, windowWidth: 1366, windowHeight: 800},
		{width: 1680, height: 1050, availTop: 0, availBottom: 48, dpr: 1, windowWidth: 1440, windowHeight: 900},
		{width: 1707, height: 960, availTop: 0, availBottom: 48, dpr: 1.5, windowWidth: 1420, windowHeight: 840},
		{width: 1920, height: 1080, availTop: 0, availBottom: 40, dpr: 1, windowWidth: 1440, windowHeight: 900},
		{width: 1920, height: 1080, availTop: 0, availBottom: 48, dpr: 1.25, windowWidth: 1536, windowHeight: 920},
		{width: 1920, height: 1080, availTop: 0, availBottom: 48, dpr: 2, windowWidth: 1520, windowHeight: 920},
		{width: 1920, height: 1200, availTop: 0, availBottom: 48, dpr: 1, windowWidth: 1560, windowHeight: 960},
		{width: 2048, height: 1152, availTop: 0, availBottom: 48, dpr: 1.25, windowWidth: 1600, windowHeight: 960},
		{width: 2560, height: 1440, availTop: 0, availBottom: 48, dpr: 1, windowWidth: 1600, windowHeight: 960},
		{width: 2560, height: 1600, availTop: 0, availBottom: 48, dpr: 1.5, windowWidth: 1660, windowHeight: 1000},
	},
	PlatformLinux: {
		{width: 1440, height: 900, availTop: 27, availBottom: 0, dpr: 1, windowWidth: 1366, windowHeight: 820},
		{width: 1600, height: 900, availTop: 27, availBottom: 0, dpr: 1, windowWidth: 1380, windowHeight: 820},
		{width: 1680, height: 1050, availTop: 27, availBottom: 0, dpr: 1, windowWidth: 1440, windowHeight: 880},
		{width: 1920, height: 1080, availTop: 27, availBottom: 0, dpr: 1, windowWidth: 1440, windowHeight: 900},
		{width: 1920, height: 1080, availTop: 32, availBottom: 0, dpr: 2, windowWidth: 1500, windowHeight: 920},
		{width: 1920, height: 1200, availTop: 27, availBottom: 0, dpr: 1, windowWidth: 1560, windowHeight: 960},
		{width: 2560, height: 1440, availTop: 27, availBottom: 0, dpr: 1, windowWidth: 1640, windowHeight: 980},
	},
}

// EnsureOptions configures per-profile random fingerprint identity generation.
type EnsureOptions struct {
	// HostOS selects the spoofed platform ("macos", "windows", "linux").
	// Empty defaults to the current host OS.
	HostOS Platform
	// Brand selects the spoofed browser brand. Empty defaults to BrandChrome.
	Brand Brand
	// CollisionRoots lists directories whose immediate child directories (and
	// themselves) are scanned to avoid seed/viewport collisions and balance
	// diversity across sibling profiles.
	CollisionRoots []string
	// LegacyIdentityFile optionally specifies a legacy camelCase identity file
	// (such as ".tulai-fingerprint.json") checked when .browserkernel.json has
	// no seed yet, migrating existing profiles and avoiding collisions with
	// untouched legacy siblings.
	LegacyIdentityFile string

	pickSeed            func(attempt int) uint32
	pickCores           func(attempt int) int
	pickScreen          func(attempt int) ScreenSpec
	pickPlatformVersion func(attempt int) string
}

// DefaultScreenFor returns a deterministic desktop screen preset for platform
// and seed, used when an existing profile carries no explicit ScreenSpec.
func DefaultScreenFor(platform Platform, seed uint32) ScreenSpec {
	presets := identityScreenPresets[platform]
	if len(presets) == 0 {
		presets = identityScreenPresets[PlatformLinux]
	}
	p := presets[int(seed%uint32(len(presets)))]
	return ScreenSpec{
		Width:        p.width,
		Height:       p.height,
		AvailTop:     p.availTop,
		AvailBottom:  p.availBottom,
		DPR:          p.dpr,
		WindowWidth:  p.windowWidth,
		WindowHeight: p.windowHeight,
	}
}

// GenerateProfile creates a random, hardware-coherent, diversity-balanced
// FingerprintSpec that avoids seed and viewport collisions with siblings.
func GenerateProfile(hostOS Platform, brand Brand, siblings []FingerprintSpec) (FingerprintSpec, error) {
	return generateProfile(resolveHostOS(hostOS), resolveBrand(brand), siblings, EnsureOptions{})
}

// EnsureProfile returns the profile's persisted FingerprintSpec in dir,
// generating and atomically persisting a new random identity into
// .browserkernel.json on first use while preserving any saved user preferences.
// Corrupted identity files fail closed instead of silently regenerating.
func EnsureProfile(ctx context.Context, dir string, opts EnsureOptions) (FingerprintSpec, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return FingerprintSpec{}, err
	}
	if strings.TrimSpace(dir) == "" || filepath.Clean(dir) == "." {
		return FingerprintSpec{}, errors.New("缺少浏览器资料目录")
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return FingerprintSpec{}, err
	}
	hostOS := resolveHostOS(opts.HostOS)
	brand := resolveBrand(opts.Brand)

	// Fast path: already initialized profile in .browserkernel.json.
	existing, _, err := readProfile(absDir)
	if err != nil {
		return FingerprintSpec{}, fmt.Errorf("账号 Fingerprint 指纹文件无法读取：%w", err)
	}
	if existing.Seed > 0 {
		completed, err := completeExistingIdentity(existing, hostOS, brand)
		if err != nil || reflect.DeepEqual(completed, existing) {
			return completed, err
		}
	}

	if err := os.MkdirAll(absDir, 0o700); err != nil {
		return FingerprintSpec{}, err
	}
	parentDir := filepath.Dir(absDir)
	if err := os.MkdirAll(parentDir, 0o700); err != nil {
		return FingerprintSpec{}, err
	}

	// Lock order is strictly parent generation lock -> profile edit lock.
	genLock, err := acquireFileLock(ctx, filepath.Join(parentDir, genLockFile), nil)
	if err != nil {
		return FingerprintSpec{}, err
	}
	defer ReleaseInstallLock(genLock)

	profileLock, err := acquireFileLock(ctx, filepath.Join(absDir, ".browserkernel.lock"), nil)
	if err != nil {
		return FingerprintSpec{}, err
	}
	defer ReleaseInstallLock(profileLock)

	current, original, err := readProfile(absDir)
	if err != nil {
		return FingerprintSpec{}, fmt.Errorf("账号 Fingerprint 指纹文件无法读取：%w", err)
	}
	if current.Seed > 0 {
		completed, err := completeExistingIdentity(current, hostOS, brand)
		if err != nil {
			return FingerprintSpec{}, err
		}
		if !reflect.DeepEqual(completed, current) {
			if err := saveProfileLocked(ctx, absDir, completed, original); err != nil {
				return FingerprintSpec{}, err
			}
		}
		return completed, nil
	}

	// Migrate an existing legacy identity file into .browserkernel.json if present.
	if legacyName := strings.TrimSpace(opts.LegacyIdentityFile); legacyName != "" {
		legacy, found, err := readLegacyIdentityFile(absDir, legacyName)
		if err != nil {
			return FingerprintSpec{}, err
		}
		if found {
			merged := mergeIdentity(current, legacy)
			if err := saveProfileLocked(ctx, absDir, merged, original); err != nil {
				return FingerprintSpec{}, err
			}
			return merged, nil
		}
	}

	siblings, err := readSiblingIdentities(absDir, opts.CollisionRoots, strings.TrimSpace(opts.LegacyIdentityFile))
	if err != nil {
		return FingerprintSpec{}, err
	}
	generated, err := generateProfile(hostOS, brand, siblings, opts)
	if err != nil {
		return FingerprintSpec{}, err
	}
	merged := mergeIdentity(current, generated)
	if err := saveProfileLocked(ctx, absDir, merged, original); err != nil {
		return FingerprintSpec{}, err
	}
	return merged, nil
}

func resolveHostOS(hostOS Platform) Platform {
	if p := NormalizePlatform(string(hostOS)); p != "" {
		return p
	}
	if p := NormalizePlatform(runtime.GOOS); p != "" {
		return p
	}
	return PlatformLinux
}

func resolveBrand(brand Brand) Brand {
	if b := NormalizeBrand(string(brand)); b != "" {
		return b
	}
	return BrandChrome
}

func validateVersionString(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > 32 || !identityVersionRe.MatchString(trimmed) {
		return "", errors.New("账号 Fingerprint 指纹文件不完整，请恢复原文件后重试")
	}
	return trimmed, nil
}

func defaultCoresFor(platform Platform, seed uint32) int {
	choices := validCoreChoices(platform, seed)
	return choices[int(seed%uint32(len(choices)))]
}

func validCoreChoices(platform Platform, seed uint32) []int {
	if platform == PlatformMacOS {
		if seed%2 == 0 {
			return macOSM4Cores
		}
		return macOSM2MemoryCores[int(seed%3)]
	}
	if list := identityCoreChoices[platform]; len(list) > 0 {
		return list
	}
	return identityCoreChoices[PlatformLinux]
}

func completeExistingIdentity(f FingerprintSpec, hostOS Platform, brand Brand) (FingerprintSpec, error) {
	if f.Seed == 0 || f.Seed > maxGeneratedSeed {
		return FingerprintSpec{}, errors.New("账号 Fingerprint 指纹文件不完整，请恢复原文件后重试")
	}
	if f.Platform == "" {
		f.Platform = hostOS
	} else if p := NormalizePlatform(string(f.Platform)); p != "" {
		f.Platform = p
	} else {
		return FingerprintSpec{}, errors.New("账号 Fingerprint 指纹文件不完整，请恢复原文件后重试")
	}
	if f.Brand == "" {
		f.Brand = brand
	} else if b := NormalizeBrand(string(f.Brand)); b != "" {
		f.Brand = b
	} else {
		return FingerprintSpec{}, errors.New("账号 Fingerprint 指纹文件不完整，请恢复原文件后重试")
	}
	if f.PlatformVersion != "" {
		v, err := validateVersionString(f.PlatformVersion)
		if err != nil {
			return FingerprintSpec{}, err
		}
		f.PlatformVersion = v
	}
	if f.BrandVersion != "" {
		v, err := validateVersionString(f.BrandVersion)
		if err != nil {
			return FingerprintSpec{}, err
		}
		f.BrandVersion = v
	}
	if f.HardwareConcurrency == 0 {
		f.HardwareConcurrency = defaultCoresFor(f.Platform, f.Seed)
	} else if f.HardwareConcurrency < 2 || f.HardwareConcurrency > 64 {
		return FingerprintSpec{}, errors.New("账号 Fingerprint 指纹文件不完整，请恢复原文件后重试")
	}
	if f.Screen == nil {
		scr := DefaultScreenFor(f.Platform, f.Seed)
		f.Screen = &scr
	} else if err := f.Screen.Validate(); err != nil {
		return FingerprintSpec{}, errors.New("账号 Fingerprint 指纹文件不完整，请恢复原文件后重试")
	}
	return f, nil
}

func mergeIdentity(base, identity FingerprintSpec) FingerprintSpec {
	out := base
	out.Seed = identity.Seed
	out.Platform = identity.Platform
	out.PlatformVersion = identity.PlatformVersion
	out.Brand = identity.Brand
	out.BrandVersion = identity.BrandVersion
	out.HardwareConcurrency = identity.HardwareConcurrency
	if identity.Screen != nil {
		scr := *identity.Screen
		if scr.WindowX != nil {
			x := *scr.WindowX
			scr.WindowX = &x
		}
		if scr.WindowY != nil {
			y := *scr.WindowY
			scr.WindowY = &y
		}
		out.Screen = &scr
	}
	return out
}

type legacyScreenJSON struct {
	Width        *int     `json:"width"`
	Height       *int     `json:"height"`
	AvailTop     *int     `json:"availTop"`
	AvailBottom  *int     `json:"availBottom"`
	DPR          *float64 `json:"dpr"`
	WindowWidth  *int     `json:"windowWidth"`
	WindowHeight *int     `json:"windowHeight"`
	WindowX      *int     `json:"windowX"`
	WindowY      *int     `json:"windowY"`
}

type legacyIdentityJSON struct {
	Version             *int              `json:"version"`
	Seed                *int64            `json:"seed"`
	Platform            *string           `json:"platform"`
	PlatformVersion     *string           `json:"platformVersion"`
	Brand               *string           `json:"brand"`
	BrandVersion        *string           `json:"brandVersion"`
	HardwareConcurrency *int              `json:"hardwareConcurrency"`
	Screen              *legacyScreenJSON `json:"screen"`
}

func readLegacyIdentityFile(dir, name string) (FingerprintSpec, bool, error) {
	if !filepath.IsLocal(name) || filepath.Base(name) != name {
		return FingerprintSpec{}, false, fmt.Errorf("无效的历史指纹文件名: %q", name)
	}
	root, err := os.OpenRoot(dir)
	if errors.Is(err, os.ErrNotExist) {
		return FingerprintSpec{}, false, nil
	}
	if err != nil {
		return FingerprintSpec{}, false, err
	}
	defer root.Close()
	data, err := root.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return FingerprintSpec{}, false, nil
	}
	if err != nil {
		return FingerprintSpec{}, false, fmt.Errorf("账号 Fingerprint 指纹文件无法读取：%w", err)
	}
	var raw legacyIdentityJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return FingerprintSpec{}, false, fmt.Errorf("账号 Fingerprint 指纹文件无法读取：%w", err)
	}
	if raw.Version == nil || *raw.Version != 1 ||
		raw.Seed == nil || *raw.Seed < 1 || *raw.Seed > int64(maxGeneratedSeed) ||
		raw.Platform == nil ||
		raw.Brand == nil || strings.TrimSpace(*raw.Brand) == "" ||
		raw.HardwareConcurrency == nil || *raw.HardwareConcurrency < 2 || *raw.HardwareConcurrency > 64 {
		return FingerprintSpec{}, false, errors.New("账号 Fingerprint 指纹文件不完整，请恢复原文件后重试")
	}
	plat := Platform(*raw.Platform)
	if plat != PlatformMacOS && plat != PlatformWindows && plat != PlatformLinux {
		return FingerprintSpec{}, false, errors.New("账号 Fingerprint 指纹文件不完整，请恢复原文件后重试")
	}
	brand := NormalizeBrand(*raw.Brand)
	if brand == "" {
		return FingerprintSpec{}, false, errors.New("账号 Fingerprint 指纹文件不完整，请恢复原文件后重试")
	}
	spec := FingerprintSpec{
		Seed:                uint32(*raw.Seed),
		Platform:            plat,
		Brand:               brand,
		HardwareConcurrency: *raw.HardwareConcurrency,
	}
	if raw.PlatformVersion != nil {
		v, err := validateVersionString(*raw.PlatformVersion)
		if err != nil {
			return FingerprintSpec{}, false, err
		}
		spec.PlatformVersion = v
	}
	if raw.BrandVersion != nil {
		v, err := validateVersionString(*raw.BrandVersion)
		if err != nil {
			return FingerprintSpec{}, false, err
		}
		spec.BrandVersion = v
	}
	if raw.Screen == nil {
		scr := DefaultScreenFor(spec.Platform, spec.Seed)
		spec.Screen = &scr
	} else {
		if raw.Screen.Width == nil || raw.Screen.Height == nil ||
			raw.Screen.AvailTop == nil || raw.Screen.AvailBottom == nil ||
			raw.Screen.DPR == nil || raw.Screen.WindowWidth == nil || raw.Screen.WindowHeight == nil {
			return FingerprintSpec{}, false, errors.New("账号 Fingerprint 指纹文件不完整，请恢复原文件后重试")
		}
		scr := ScreenSpec{
			Width:        *raw.Screen.Width,
			Height:       *raw.Screen.Height,
			AvailTop:     *raw.Screen.AvailTop,
			AvailBottom:  *raw.Screen.AvailBottom,
			DPR:          *raw.Screen.DPR,
			WindowWidth:  *raw.Screen.WindowWidth,
			WindowHeight: *raw.Screen.WindowHeight,
			WindowX:      raw.Screen.WindowX,
			WindowY:      raw.Screen.WindowY,
		}
		if err := scr.Validate(); err != nil {
			return FingerprintSpec{}, false, errors.New("账号 Fingerprint 指纹文件不完整，请恢复原文件后重试")
		}
		spec.Screen = &scr
	}
	return spec, true, nil
}

type siblingEntry struct {
	dir   string
	label string
}

func collectSiblingDirs(profileDir string, collisionRoots []string) ([]siblingEntry, error) {
	roots := make([]string, 0, 1+len(collisionRoots))
	roots = append(roots, filepath.Dir(profileDir))
	for _, r := range collisionRoots {
		if trimmed := strings.TrimSpace(r); trimmed != "" {
			roots = append(roots, trimmed)
		}
	}
	seenRoots := make(map[string]struct{}, len(roots))
	seenDirs := make(map[string]struct{})
	var dirs []siblingEntry
	addDir := func(dir, label string) {
		abs, err := filepath.Abs(dir)
		if err != nil {
			abs = filepath.Clean(dir)
		}
		if abs == profileDir {
			return
		}
		if _, ok := seenDirs[abs]; ok {
			return
		}
		seenDirs[abs] = struct{}{}
		dirs = append(dirs, siblingEntry{dir: abs, label: label})
	}
	for _, rawRoot := range roots {
		absRoot, err := filepath.Abs(rawRoot)
		if err != nil {
			absRoot = filepath.Clean(rawRoot)
		}
		if _, ok := seenRoots[absRoot]; ok {
			continue
		}
		seenRoots[absRoot] = struct{}{}
		entries, err := os.ReadDir(absRoot)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || isNotDirError(err) {
				continue
			}
			return nil, err
		}
		label := filepath.Base(absRoot)
		if label == "" {
			label = absRoot
		}
		addDir(absRoot, label)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			addDir(filepath.Join(absRoot, entry.Name()), entry.Name())
		}
	}
	return dirs, nil
}

func isNotDirError(err error) bool {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if info, statErr := os.Stat(pathErr.Path); statErr == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func readSiblingIdentities(profileDir string, collisionRoots []string, legacyFile string) ([]FingerprintSpec, error) {
	dirs, err := collectSiblingDirs(profileDir, collisionRoots)
	if err != nil {
		return nil, err
	}
	var existing []FingerprintSpec
	for _, entry := range dirs {
		spec, raw, err := readProfile(entry.dir)
		if err != nil {
			return nil, fmt.Errorf("读取已有 Fingerprint 环境指纹失败（%s）：%w", entry.label, err)
		}
		if raw != nil && spec.Seed > 0 {
			if spec.Platform != "" && spec.Screen == nil {
				scr := DefaultScreenFor(spec.Platform, spec.Seed)
				spec.Screen = &scr
			}
			existing = append(existing, spec)
			continue
		}
		if legacyFile != "" {
			legacy, found, err := readLegacyIdentityFile(entry.dir, legacyFile)
			if err != nil {
				return nil, fmt.Errorf("读取已有 Fingerprint 环境指纹失败（%s）：%w", entry.label, err)
			}
			if found {
				existing = append(existing, legacy)
			}
		}
	}
	return existing, nil
}

func cryptoRandInt(minVal, maxExclusive int) (int, error) {
	if maxExclusive <= minVal {
		return minVal, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxExclusive-minVal)))
	if err != nil {
		return 0, err
	}
	return minVal + int(n.Int64()), nil
}

func pickLeastUsed[T comparable](choices []T, counts map[T]int) (T, error) {
	minCount := math.MaxInt
	for _, choice := range choices {
		if c := counts[choice]; c < minCount {
			minCount = c
		}
	}
	tied := make([]T, 0, len(choices))
	for _, choice := range choices {
		if counts[choice] == minCount {
			tied = append(tied, choice)
		}
	}
	idx, err := cryptoRandInt(0, len(tied))
	if err != nil {
		var zero T
		return zero, err
	}
	return tied[idx], nil
}

func randomStep(minVal, maxVal, step int) (int, error) {
	if maxVal <= minVal {
		return minVal, nil
	}
	span := (maxVal - minVal) / step
	n, err := cryptoRandInt(0, span+1)
	if err != nil {
		return 0, err
	}
	return minVal + n*step, nil
}

func screenPresetKey(s ScreenSpec) string {
	return fmt.Sprintf("%dx%d@%g", s.Width, s.Height, s.DPR)
}

func presetKey(p screenPreset) string {
	return fmt.Sprintf("%dx%d@%g", p.width, p.height, p.dpr)
}

func viewportKey(platform Platform, s ScreenSpec) string {
	return fmt.Sprintf("%s|%dx%d@%g|%dx%d", platform, s.Width, s.Height, s.DPR, s.WindowWidth, s.WindowHeight)
}

func gcdInt(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func lcmInt(a, b int) int {
	return (a / gcdInt(a, b)) * b
}

// generateBalancedSeed picks a random 31-bit seed spanning both odd and even
// integers while balancing fingerprint-chromium's deterministic GPU
// (seed % gpuMod), deviceMemory (seed % 3), and brand patch (seed % 4) buckets.
func generateBalancedSeed(platform Platform, siblings []FingerprintSpec) (uint32, error) {
	gpuMod := identityGPUModulus[platform]
	if gpuMod == 0 {
		gpuMod = 2
	}
	gpuCounts := make(map[int]int, gpuMod)
	memCounts := make(map[int]int, 3)
	patchCounts := make(map[int]int, 4)
	for _, item := range siblings {
		if item.Platform != platform || item.Seed == 0 {
			continue
		}
		gpuCounts[int(item.Seed)%gpuMod]++
		memCounts[int(item.Seed)%3]++
		patchCounts[int(item.Seed)%4]++
	}
	gpuChoices := make([]int, gpuMod)
	for i := range gpuChoices {
		gpuChoices[i] = i
	}
	targetGPU, err := pickLeastUsed(gpuChoices, gpuCounts)
	if err != nil {
		return 0, err
	}
	// deviceMemory is {8, 16, 32}[seed % 3]. No Apple M4 machine ships with
	// 8 GiB, so even (M4) seeds only draw the two larger buckets.
	memChoices := []int{0, 1, 2}
	if platform == PlatformMacOS && targetGPU%2 == 0 {
		memChoices = []int{1, 2}
	}
	targetMem, err := pickLeastUsed(memChoices, memCounts)
	if err != nil {
		return 0, err
	}
	patchChoices := make([]int, 0, 2)
	for _, p := range []int{0, 1, 2, 3} {
		if p%2 == targetGPU%2 {
			patchChoices = append(patchChoices, p)
		}
	}
	targetPatch, err := pickLeastUsed(patchChoices, patchCounts)
	if err != nil {
		return 0, err
	}

	period := lcmInt(gpuMod, 12)
	residue := 0
	for r := 0; r < period; r++ {
		if r%gpuMod == targetGPU && r%3 == targetMem && r%4 == targetPatch {
			residue = r
			break
		}
	}
	minK := (int(minGeneratedSeed) - residue + period - 1) / period
	if minK < 0 {
		minK = 0
	}
	maxK := (int(maxGeneratedSeed) - residue) / period
	k, err := cryptoRandInt(minK, maxK+1)
	if err != nil {
		return 0, err
	}
	return uint32(k*period + residue), nil
}

func generateDynamicScreen(platform Platform, platformVersion string, siblings []FingerprintSpec) (ScreenSpec, error) {
	screenList := identityScreenPresets[platform]
	if len(screenList) == 0 {
		screenList = identityScreenPresets[PlatformLinux]
	}
	presetCounts := make(map[string]int, len(screenList))
	for _, item := range siblings {
		if item.Platform != platform || item.Screen == nil {
			continue
		}
		presetCounts[screenPresetKey(*item.Screen)]++
	}
	presetKeys := make([]string, len(screenList))
	for i, p := range screenList {
		presetKeys[i] = presetKey(p)
	}
	chosenKey, err := pickLeastUsed(presetKeys, presetCounts)
	if err != nil {
		return ScreenSpec{}, err
	}
	matching := make([]screenPreset, 0, len(screenList))
	for _, p := range screenList {
		if presetKey(p) == chosenKey {
			matching = append(matching, p)
		}
	}
	idx, err := cryptoRandInt(0, len(matching))
	if err != nil {
		return ScreenSpec{}, err
	}
	preset := matching[idx]

	availTop := preset.availTop
	availBottom := preset.availBottom
	if platform == PlatformWindows {
		if platformVersion == "10.0.0" {
			availBottom = 40
		} else {
			availBottom = 48
		}
	} else if platform == PlatformMacOS && preset.availBottomMin > 0 && preset.availBottomMax >= preset.availBottomMin {
		availBottom, err = cryptoRandInt(preset.availBottomMin, preset.availBottomMax+1)
		if err != nil {
			return ScreenSpec{}, err
		}
	}
	availH := preset.height - availTop - availBottom
	minWinW := min(preset.width, 1320)
	maxWinW := min(preset.width, max(minWinW, min(1800, preset.width-16)))
	minWinH := min(availH, 760)
	maxWinH := min(availH, max(minWinH, min(1080, availH-12)))
	windowWidth, err := randomStep(minWinW, maxWinW, 2)
	if err != nil {
		return ScreenSpec{}, err
	}
	windowHeight, err := randomStep(minWinH, maxWinH, 2)
	if err != nil {
		return ScreenSpec{}, err
	}
	maxX := max(0, preset.width-windowWidth)
	maxY := max(availTop, preset.height-availBottom-windowHeight)
	windowX, err := cryptoRandInt(0, maxX+1)
	if err != nil {
		return ScreenSpec{}, err
	}
	windowY, err := cryptoRandInt(availTop, maxY+1)
	if err != nil {
		return ScreenSpec{}, err
	}
	return ScreenSpec{
		Width:        preset.width,
		Height:       preset.height,
		AvailTop:     availTop,
		AvailBottom:  availBottom,
		DPR:          preset.dpr,
		WindowWidth:  windowWidth,
		WindowHeight: windowHeight,
		WindowX:      &windowX,
		WindowY:      &windowY,
	}, nil
}

func generateProfile(hostOS Platform, brand Brand, siblings []FingerprintSpec, opts EnsureOptions) (FingerprintSpec, error) {
	defaultVersionList := identityPlatformVersions[hostOS]
	if len(defaultVersionList) == 0 {
		defaultVersionList = identityPlatformVersions[PlatformLinux]
	}

	siblingSeeds := make(map[uint32]struct{}, len(siblings))
	siblingViewports := make(map[string]struct{}, len(siblings))
	for _, s := range siblings {
		if s.Seed > 0 {
			siblingSeeds[s.Seed] = struct{}{}
		}
		if s.Screen != nil {
			siblingViewports[viewportKey(s.Platform, *s.Screen)] = struct{}{}
		}
	}

	for attempt := 0; attempt < 128; attempt++ {
		var (
			seed uint32
			err  error
		)
		if opts.pickSeed != nil {
			seed = opts.pickSeed(attempt)
		} else {
			seed, err = generateBalancedSeed(hostOS, siblings)
			if err != nil {
				return FingerprintSpec{}, err
			}
		}
		if seed == 0 || seed > maxGeneratedSeed {
			return FingerprintSpec{}, errors.New("生成的 Fingerprint 种子无效，请重试")
		}
		if _, exists := siblingSeeds[seed]; exists {
			continue
		}

		versionList := defaultVersionList
		if hostOS == PlatformMacOS && seed%2 == 0 {
			versionList = macOSSequoiaVersions
		}
		var platformVersion string
		if opts.pickPlatformVersion != nil {
			platformVersion = opts.pickPlatformVersion(attempt)
		} else {
			versionCounts := make(map[string]int, len(versionList))
			for _, item := range siblings {
				if item.Platform == hostOS && item.PlatformVersion != "" {
					versionCounts[item.PlatformVersion]++
				}
			}
			platformVersion, err = pickLeastUsed(versionList, versionCounts)
			if err != nil {
				return FingerprintSpec{}, err
			}
		}

		coresList := validCoreChoices(hostOS, seed)
		var cores int
		if opts.pickCores != nil {
			cores = opts.pickCores(attempt)
		} else {
			coreCounts := make(map[int]int, len(coresList))
			for _, item := range siblings {
				if item.Platform == hostOS && item.HardwareConcurrency > 0 {
					coreCounts[item.HardwareConcurrency]++
				}
			}
			cores, err = pickLeastUsed(coresList, coreCounts)
			if err != nil {
				return FingerprintSpec{}, err
			}
		}

		var screen ScreenSpec
		if opts.pickScreen != nil {
			screen = opts.pickScreen(attempt)
		} else {
			screen, err = generateDynamicScreen(hostOS, platformVersion, siblings)
			if err != nil {
				return FingerprintSpec{}, err
			}
		}

		candidate := FingerprintSpec{
			Seed:                seed,
			Platform:            hostOS,
			PlatformVersion:     platformVersion,
			Brand:               brand,
			HardwareConcurrency: cores,
			Screen:              &screen,
		}
		validated, err := completeExistingIdentity(candidate, hostOS, brand)
		if err != nil {
			return FingerprintSpec{}, err
		}
		if opts.pickScreen == nil && attempt < 96 {
			if _, collides := siblingViewports[viewportKey(hostOS, *validated.Screen)]; collides {
				continue
			}
		}
		// Keep slices sorted / clean even though DisableSpoofing is empty.
		_ = slices.Clone(validated.DisableSpoofing)
		return validated, nil
	}
	return FingerprintSpec{}, errors.New("无法生成与现有环境不同的 Fingerprint 指纹，请重试")
}
