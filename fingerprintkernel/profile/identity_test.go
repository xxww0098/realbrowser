package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestEnsureProfileCreatesPersistsAndAvoidsCollisions(t *testing.T) {
	root := t.TempDir()
	sampleScreen := ScreenSpec{
		Width:        1512,
		Height:       982,
		AvailTop:     37,
		AvailBottom:  70,
		DPR:          2,
		WindowWidth:  1440,
		WindowHeight: 860,
	}
	p1 := filepath.Join(root, "account-1")
	first, err := EnsureProfile(t.Context(), p1, EnsureOptions{
		HostOS:              PlatformMacOS,
		pickSeed:            func(int) uint32 { return 200002 },
		pickCores:           func(int) int { return 10 },
		pickScreen:          func(int) ScreenSpec { return sampleScreen },
		pickPlatformVersion: func(int) string { return "15.3.1" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Seed != 200002 || first.Platform != PlatformMacOS || first.PlatformVersion != "15.3.1" ||
		first.Brand != BrandChrome || first.HardwareConcurrency != 10 || first.Screen == nil || *first.Screen != sampleScreen {
		t.Fatalf("unexpected first profile: %+v screen=%+v", first, first.Screen)
	}

	// Reload returns the exact saved identity without calling generators.
	reloaded, err := EnsureProfile(t.Context(), p1, EnsureOptions{
		HostOS: PlatformWindows,
		pickSeed: func(int) uint32 {
			t.Fatal("must not regenerate existing profile")
			return 0
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Seed != first.Seed || reloaded.Platform != first.Platform || reloaded.PlatformVersion != first.PlatformVersion ||
		reloaded.Brand != first.Brand || reloaded.HardwareConcurrency != first.HardwareConcurrency ||
		reloaded.Screen == nil || *reloaded.Screen != *first.Screen {
		t.Fatalf("reloaded profile drifted: got %+v want %+v", reloaded, first)
	}

	// Sibling profile skips colliding seed and claims next distinct seed.
	p2 := filepath.Join(root, "account-2")
	candidates := []uint32{200002, 200002, 400005}
	second, err := EnsureProfile(t.Context(), p2, EnsureOptions{
		HostOS: PlatformMacOS,
		pickSeed: func(i int) uint32 {
			if i < len(candidates) {
				return candidates[i]
			}
			return 600007
		},
		pickCores: func(int) int { return 12 },
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Seed != 400005 || second.HardwareConcurrency != 12 || second.PlatformVersion == first.PlatformVersion {
		t.Fatalf("unexpected sibling profile: %+v", second)
	}

	// Cross-root collision avoidance via CollisionRoots.
	extraRoot := filepath.Join(root, "other-root")
	p3 := filepath.Join(extraRoot, "account-3")
	third, err := EnsureProfile(t.Context(), p3, EnsureOptions{
		HostOS:         PlatformWindows,
		CollisionRoots: []string{root},
		pickSeed: func(i int) uint32 {
			if i == 0 {
				return 400005
			}
			return 800009
		},
		pickCores: func(int) int { return 8 },
	})
	if err != nil {
		t.Fatal(err)
	}
	if third.Seed != 800009 || third.Platform != PlatformWindows || !slices.Contains(identityPlatformVersions[PlatformWindows], third.PlatformVersion) {
		t.Fatalf("unexpected cross-root profile: %+v", third)
	}
}

func assertMacHardware(t *testing.T, id FingerprintSpec) {
	t.Helper()
	if id.Seed%2 == 0 {
		if !strings.HasPrefix(id.PlatformVersion, "15.") {
			t.Fatalf("Apple M4 seed must use macOS 15.x Sequoia, got %q", id.PlatformVersion)
		}
		if !slices.Contains([]int{10, 14, 16}, id.HardwareConcurrency) {
			t.Fatalf("Apple M4 cores mismatch: %d", id.HardwareConcurrency)
		}
		if id.Seed%3 == 0 {
			t.Fatalf("Apple M4 has no 8 GiB model, got memory bucket %d", id.Seed%3)
		}
		return
	}
	if !slices.Contains([]int{8, 10, 12}, id.HardwareConcurrency) {
		t.Fatalf("Apple M2 cores mismatch: %d", id.HardwareConcurrency)
	}
	switch id.Seed % 3 {
	case 0:
		if id.HardwareConcurrency != 8 {
			t.Fatalf("8 GiB Apple M2 must be 8-core, got %d", id.HardwareConcurrency)
		}
	case 2:
		if !slices.Contains([]int{10, 12}, id.HardwareConcurrency) {
			t.Fatalf("32 GiB Apple M2 must be 10/12-core, got %d", id.HardwareConcurrency)
		}
	}
}

func TestEnsureProfileBalancesDiversityAndHardwareCoherence(t *testing.T) {
	root := t.TempDir()

	// macOS: 6 sibling accounts must span both GPU parities (M4/M2), all 3 memory
	// buckets, all 4 Chrome patch buckets, distinct OS versions, screen presets, and viewports.
	macSeeds := map[uint32]bool{}
	macGPU := map[uint32]bool{}
	macMem := map[uint32]bool{}
	macPatch := map[uint32]bool{}
	macVer := map[string]bool{}
	macCores := map[int]bool{}
	macPresets := map[string]bool{}
	macViewports := map[string]bool{}

	for i := 0; i < 6; i++ {
		id, err := EnsureProfile(t.Context(), filepath.Join(root, "mac", fmt.Sprintf("acc-%d", i)), EnsureOptions{HostOS: PlatformMacOS})
		if err != nil {
			t.Fatal(err)
		}
		if id.Seed < minGeneratedSeed || id.Seed > maxGeneratedSeed {
			t.Fatalf("seed out of range: %d", id.Seed)
		}
		if id.Screen == nil || id.Screen.WindowX == nil || id.Screen.WindowY == nil {
			t.Fatalf("missing generated screen geometry: %+v", id.Screen)
		}
		if err := id.Screen.Validate(); err != nil {
			t.Fatalf("invalid generated screen: %v", err)
		}
		assertMacHardware(t, id)
		macSeeds[id.Seed] = true
		macGPU[id.Seed%2] = true
		macMem[id.Seed%3] = true
		macPatch[id.Seed%4] = true
		macVer[id.PlatformVersion] = true
		macCores[id.HardwareConcurrency] = true
		macPresets[screenPresetKey(*id.Screen)] = true
		macViewports[viewportKey(id.Platform, *id.Screen)] = true
	}
	if len(macSeeds) != 6 || len(macGPU) != 2 || len(macMem) != 3 || len(macPatch) != 4 ||
		len(macVer) != 6 || len(macCores) < 4 || len(macPresets) != 6 || len(macViewports) != 6 {
		t.Fatalf("macOS diversity shortfall: seeds=%d gpu=%d mem=%d patch=%d ver=%d cores=%d presets=%d viewports=%d",
			len(macSeeds), len(macGPU), len(macMem), len(macPatch), len(macVer), len(macCores), len(macPresets), len(macViewports))
	}

	// Additional macOS hardware coherence sweep across 24 accounts.
	macSweepRoot := filepath.Join(root, "mac-sweep")
	for i := 0; i < 24; i++ {
		id, err := EnsureProfile(t.Context(), filepath.Join(macSweepRoot, fmt.Sprintf("acc-%d", i)), EnsureOptions{HostOS: PlatformMacOS})
		if err != nil {
			t.Fatal(err)
		}
		assertMacHardware(t, id)
	}

	// Windows: 10 sibling accounts must span all 10 D3D11 GPU slots, all 3 memory
	// buckets, all 4 patch buckets, all 5 OS versions, and match Win10/Win11 taskbar height.
	winSeeds := map[uint32]bool{}
	winGPU := map[uint32]bool{}
	winMem := map[uint32]bool{}
	winPatch := map[uint32]bool{}
	winVer := map[string]bool{}
	for i := 0; i < 10; i++ {
		id, err := EnsureProfile(t.Context(), filepath.Join(root, "win", fmt.Sprintf("acc-%d", i)), EnsureOptions{HostOS: PlatformWindows})
		if err != nil {
			t.Fatal(err)
		}
		wantTaskbar := 48
		if id.PlatformVersion == "10.0.0" {
			wantTaskbar = 40
		}
		if id.Screen == nil || id.Screen.AvailBottom != wantTaskbar {
			t.Fatalf("Windows %s taskbar = %+v, want %d", id.PlatformVersion, id.Screen, wantTaskbar)
		}
		winSeeds[id.Seed] = true
		winGPU[id.Seed%10] = true
		winMem[id.Seed%3] = true
		winPatch[id.Seed%4] = true
		winVer[id.PlatformVersion] = true
	}
	if len(winSeeds) != 10 || len(winGPU) != 10 || len(winMem) != 3 || len(winPatch) != 4 || len(winVer) != 5 {
		t.Fatalf("Windows diversity shortfall: seeds=%d gpu=%d mem=%d patch=%d ver=%d",
			len(winSeeds), len(winGPU), len(winMem), len(winPatch), len(winVer))
	}
}

func TestEnsureProfilePreservesSavedPreferencesAndUnknownFields(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "account-prefs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	const initial = `{"lang":"en-GB","timezone":"Europe/London","window_width":640,"window_height":480,"color_scheme":"dark","future_flag":true}`
	if err := os.WriteFile(filepath.Join(dir, profileFile), []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := EnsureProfile(t.Context(), dir, EnsureOptions{HostOS: PlatformMacOS})
	if err != nil {
		t.Fatal(err)
	}
	if got.Seed == 0 || got.Screen == nil {
		t.Fatalf("identity not generated: %+v", got)
	}
	if got.Lang != "en-GB" || got.Timezone != "Europe/London" || got.WindowWidth != 640 || got.WindowHeight != 480 || got.ColorScheme != ColorSchemeDark {
		t.Fatalf("saved preferences lost: %+v", got)
	}
	raw, err := os.ReadFile(filepath.Join(dir, profileFile))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["future_flag"] != true {
		t.Fatalf("unknown field lost after EnsureProfile: %s", raw)
	}
}

func TestEnsureProfileMigratesLegacyIdentityAndScansLegacySiblings(t *testing.T) {
	root := t.TempDir()
	const legacyName = ".tulai-fingerprint.json"

	// Legacy profile with pre-existing .browserkernel.json preferences (Seed == 0)
	// and legacy v1 identity without platformVersion or windowX/windowY.
	p1 := filepath.Join(root, "legacy-account")
	if err := os.MkdirAll(p1, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := WriteProfile(p1, FingerprintSpec{Lang: "zh-CN", Timezone: "Asia/Shanghai"}); err != nil {
		t.Fatal(err)
	}
	legacyJSON := `{"version":1,"seed":200002,"platform":"macos","brand":"Chrome","hardwareConcurrency":10,"screen":{"width":1512,"height":982,"availTop":37,"availBottom":70,"dpr":2,"windowWidth":1440,"windowHeight":860}}`
	if err := os.WriteFile(filepath.Join(p1, legacyName), []byte(legacyJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	migrated, err := EnsureProfile(t.Context(), p1, EnsureOptions{
		HostOS:             PlatformWindows,
		LegacyIdentityFile: legacyName,
	})
	if err != nil {
		t.Fatal(err)
	}
	if migrated.Seed != 200002 || migrated.Platform != PlatformMacOS || migrated.HardwareConcurrency != 10 ||
		migrated.Lang != "zh-CN" || migrated.Timezone != "Asia/Shanghai" || migrated.Screen == nil ||
		migrated.Screen.Width != 1512 || migrated.Screen.WindowX != nil {
		t.Fatalf("unexpected migrated profile: %+v screen=%+v", migrated, migrated.Screen)
	}

	// Untouched legacy sibling (no .browserkernel.json yet) must still be seen during collision scanning.
	p2 := filepath.Join(root, "untouched-legacy")
	if err := os.MkdirAll(p2, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p2, legacyName), []byte(`{"version":1,"seed":400005,"platform":"macos","brand":"Chrome","hardwareConcurrency":8}`), 0o600); err != nil {
		t.Fatal(err)
	}
	p3 := filepath.Join(root, "new-account")
	created, err := EnsureProfile(t.Context(), p3, EnsureOptions{
		HostOS:             PlatformMacOS,
		LegacyIdentityFile: legacyName,
		pickSeed: func(i int) uint32 {
			if i == 0 {
				return 400005
			}
			return 600007
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Seed != 600007 {
		t.Fatalf("failed to avoid untouched legacy sibling seed: got %d want 600007", created.Seed)
	}
}

func TestEnsureProfileFailsClosedOnCorruptedFiles(t *testing.T) {
	const legacyName = ".tulai-fingerprint.json"
	for name, body := range map[string]string{
		"broken-json":        `{broken-json`,
		"negative-seed":      `{"seed":-5,"platform":"macos","brand":"Chrome","hardware_concurrency":8}`,
		"bad-screen":         `{"seed":200002,"platform":"macos","brand":"Chrome","hardware_concurrency":8,"screen":{"width":800,"height":600,"avail_top":0,"avail_bottom":0,"dpr":1,"window_width":1440,"window_height":900}}`,
		"bad-platform-ver":   `{"seed":200002,"platform":"macos","platform_version":"bad-ver","brand":"Chrome","hardware_concurrency":8}`,
		"oob-window-pos":     `{"seed":200002,"platform":"macos","brand":"Chrome","hardware_concurrency":8,"screen":{"width":1512,"height":982,"avail_top":37,"avail_bottom":70,"dpr":2,"window_width":1440,"window_height":860,"window_x":999,"window_y":40}}`,
		"partial-window-pos": `{"seed":200002,"platform":"macos","brand":"Chrome","hardware_concurrency":8,"screen":{"width":1512,"height":982,"avail_top":37,"avail_bottom":70,"dpr":2,"window_width":1440,"window_height":860,"window_x":20}}`,
	} {
		t.Run("profile-"+name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "acc")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, profileFile), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := EnsureProfile(t.Context(), dir, EnsureOptions{HostOS: PlatformMacOS}); err == nil {
				t.Fatalf("accepted corrupted .browserkernel.json (%s)", name)
			}
		})
	}

	for name, body := range map[string]string{
		"broken-json":   `{broken-json`,
		"negative-seed": `{"version":1,"seed":-5,"platform":"macos","brand":"Chrome","hardwareConcurrency":8}`,
		"bad-screen":    `{"version":1,"seed":200002,"platform":"macos","brand":"Chrome","hardwareConcurrency":8,"screen":{"width":800,"height":600,"availTop":0,"availBottom":0,"dpr":1,"windowWidth":1440,"windowHeight":900}}`,
		"partial-pos":   `{"version":1,"seed":200002,"platform":"macos","brand":"Chrome","hardwareConcurrency":8,"screen":{"width":1512,"height":982,"availTop":37,"availBottom":70,"dpr":2,"windowWidth":1440,"windowHeight":860,"windowX":20}}`,
	} {
		t.Run("legacy-"+name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "acc")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, legacyName), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := EnsureProfile(t.Context(), dir, EnsureOptions{HostOS: PlatformMacOS, LegacyIdentityFile: legacyName}); err == nil {
				t.Fatalf("accepted corrupted legacy identity (%s)", name)
			}
		})
	}
}

func TestEnsureProfileConcurrentHelperProcess(t *testing.T) {
	profile := os.Getenv("FPKERNEL_HELPER_PROFILE")
	if profile == "" {
		return
	}
	hostOS := Platform(os.Getenv("FPKERNEL_HELPER_OS"))
	var roots []string
	if r := os.Getenv("FPKERNEL_HELPER_ROOT"); r != "" {
		roots = []string{r}
	}
	spec, err := EnsureProfile(context.Background(), profile, EnsureOptions{
		HostOS:         hostOS,
		CollisionRoots: roots,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out, _ := json.Marshal(spec)
	fmt.Println(string(out))
	os.Exit(0)
}

func TestEnsureProfileHandlesConcurrentSameAndSiblingProcesses(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runWorker := func(profile, hostOS, collisionRoot string) (FingerprintSpec, error) {
		cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestEnsureProfileConcurrentHelperProcess$")
		cmd.Env = append(os.Environ(),
			"FPKERNEL_HELPER_PROFILE="+profile,
			"FPKERNEL_HELPER_OS="+hostOS,
			"FPKERNEL_HELPER_ROOT="+collisionRoot,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return FingerprintSpec{}, fmt.Errorf("%v: %s", err, out)
		}
		var spec FingerprintSpec
		if err := json.Unmarshal([]byte(strings.TrimSpace(string(out))), &spec); err != nil {
			return FingerprintSpec{}, fmt.Errorf("parse %q: %w", out, err)
		}
		return spec, nil
	}

	root := t.TempDir()

	// 6 concurrent processes racing on the same fresh profile converge to the exact same identity.
	sameRoot := filepath.Join(root, "same")
	sameProfile := filepath.Join(sameRoot, "account-1")
	sameResults := make([]FingerprintSpec, 6)
	sameErrs := make([]error, 6)
	var wg sync.WaitGroup
	for i := range sameResults {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sameResults[idx], sameErrs[idx] = runWorker(sameProfile, string(PlatformMacOS), sameRoot)
		}(i)
	}
	wg.Wait()
	for i, err := range sameErrs {
		if err != nil {
			t.Fatalf("same-profile worker %d failed: %v", i, err)
		}
		if sameResults[i].Seed != sameResults[0].Seed || sameResults[i].PlatformVersion != sameResults[0].PlatformVersion ||
			sameResults[i].HardwareConcurrency != sameResults[0].HardwareConcurrency {
			t.Fatalf("same-profile workers diverged: [0]=%+v [%d]=%+v", sameResults[0], i, sameResults[i])
		}
	}

	// 6 concurrent sibling processes serialize cleanly across all diversity buckets.
	sibRoot := filepath.Join(root, "siblings")
	sibResults := make([]FingerprintSpec, 6)
	sibErrs := make([]error, 6)
	for i := range sibResults {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sibResults[idx], sibErrs[idx] = runWorker(filepath.Join(sibRoot, fmt.Sprintf("acc-%d", idx)), string(PlatformMacOS), sibRoot)
		}(i)
	}
	wg.Wait()
	seeds := map[uint32]bool{}
	gpus := map[uint32]bool{}
	mems := map[uint32]bool{}
	patches := map[uint32]bool{}
	vers := map[string]bool{}
	presets := map[string]bool{}
	for i, err := range sibErrs {
		if err != nil {
			t.Fatalf("sibling worker %d failed: %v", i, err)
		}
		item := sibResults[i]
		assertMacHardware(t, item)
		seeds[item.Seed] = true
		gpus[item.Seed%2] = true
		mems[item.Seed%3] = true
		patches[item.Seed%4] = true
		vers[item.PlatformVersion] = true
		presets[screenPresetKey(*item.Screen)] = true
	}
	if len(seeds) != 6 || len(gpus) != 2 || len(mems) != 3 || len(patches) != 4 || len(vers) != 6 || len(presets) != 6 {
		t.Fatalf("concurrent sibling diversity shortfall: seeds=%d gpus=%d mems=%d patches=%d vers=%d presets=%d",
			len(seeds), len(gpus), len(mems), len(patches), len(vers), len(presets))
	}
}
