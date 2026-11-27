package engine

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ulikunitz/xz"
)

func TestKernelBinaryProbeCapabilitiesAndCrossPlatformLayouts(t *testing.T) {
	// 1. ParseChromiumMajorVersion cases.
	for in, want := range map[string]int{
		"Chromium 148.0.7778.215":     148,
		"Google Chrome 139.0.7258.66": 139,
		"Chromium 144":                144,
		"unrecognized version string": 0,
		"":                            0,
	} {
		if got := ParseChromiumMajorVersion(in); got != want {
			t.Errorf("ParseChromiumMajorVersion(%q) = %d, want %d", in, got, want)
		}
	}

	tmp := t.TempDir()

	// 2. macOS .app bundle: trampoline binary delegates to Chromium Framework;
	// place switch tokens spanning a 256 KiB chunk boundary to test overlap window.
	appBin := filepath.Join(tmp, "Chromium.app", "Contents", "MacOS", "Chromium")
	fwPath := filepath.Join(tmp, "Chromium.app", "Contents", "Frameworks", "Chromium Framework.framework", "Versions", "148.0.7778.215", "Chromium Framework")
	if err := os.MkdirAll(filepath.Dir(appBin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(fwPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Put "fingerprint-platform" straddling the exact 256*1024 byte boundary!
	var fwBuf bytes.Buffer
	fwBuf.Write(bytes.Repeat([]byte{'A'}, 256*1024-8))
	fwBuf.WriteString("fingerprint-platform")
	fwBuf.Write(bytes.Repeat([]byte{'B'}, 1024))
	fwBuf.WriteString("disable-spoofing")
	fwBuf.Write(bytes.Repeat([]byte{'C'}, 1024))
	if err := os.WriteFile(fwPath, fwBuf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := &KernelManager{
		GOOS:       "darwin",
		GOARCH:     "arm64",
		RunVersion: func(context.Context, string) (string, error) { return "Chromium 148.0.7778.215", nil },
	}
	info := mgr.Probe(appBin)
	if info.Kind != KindFingerprintChromium {
		t.Fatalf("macOS patched Chromium.app Kind = %q, want fingerprint-chromium", info.Kind)
	}
	if !info.Capabilities.BinaryVerified || !info.Capabilities.FingerprintPatched ||
		!info.Capabilities.SupportsDisableSpoofing || !info.Capabilities.SupportsDeviceMemory148 ||
		info.Capabilities.IsFallbackChrome || info.Capabilities.MajorVersion != 148 {
		t.Fatalf("macOS patched capabilities mismatch: %+v", info.Capabilities)
	}

	// 3. Windows versioned <ver>\chrome.dll layout without patch -> detected as unpatched fallback!
	winExe := filepath.Join(tmp, "WinChrome", "chrome.exe")
	winDll := filepath.Join(tmp, "WinChrome", "145.0.0.1", "chrome.dll")
	if err := os.MkdirAll(filepath.Dir(winDll), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(winExe, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(winDll, bytes.Repeat([]byte{'Z'}, 8192), 0o644); err != nil {
		t.Fatal(err)
	}
	winMgr := &KernelManager{
		GOOS:       "windows",
		GOARCH:     "amd64",
		RunVersion: func(context.Context, string) (string, error) { return "Chromium 145.0.0.1", nil },
	}
	winInfo := winMgr.Probe(winExe)
	if !winInfo.Capabilities.IsFallbackChrome || winInfo.Capabilities.FingerprintPatched {
		t.Fatalf("unpatched Windows chrome.dll must be flagged IsFallbackChrome: %+v", winInfo.Capabilities)
	}

	// 4. Small (<4KB) shell wrapper and compressed Linux .AppImage must NOT be falsely flagged IsFallbackChrome.
	smallBin := filepath.Join(tmp, "fingerprint-stub.sh")
	if err := os.WriteFile(smallBin, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	smallInfo := winMgr.Probe(smallBin)
	if smallInfo.Capabilities.IsFallbackChrome {
		t.Fatalf("small test stub should not be falsely marked IsFallbackChrome: %+v", smallInfo.Capabilities)
	}

	appImg := filepath.Join(tmp, "ungoogled-chromium-148.0.7778.215-x86_64.AppImage")
	if err := os.WriteFile(appImg, bytes.Repeat([]byte{'Q'}, 8192), 0o755); err != nil {
		t.Fatal(err)
	}
	linuxMgr := &KernelManager{
		GOOS:       "linux",
		GOARCH:     "amd64",
		RunVersion: func(context.Context, string) (string, error) { return "Chromium 148.0.7778.215", nil },
	}
	appImgInfo := linuxMgr.Probe(appImg)
	if appImgInfo.Capabilities.IsFallbackChrome {
		t.Fatalf("compressed Linux .AppImage must not be falsely flagged IsFallbackChrome: %+v", appImgInfo.Capabilities)
	}

	// 5. macOS Versions/Current priority: stale unpatched Versions/140.0.0.1 must NOT shadow patched Versions/Current!
	multiMacBin := filepath.Join(tmp, "Upgraded.app", "Contents", "MacOS", "Chromium")
	oldMacFW := filepath.Join(tmp, "Upgraded.app", "Contents", "Frameworks", "Chromium Framework.framework", "Versions", "140.0.0.1", "Chromium Framework")
	newMacFW := filepath.Join(tmp, "Upgraded.app", "Contents", "Frameworks", "Chromium Framework.framework", "Versions", "148.0.7778.215", "Chromium Framework")
	curMacLink := filepath.Join(tmp, "Upgraded.app", "Contents", "Frameworks", "Chromium Framework.framework", "Versions", "Current")
	for _, p := range []string{multiMacBin, oldMacFW, newMacFW} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(multiMacBin, []byte("#!/bin/sh\n"), 0o755)
	_ = os.WriteFile(oldMacFW, bytes.Repeat([]byte{'X'}, 8192), 0o644) // unpatched stale version
	_ = os.WriteFile(newMacFW, append(bytes.Repeat([]byte{'Y'}, 4096), []byte("fingerprint-platform disable-spoofing")...), 0o644)
	if err := os.Symlink("148.0.7778.215", curMacLink); err != nil {
		t.Fatal(err)
	}
	upInfo := mgr.Probe(multiMacBin)
	if !upInfo.Capabilities.BinaryVerified || upInfo.Capabilities.IsFallbackChrome {
		t.Fatalf("macOS Versions/Current was shadowed by stale 140.0.0.1: %+v", upInfo.Capabilities)
	}

	// 6. Windows newest versioned <ver>\chrome.dll priority: stale 140.0.0.1 must NOT shadow patched 148.0.7778.215!
	multiWinExe := filepath.Join(tmp, "UpgradedWin", "chrome.exe")
	oldWinDLL := filepath.Join(tmp, "UpgradedWin", "140.0.0.1", "chrome.dll")
	newWinDLL := filepath.Join(tmp, "UpgradedWin", "148.0.7778.215", "chrome.dll")
	for _, p := range []string{multiWinExe, oldWinDLL, newWinDLL} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(multiWinExe, []byte("stub"), 0o644)
	_ = os.WriteFile(oldWinDLL, bytes.Repeat([]byte{'X'}, 8192), 0o644)
	_ = os.WriteFile(newWinDLL, append(bytes.Repeat([]byte{'Y'}, 4096), []byte("fingerprint-platform disable-spoofing")...), 0o644)
	upWin := winMgr.Probe(multiWinExe)
	if !upWin.Capabilities.BinaryVerified || upWin.Capabilities.IsFallbackChrome {
		t.Fatalf("Windows newest versioned chrome.dll was shadowed by stale 140.0.0.1: %+v", upWin.Capabilities)
	}

	// 7. ResolvePreferred promotes verified patched kernel over an unpatched non-configured fallback.
	unpatchedEnv := filepath.Join(tmp, "env-unpatched-chrome")
	patchedManagedDir := filepath.Join(tmp, "data", "kernels", "fingerprint-chromium")
	patchedManagedBin := filepath.Join(patchedManagedDir, "chromium")
	_ = os.MkdirAll(patchedManagedDir, 0o755)
	_ = os.WriteFile(unpatchedEnv, bytes.Repeat([]byte{'U'}, 8192), 0o755)
	_ = os.WriteFile(patchedManagedBin, append(bytes.Repeat([]byte{'P'}, 4096), []byte("fingerprint-platform")...), 0o755)
	prefMgr := &KernelManager{
		DataDir: filepath.Join(tmp, "data"),
		GOOS:    "linux",
		GOARCH:  "amd64",
		Getenv: func(k string) string {
			if k == EnvChromiumPath {
				return unpatchedEnv
			}
			return ""
		},
		RunVersion: func(context.Context, string) (string, error) { return "Chromium 148.0.7778.215", nil },
	}
	best, err := prefMgr.ResolvePreferred()
	if err != nil {
		t.Fatalf("ResolvePreferred: %v", err)
	}
	if best.Path != patchedManagedBin {
		t.Fatalf("ResolvePreferred = %q, want patched managed binary %q", best.Path, patchedManagedBin)
	}
	// Explicit user-configured path is still respected even if unpatched.
	prefMgr.SetConfiguredPath(unpatchedEnv)
	explicit, err := prefMgr.ResolvePreferred()
	if err != nil || explicit.Path != unpatchedEnv {
		t.Fatalf("explicit ConfiguredPath should be respected: got %+v err=%v", explicit, err)
	}

	// 8. Real macOS com.apple.quarantine xattr detection + ClearMacQuarantine verification.
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("xattr"); err == nil {
			qBundle := filepath.Join(tmp, "Quarantined.app")
			qBin := filepath.Join(qBundle, "Contents", "MacOS", "Chromium")
			_ = os.MkdirAll(filepath.Dir(qBin), 0o755)
			_ = os.WriteFile(qBin, []byte("#!/bin/sh\n"), 0o755)
			if err := exec.Command("xattr", "-w", "com.apple.quarantine", "0081;65f00000;Safari;", qBundle).Run(); err == nil {
				if !detectMacQuarantine(qBin) {
					t.Fatalf("detectMacQuarantine(%q) = false, want true", qBin)
				}
				if err := ClearMacQuarantine(context.Background(), qBin); err != nil {
					t.Fatalf("ClearMacQuarantine: %v", err)
				}
				if detectMacQuarantine(qBin) {
					t.Fatalf("detectMacQuarantine(%q) still true after ClearMacQuarantine", qBin)
				}
			}
		}
	}
}

func TestReleaseSHA256VerificationTarXZAndDMGInstall(t *testing.T) {
	// 1. ParseSHA256Checksum formats & mismatch rejection.
	payload := []byte("fake-fingerprint-chromium-binary-payload-v148")
	sumBytes := sha256.Sum256(payload)
	wantHex := hex.EncodeToString(sumBytes[:])
	wrongHex := strings.Repeat("a", 64)

	cases := []struct {
		name  string
		raw   string
		asset string
		want  string
		ok    bool
	}{
		{"bare", wantHex + "\n", "fp.tar.xz", wantHex, true},
		{"gnu", wantHex + "  fp.tar.xz\n", "fp.tar.xz", wantHex, true},
		{"gnu_star", wantHex + " *./dist/fp.tar.xz\n", "fp.tar.xz", wantHex, true},
		{"bsd", "SHA256 (fp.tar.xz) = " + strings.ToUpper(wantHex) + "\n", "fp.tar.xz", wantHex, true},
		{"manifest_multi", wrongHex + "  other.zip\n" + wantHex + "  fp.tar.xz\n", "fp.tar.xz", wantHex, true},
		{"wrong_filename", wantHex + "  other.zip\n", "fp.tar.xz", "", false},
		{"invalid_hex", "not-a-hash  fp.tar.xz\n", "fp.tar.xz", "", false},
	}
	for _, tc := range cases {
		got, err := ParseSHA256Checksum(tc.raw, tc.asset)
		if tc.ok && (err != nil || got != tc.want) {
			t.Errorf("%s: got (%q,%v), want (%q,nil)", tc.name, got, err, tc.want)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s: expected error, got %q", tc.name, got)
		}
	}

	// 2. SelectReleaseAsset attaches ChecksumURL for sidecar .sha256 or SHA256SUMS.
	assets := []ReleaseAsset{
		{Name: "148.0.7778.215_linux_x64.tar.xz", DownloadURL: "https://example.invalid/fp.tar.xz", Size: 100},
		{Name: "148.0.7778.215_linux_x64.tar.xz.sha256", DownloadURL: "https://example.invalid/fp.tar.xz.sha256", Size: 80},
	}
	sel := SelectReleaseAsset(assets, "linux", "amd64")
	if sel == nil || sel.ChecksumURL != "https://example.invalid/fp.tar.xz.sha256" {
		t.Fatalf("SelectReleaseAsset did not attach ChecksumURL: %+v", sel)
	}

	// 3. DownloadRelease verifies SHA-256 on success AND rejects tampered downloads cleanly.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.bin":
			_, _ = w.Write(payload)
		case "/ok.bin.sha256":
			_, _ = w.Write([]byte(wantHex + "  ok.bin\n"))
		case "/bad.bin.sha256":
			_, _ = w.Write([]byte(wrongHex + "  ok.bin\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	mgr := &KernelManager{HTTPClient: srv.Client()}
	dlDir := t.TempDir()
	okPath, err := mgr.DownloadRelease(context.Background(), ReleaseAsset{
		Name:        "ok.bin",
		DownloadURL: srv.URL + "/ok.bin",
		ChecksumURL: srv.URL + "/ok.bin.sha256",
	}, dlDir, nil)
	if err != nil {
		t.Fatalf("DownloadRelease(valid sha256): %v", err)
	}
	if gotBytes, err := os.ReadFile(okPath); err != nil || !bytes.Equal(gotBytes, payload) {
		t.Fatalf("downloaded file mismatch: err=%v len=%d", err, len(gotBytes))
	}

	badDir := t.TempDir()
	if _, err := mgr.DownloadRelease(context.Background(), ReleaseAsset{
		Name:        "ok.bin",
		DownloadURL: srv.URL + "/ok.bin",
		ChecksumURL: srv.URL + "/bad.bin.sha256",
	}, badDir, nil); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch error, got %v", err)
	}
	if ents, _ := os.ReadDir(badDir); len(ents) != 0 {
		t.Fatalf("tampered download left files behind: %v", ents)
	}

	// 4. ExtractTarXZ & InstallReleaseArchive (valid extraction + slip/symlink/hardlink rejection).
	writeTarXZ := func(t *testing.T, path string, entries []struct {
		hdr  tar.Header
		body []byte
	}) {
		t.Helper()
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		xw, err := xz.NewWriter(f)
		if err != nil {
			_ = f.Close()
			t.Fatal(err)
		}
		tw := tar.NewWriter(xw)
		for _, e := range entries {
			hdr := e.hdr
			if hdr.Typeflag == tar.TypeReg {
				hdr.Size = int64(len(e.body))
			}
			if err := tw.WriteHeader(&hdr); err != nil {
				t.Fatal(err)
			}
			if len(e.body) > 0 {
				if _, err := tw.Write(e.body); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := xw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}

	tarTmp := t.TempDir()
	goodTarXZ := filepath.Join(tarTmp, "148.0.7778.215_linux_x64.tar.xz")
	writeTarXZ(t, goodTarXZ, []struct {
		hdr  tar.Header
		body []byte
	}{
		{hdr: tar.Header{Name: "148.0.7778.215_linux_x64/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{hdr: tar.Header{Name: "148.0.7778.215_linux_x64/chrome", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("#!/bin/sh\necho ok\n")},
		{hdr: tar.Header{Name: "148.0.7778.215_linux_x64/lib.so.1", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("lib")},
		{hdr: tar.Header{Name: "148.0.7778.215_linux_x64/lib.so", Typeflag: tar.TypeSymlink, Linkname: "lib.so.1"}},
	})
	outTarDir := filepath.Join(tarTmp, "installed")
	exe, err := InstallReleaseArchive(context.Background(), goodTarXZ, outTarDir)
	if err != nil {
		t.Fatalf("InstallReleaseArchive(.tar.xz): %v", err)
	}
	wantExe := filepath.Join(outTarDir, "148.0.7778.215_linux_x64", "chrome")
	if exe != wantExe {
		t.Fatalf("ExtractTarXZ exe = %q, want %q", exe, wantExe)
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(exe); err != nil || fi.Mode().Perm()&0o111 == 0 {
			t.Fatalf("extracted chrome missing +x: mode=%v err=%v", fi.Mode(), err)
		}
	}

	// Malicious tar.xz archives must be rejected.
	slipArchives := []struct {
		name    string
		entries []struct {
			hdr  tar.Header
			body []byte
		}
	}{
		{
			name: "dotdot",
			entries: []struct {
				hdr  tar.Header
				body []byte
			}{{hdr: tar.Header{Name: "../escape.txt", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("bad")}},
		},
		{
			name: "absolute",
			entries: []struct {
				hdr  tar.Header
				body []byte
			}{{hdr: tar.Header{Name: "/tmp/escape.txt", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("bad")}},
		},
		{
			name: "symlink_escape",
			entries: []struct {
				hdr  tar.Header
				body []byte
			}{{hdr: tar.Header{Name: "dir/badlink", Typeflag: tar.TypeSymlink, Linkname: "../../outside"}}},
		},
		{
			name: "hardlink_escape",
			entries: []struct {
				hdr  tar.Header
				body []byte
			}{{hdr: tar.Header{Name: "dir/badhard", Typeflag: tar.TypeLink, Linkname: "../outside"}}},
		},
	}
	for _, sc := range slipArchives {
		badPath := filepath.Join(tarTmp, sc.name+".tar.xz")
		writeTarXZ(t, badPath, sc.entries)
		if _, err := ExtractTarXZ(badPath, filepath.Join(tarTmp, "bad-"+sc.name)); err == nil {
			t.Errorf("ExtractTarXZ(%s) unexpectedly succeeded", sc.name)
		}
	}

	// 5. macOS real .dmg auto-install & .zip quarantine clearing (darwin only).
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("hdiutil"); err == nil {
			srcRoot := filepath.Join(tarTmp, "dmg-src")
			appBin := filepath.Join(srcRoot, "Chromium.app", "Contents", "MacOS", "Chromium")
			fwPath := filepath.Join(srcRoot, "Chromium.app", "Contents", "Frameworks", "Chromium Framework.framework", "Versions", "148.0.0.1", "Chromium Framework")
			_ = os.MkdirAll(filepath.Dir(appBin), 0o755)
			_ = os.MkdirAll(filepath.Dir(fwPath), 0o755)
			_ = os.WriteFile(appBin, []byte("#!/bin/sh\necho Chromium 148.0.0.1\n"), 0o755)
			_ = os.WriteFile(fwPath, append(bytes.Repeat([]byte{'A'}, 4096), []byte("fingerprint-platform disable-spoofing")...), 0o644)

			dmgPath := filepath.Join(tarTmp, "fp-macos-arm64.dmg")
			if out, err := exec.Command("hdiutil", "create", "-volname", "FPTest", "-srcfolder", srcRoot, "-ov", "-format", "UDZO", dmgPath).CombinedOutput(); err != nil {
				t.Fatalf("hdiutil create: %v (%s)", err, string(out))
			}
			_ = exec.Command("xattr", "-w", "com.apple.quarantine", "0081;65f00000;Safari;", dmgPath).Run()

			dmgDest := filepath.Join(tarTmp, "dmg-dest")
			installedExe, err := InstallReleaseArchive(context.Background(), dmgPath, dmgDest)
			if err != nil {
				t.Fatalf("InstallReleaseArchive(.dmg): %v", err)
			}
			wantMacExe := filepath.Join(dmgDest, "Chromium.app", "Contents", "MacOS", "Chromium")
			if installedExe != wantMacExe {
				t.Fatalf("InstallDMG exe = %q, want %q", installedExe, wantMacExe)
			}
			if detectMacQuarantine(installedExe) {
				t.Fatalf("InstallDMG did not clear com.apple.quarantine on %s", installedExe)
			}
			// Re-installing over an existing bundle must succeed atomically.
			if _, err := InstallDMG(context.Background(), dmgPath, dmgDest); err != nil {
				t.Fatalf("InstallDMG upgrade over existing bundle failed: %v", err)
			}

			// ExtractZip on macOS also clears quarantine if set.
			zipPath := filepath.Join(tarTmp, "fp-mac.zip")
			zf, err := os.Create(zipPath)
			if err != nil {
				t.Fatal(err)
			}
			zw := zip.NewWriter(zf)
			w, _ := zw.Create("Chromium.app/Contents/MacOS/Chromium")
			_, _ = w.Write([]byte("#!/bin/sh\necho ok\n"))
			_ = zw.Close()
			_ = zf.Close()
			zipExe, err := ExtractZip(zipPath, filepath.Join(tarTmp, "zip-dest"))
			if err != nil {
				t.Fatalf("ExtractZip(mac): %v", err)
			}
			if detectMacQuarantine(zipExe) {
				t.Fatalf("ExtractZip did not clear quarantine on %s", zipExe)
			}
		}
	}
}

func TestRound3KernelDiscoveryOrderingArchiveSecurityAndGitHubDigest(t *testing.T) {
	// 1. DetectAll prefers newer versioned managed subdir (149 > 144) and arbitrary macOS .app names.
	dataDir := t.TempDir()
	mgr := NewKernelManager(dataDir, "")
	mgr.GOOS = "linux"
	mgr.GOARCH = "amd64"
	mgr.Getenv = func(string) string { return "" }
	mgr.RunVersion = func(_ context.Context, bin string) (string, error) {
		if strings.Contains(bin, "149.") {
			return "Chromium 149.0.0.1", nil
		}
		return "Chromium 144.0.0.1", nil
	}
	md := mgr.ManagedKernelDir()
	oldBin := filepath.Join(md, "144.0.0.1_linux_x64", "chrome")
	newBin := filepath.Join(md, "149.0.0.1_linux_x64", "chrome")
	_ = os.MkdirAll(filepath.Dir(oldBin), 0o755)
	_ = os.MkdirAll(filepath.Dir(newBin), 0o755)
	patchedPayload := append(bytes.Repeat([]byte{'P'}, 4096), []byte("fingerprint-platform disable-spoofing")...)
	_ = os.WriteFile(oldBin, patchedPayload, 0o755)
	_ = os.WriteFile(newBin, patchedPayload, 0o755)

	best, err := mgr.ResolvePreferred()
	if err != nil || best.Path != newBin {
		t.Fatalf("ResolvePreferred = (%q, %v), want newer managed %q", best.Path, err, newBin)
	}

	// SelectPreferredKernel promotes patched fingerprint-chromium over unscanned generic Chromium.
	unscannedGeneric := KernelInfo{Path: "/usr/bin/chromium", Kind: KindChromium, Source: "env", Exists: true}
	patchedManaged := KernelInfo{
		Path:         newBin,
		Kind:         KindFingerprintChromium,
		Source:       "managed",
		Exists:       true,
		Capabilities: KernelCapabilities{FingerprintPatched: true, BinaryVerified: true},
	}
	if sel, ok := SelectPreferredKernel([]KernelInfo{unscannedGeneric, patchedManaged}); !ok || sel.Path != newBin {
		t.Fatalf("SelectPreferredKernel did not promote patched over unscanned generic Chromium: %+v", sel)
	}

	// Arbitrary macOS .app name inside ManagedKernelDir is discovered.
	macDir := t.TempDir()
	macMgr := NewKernelManager(macDir, "")
	macMgr.GOOS = "darwin"
	macMgr.GOARCH = "arm64"
	macMgr.Getenv = func(string) string { return "" }
	macMgr.RunVersion = func(context.Context, string) (string, error) { return "Chromium 148.0.0.1", nil }
	customAppExe := filepath.Join(macMgr.ManagedKernelDir(), "Ungoogled Chromium.app", "Contents", "MacOS", "Ungoogled Chromium")
	_ = os.MkdirAll(filepath.Dir(customAppExe), 0o755)
	_ = os.WriteFile(customAppExe, patchedPayload, 0o755)
	foundMac := macMgr.DetectAll()
	foundCustom := false
	for _, k := range foundMac {
		if k.Path == customAppExe {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Fatalf("DetectAll(darwin) missed custom managed .app bundle %s in %+v", customAppExe, foundMac)
	}

	// 2. GitHub native asset Digest ("sha256:<hex>"), release Body hash, and Windows backslash manifest paths.
	payload := []byte("github-native-digest-payload")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])

	if got, err := ParseSHA256Checksum(hexSum+`  dist\fp-win.zip`+"\n", "fp-win.zip"); err != nil || got != hexSum {
		t.Fatalf("ParseSHA256Checksum(windows path) = (%q, %v), want %q", got, err, hexSum)
	}

	selDig := SelectReleaseAsset([]ReleaseAsset{
		{Name: "148.0_linux_x64.tar.xz", DownloadURL: "https://example.invalid/a.tar.xz", Digest: "sha256:" + hexSum},
	}, "linux", "amd64")
	if selDig == nil || selDig.SHA256 != hexSum {
		t.Fatalf("SelectReleaseAsset did not normalize native GitHub Digest: %+v", selDig)
	}

	relBody, ok := pickCompatibleRelease([]ReleaseInfo{{
		TagName: "v148",
		Body:    "Checksums:\n- `148.0_linux_x64.tar.xz`: `" + hexSum + "`\n",
		Assets:  []ReleaseAsset{{Name: "148.0_linux_x64.tar.xz", DownloadURL: "https://example.invalid/a.tar.xz"}},
	}}, "linux", "amd64")
	if !ok || relBody.Matched == nil || relBody.Matched.SHA256 != hexSum {
		t.Fatalf("pickCompatibleRelease did not extract SHA256 from release body: %+v", relBody)
	}

	// DownloadRelease verifies asset.Digest directly even when SHA256 and ChecksumURL are empty.
	digSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer digSrv.Close()
	digMgr := &KernelManager{HTTPClient: digSrv.Client()}
	if _, err := digMgr.DownloadRelease(context.Background(), ReleaseAsset{
		Name:        "a.tar.xz",
		DownloadURL: digSrv.URL,
		Digest:      "sha256:" + strings.Repeat("b", 64),
	}, t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("DownloadRelease(bad native Digest) expected mismatch, got %v", err)
	}
	if _, err := digMgr.DownloadRelease(context.Background(), ReleaseAsset{
		Name:        "a.tar.xz",
		DownloadURL: digSrv.URL,
		Digest:      "sha256:" + hexSum,
	}, t.TempDir(), nil); err != nil {
		t.Fatalf("DownloadRelease(valid native Digest): %v", err)
	}

	// 3. Chained-symlink escape, decompression bomb budget, and read-only overwrite in ExtractTarXZ & ExtractZip.
	secTmp := t.TempDir()
	writeTarXZEntries := func(p string, entries []struct {
		hdr  tar.Header
		body []byte
	}) {
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		xw, err := xz.NewWriter(f)
		if err != nil {
			t.Fatal(err)
		}
		tw := tar.NewWriter(xw)
		for _, e := range entries {
			hdr := e.hdr
			if hdr.Typeflag == tar.TypeReg {
				hdr.Size = int64(len(e.body))
			}
			if err := tw.WriteHeader(&hdr); err != nil {
				t.Fatal(err)
			}
			if len(e.body) > 0 {
				_, _ = tw.Write(e.body)
			}
		}
		_ = tw.Close()
		_ = xw.Close()
		_ = f.Close()
	}

	// Chained symlink attack: a/b -> .. (resolves to dest), then a/b/c -> .. (escapes dest!).
	chainTar := filepath.Join(secTmp, "chain.tar.xz")
	writeTarXZEntries(chainTar, []struct {
		hdr  tar.Header
		body []byte
	}{
		{hdr: tar.Header{Name: "a/b", Typeflag: tar.TypeSymlink, Linkname: ".."}},
		{hdr: tar.Header{Name: "a/b/c", Typeflag: tar.TypeSymlink, Linkname: ".."}},
		{hdr: tar.Header{Name: "a/b/c/pwned.txt", Typeflag: tar.TypeReg, Mode: 0o644}, body: []byte("escape")},
	})
	if _, err := ExtractTarXZ(chainTar, filepath.Join(secTmp, "chain-dest")); err == nil {
		t.Fatalf("ExtractTarXZ allowed chained-symlink escape!")
	}
	if _, err := os.Stat(filepath.Join(secTmp, "pwned.txt")); err == nil {
		t.Fatalf("chained-symlink escape wrote outside destDir!")
	}

	// Read-only overwrite upgrade succeeds in ExtractTarXZ.
	roTar := filepath.Join(secTmp, "ro.tar.xz")
	writeTarXZEntries(roTar, []struct {
		hdr  tar.Header
		body []byte
	}{
		{hdr: tar.Header{Name: "ver/chrome", Typeflag: tar.TypeReg, Mode: 0o444}, body: []byte("v1")},
	})
	roDest := filepath.Join(secTmp, "ro-dest")
	if _, err := ExtractTarXZ(roTar, roDest); err != nil {
		t.Fatalf("ExtractTarXZ first pass: %v", err)
	}
	_ = os.Chmod(filepath.Join(roDest, "ver", "chrome"), 0o444)
	if _, err := ExtractTarXZ(roTar, roDest); err != nil {
		t.Fatalf("ExtractTarXZ overwrite over read-only 0444 file failed: %v", err)
	}

	// Decompression bomb budget enforcement in ExtractTarXZ and ExtractZip.
	origLimit := maxArchiveExtractBytes
	maxArchiveExtractBytes = 1024
	defer func() { maxArchiveExtractBytes = origLimit }()

	bombTar := filepath.Join(secTmp, "bomb.tar.xz")
	writeTarXZEntries(bombTar, []struct {
		hdr  tar.Header
		body []byte
	}{
		{hdr: tar.Header{Name: "big.bin", Typeflag: tar.TypeReg, Mode: 0o644}, body: bytes.Repeat([]byte{'Z'}, 4096)},
	})
	if _, err := ExtractTarXZ(bombTar, filepath.Join(secTmp, "bomb-tar-dest")); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("ExtractTarXZ bomb expected size limit error, got %v", err)
	}

	bombZip := filepath.Join(secTmp, "bomb.zip")
	bzf, _ := os.Create(bombZip)
	bzw := zip.NewWriter(bzf)
	bw, _ := bzw.Create("big.bin")
	_, _ = bw.Write(bytes.Repeat([]byte{'Z'}, 4096))
	_ = bzw.Close()
	_ = bzf.Close()
	if _, err := ExtractZip(bombZip, filepath.Join(secTmp, "bomb-zip-dest")); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("ExtractZip bomb expected size limit error, got %v", err)
	}
	maxArchiveExtractBytes = origLimit

	// 4. macOS InstallDMG edge cases: spaces in bundle name + Finder /Applications symlink + corrupt DMG.
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("hdiutil"); err == nil {
			src := filepath.Join(secTmp, "dmg-spaces-src")
			exePath := filepath.Join(src, "Fingerprint Chromium.app", "Contents", "MacOS", "Fingerprint Chromium")
			_ = os.MkdirAll(filepath.Dir(exePath), 0o755)
			_ = os.WriteFile(exePath, []byte("#!/bin/sh\necho ok\n"), 0o755)
			_ = os.Symlink("/Applications", filepath.Join(src, "Applications"))

			dmgSpaces := filepath.Join(secTmp, "fp spaces.dmg")
			if out, err := exec.Command("hdiutil", "create", "-volname", "FP Spaces", "-srcfolder", src, "-ov", "-format", "UDZO", dmgSpaces).CombinedOutput(); err != nil {
				t.Fatalf("hdiutil create(spaces): %v (%s)", err, string(out))
			}
			destSpaces := filepath.Join(secTmp, "dmg-spaces-dest")
			gotExe, err := InstallDMG(context.Background(), dmgSpaces, destSpaces)
			if err != nil {
				t.Fatalf("InstallDMG(spaces + Finder symlink): %v", err)
			}
			wantExe := filepath.Join(destSpaces, "Fingerprint Chromium.app", "Contents", "MacOS", "Fingerprint Chromium")
			if gotExe != wantExe {
				t.Fatalf("InstallDMG(spaces) exe = %q, want %q", gotExe, wantExe)
			}

			// Corrupt/truncated DMG must fail cleanly without panicking or hanging.
			corruptDMG := filepath.Join(secTmp, "corrupt.dmg")
			_ = os.WriteFile(corruptDMG, []byte("not-a-valid-dmg-image"), 0o644)
			if _, err := InstallDMG(context.Background(), corruptDMG, filepath.Join(secTmp, "corrupt-dest")); err == nil {
				t.Fatalf("InstallDMG(corrupt) unexpectedly succeeded")
			}
		}
	}
}

func TestSelectReleaseAssetMatrix(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "148.0.7778.215-1.1_macos_arm64.dmg", DownloadURL: "https://example/mac-arm.dmg"},
		{Name: "148.0.7778.215-1.1_macos_x64.dmg", DownloadURL: "https://example/mac-x64.dmg"},
		{Name: "148.0.7778.215-1.1_linux_x64.tar.xz", DownloadURL: "https://example/linux.tar.xz"},
		{Name: "148.0.7778.215-1.1_windows_x64.zip", DownloadURL: "https://example/win.zip"},
		{Name: "148.0.7778.215-1.1_windows_x64.zip.sha256", DownloadURL: "https://example/sha"},
		{Name: "chromedriver_macos_arm64.zip", DownloadURL: "https://example/drv"},
	}
	cases := []struct {
		goos, goarch, want string
	}{
		{"darwin", "arm64", "148.0.7778.215-1.1_macos_arm64.dmg"},
		{"darwin", "amd64", "148.0.7778.215-1.1_macos_x64.dmg"},
		{"linux", "amd64", "148.0.7778.215-1.1_linux_x64.tar.xz"},
		{"windows", "amd64", "148.0.7778.215-1.1_windows_x64.zip"},
	}
	for _, tc := range cases {
		got := SelectReleaseAsset(assets, tc.goos, tc.goarch)
		if got == nil || got.Name != tc.want {
			t.Errorf("SelectReleaseAsset(%s/%s) = %+v, want %q", tc.goos, tc.goarch, got, tc.want)
		}
	}
}

func TestKernelManagerDiscoveryAndFetchLatest(t *testing.T) {
	tmp := t.TempDir()
	fakeBin := filepath.Join(tmp, "kernels", "fingerprint-chromium", "chrome")
	if err := os.MkdirAll(filepath.Dir(fakeBin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ReleaseInfo{
			TagName: "148.0.7778.215",
			Name:    "Fingerprint Chromium 148",
			HTMLURL: UpstreamReleasesPage + "/tag/148.0.7778.215",
			Assets: []ReleaseAsset{
				{Name: "148.0.7778.215_linux_x64.tar.xz", DownloadURL: "http://example/linux.tar.xz", Size: 12345},
			},
		})
	}))
	defer ts.Close()

	mgr := &KernelManager{
		DataDir:      tmp,
		GOOS:         "linux",
		GOARCH:       "amd64",
		Getenv:       func(string) string { return "" },
		RunVersion:   func(context.Context, string) (string, error) { return "Chromium 148.0.7778.215 fingerprint", nil },
		HTTPClient:   ts.Client(),
		LatestAPIURL: ts.URL,
	}
	preferred, err := mgr.ResolvePreferred()
	if err != nil {
		t.Fatalf("ResolvePreferred: %v", err)
	}
	if preferred.Path != fakeBin || preferred.Kind != KindFingerprintChromium || preferred.Source != "managed" {
		t.Fatalf("preferred = %+v", preferred)
	}

	rel, err := mgr.FetchLatestRelease(context.Background())
	if err != nil {
		t.Fatalf("FetchLatestRelease: %v", err)
	}
	if rel.TagName != "148.0.7778.215" || rel.Matched == nil || rel.Matched.Name != "148.0.7778.215_linux_x64.tar.xz" {
		t.Fatalf("release = %+v", rel)
	}
}

func TestExtractZipAndZipSlipProtection(t *testing.T) {
	tmp := t.TempDir()

	// Good archive containing chrome executable.
	goodZip := filepath.Join(tmp, "good.zip")
	f, err := os.Create(goodZip)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("pkg/chrome")
	_, _ = w.Write([]byte("binary"))
	_ = zw.Close()
	_ = f.Close()

	exe, err := ExtractZip(goodZip, filepath.Join(tmp, "out"))
	if err != nil {
		t.Fatalf("ExtractZip(good): %v", err)
	}
	if !strings.HasSuffix(exe, filepath.Join("pkg", "chrome")) {
		t.Fatalf("exe = %q", exe)
	}

	// Malicious zip-slip entry must fail.
	badZip := filepath.Join(tmp, "bad.zip")
	bf, _ := os.Create(badZip)
	bzw := zip.NewWriter(bf)
	bw, _ := bzw.Create("../escape.txt")
	_, _ = bw.Write([]byte("pwn"))
	_ = bzw.Close()
	_ = bf.Close()

	if _, err := ExtractZip(badZip, filepath.Join(tmp, "badout")); err == nil {
		t.Fatalf("expected zip-slip error, got nil")
	}
}

func TestDownloadReleaseIgnoresShortClientTimeoutAndHonorsCtxCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "12")
		fl, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("chunk1"))
		if fl != nil {
			fl.Flush()
		}
		time.Sleep(120 * time.Millisecond)
		_, _ = w.Write([]byte("chunk2"))
	}))
	defer ts.Close()

	// HTTPClient carries a tiny 40ms Timeout (like NewKernelManager's 20s API timeout),
	// yet DownloadRelease must NOT abort at 40ms.
	mgr := &KernelManager{
		HTTPClient: &http.Client{Timeout: 40 * time.Millisecond},
	}
	dest := t.TempDir()
	out, err := mgr.DownloadRelease(context.Background(), ReleaseAsset{
		Name:        "fp-test.zip",
		DownloadURL: ts.URL,
		Size:        12,
	}, dest, nil)
	if err != nil {
		t.Fatalf("DownloadRelease failed due to inherited client timeout: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil || string(b) != "chunk1chunk2" {
		t.Fatalf("downloaded content = %q err=%v", string(b), err)
	}

	// Explicit context cancellation must still abort cleanly and clean up .part file.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := mgr.DownloadRelease(ctx, ReleaseAsset{
		Name:        "cancelled.zip",
		DownloadURL: ts.URL,
	}, dest, nil); err == nil {
		t.Fatalf("expected context cancellation error")
	}
	if _, err := os.Stat(filepath.Join(dest, "cancelled.zip.part")); !os.IsNotExist(err) {
		t.Fatalf("temp .part file not cleaned up after cancel")
	}
}

func TestExtractZipSymlinksDotDotFilePriorityAndManagedSubdir(t *testing.T) {
	tmp := t.TempDir()

	// 1. Archive with: legal "..keep.txt", helper binary inside Frameworks/, real Contents/MacOS/Chromium, and a safe relative symlink.
	goodZip := filepath.Join(tmp, "bundle.zip")
	f, err := os.Create(goodZip)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	addEntry := func(name, content string, mode os.FileMode) {
		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
		hdr.SetMode(mode)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(content))
	}
	addEntry("..keep.txt", "legal-name", 0o644)
	addEntry("Chromium.app/Contents/Frameworks/Helper.app/Contents/MacOS/chrome", "helper", 0o755)
	addEntry("Chromium.app/Contents/MacOS/Chromium", "#!/bin/sh\n", 0o644) // missing +x bit should be fixed
	addEntry("Chromium.app/Contents/Frameworks/Versions/A/lib.dylib", "lib", 0o644)
	addEntry("Chromium.app/Contents/Frameworks/Versions/Current", "A", os.ModeSymlink|0o777)
	_ = zw.Close()
	_ = f.Close()

	outDir := filepath.Join(tmp, "extracted")
	exe, err := ExtractZip(goodZip, outDir)
	if err != nil {
		t.Fatalf("ExtractZip(bundle): %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(exe), "Chromium.app/Contents/MacOS/Chromium") {
		t.Fatalf("expected main Chromium executable, got %q", exe)
	}
	if fi, err := os.Stat(exe); err != nil || fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("extracted executable missing +x bit: mode=%v err=%v", fi.Mode(), err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "..keep.txt")); err != nil {
		t.Fatalf("legal ..keep.txt not extracted: %v", err)
	}
	if target, err := os.Readlink(filepath.Join(outDir, "Chromium.app/Contents/Frameworks/Versions/Current")); err != nil || target != "A" {
		t.Fatalf("symlink not preserved: target=%q err=%v", target, err)
	}

	// 2. Archive with escaping symlink must be rejected.
	badSymZip := filepath.Join(tmp, "badsym.zip")
	bf, _ := os.Create(badSymZip)
	bzw := zip.NewWriter(bf)
	hdr := &zip.FileHeader{Name: "sub/link"}
	hdr.SetMode(os.ModeSymlink | 0o777)
	bw, _ := bzw.CreateHeader(hdr)
	_, _ = bw.Write([]byte("../../../etc/passwd"))
	_ = bzw.Close()
	_ = bf.Close()
	if _, err := ExtractZip(badSymZip, filepath.Join(tmp, "badsym-out")); err == nil {
		t.Fatalf("expected error for escaping symlink")
	}

	// 3. ManagedKernelDir 1-level subdirectory discovery.
	dataDir := t.TempDir()
	nestedExe := filepath.Join(dataDir, "kernels", "fingerprint-chromium", "148.0.7778.215_linux_x64", "chrome")
	if err := os.MkdirAll(filepath.Dir(nestedExe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nestedExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	mgr := &KernelManager{
		DataDir:    dataDir,
		GOOS:       "linux",
		GOARCH:     "amd64",
		Getenv:     func(string) string { return "" },
		RunVersion: func(context.Context, string) (string, error) { return "Chromium 148 fingerprint", nil },
	}
	pref, err := mgr.ResolvePreferred()
	if err != nil || pref.Path != nestedExe {
		t.Fatalf("expected nested managed binary %q, got %+v err=%v", nestedExe, pref, err)
	}
}

func TestFetchLatestReleaseFallsBackAcrossReleasesAndAppImageChmod(t *testing.T) {
	// Simulate exact live adryfish/fingerprint-chromium release layout:
	// - 150.0.7871.186: Windows zip/exe + Linux AppImage + Linux tar.xz (+ .zsync), NO macOS build
	// - 149.0.0.0: draft release with macOS dmg (must be ignored)
	// - 148.0.7778.215: macOS dmg (ungoogled-chromium_148.0.7778.215-1.1_macos.dmg)
	var (
		ts         *httptest.Server
		listHits   int
		singleHits int
	)
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			_ = json.NewEncoder(w).Encode(ReleaseInfo{
				TagName: "150.0.7871.186",
				Assets: []ReleaseAsset{
					{Name: "ungoogled-chromium_150.0.7871.186-1.1_windows_x64.zip", DownloadURL: ts.URL + "/dl/win.zip", Size: 100},
					{Name: "ungoogled-chromium-150.0.7871.186-1-x86_64_linux.tar.xz", DownloadURL: ts.URL + "/dl/linux.tar.xz", Size: 210},
					{Name: "ungoogled-chromium-150.0.7871.186-1-x86_64.AppImage.zsync", DownloadURL: ts.URL + "/dl/zsync", Size: 10},
					{Name: "ungoogled-chromium-150.0.7871.186-1-x86_64.AppImage", DownloadURL: ts.URL + "/dl/appimage", Size: 200},
				},
			})
		case "/releases":
			listHits++
			_ = json.NewEncoder(w).Encode([]ReleaseInfo{
				{
					TagName: "150.0.7871.186",
					Assets: []ReleaseAsset{
						{Name: "ungoogled-chromium_150.0.7871.186-1.1_windows_x64.zip", DownloadURL: ts.URL + "/dl/win.zip", Size: 100},
						{Name: "ungoogled-chromium-150.0.7871.186-1-x86_64_linux.tar.xz", DownloadURL: ts.URL + "/dl/linux.tar.xz", Size: 210},
						{Name: "ungoogled-chromium-150.0.7871.186-1-x86_64.AppImage.zsync", DownloadURL: ts.URL + "/dl/zsync", Size: 10},
						{Name: "ungoogled-chromium-150.0.7871.186-1-x86_64.AppImage", DownloadURL: ts.URL + "/dl/appimage", Size: 200},
					},
				},
				{
					TagName: "149.0.0.0",
					Draft:   true,
					Assets: []ReleaseAsset{
						{Name: "ungoogled-chromium_149.0.0.0-1.1_macos.dmg", DownloadURL: ts.URL + "/dl/draft.dmg", Size: 300},
					},
				},
				{
					TagName: "148.0.7778.215",
					Assets: []ReleaseAsset{
						{Name: "ungoogled-chromium_148.0.7778.215-1.1_macos.dmg", DownloadURL: ts.URL + "/dl/mac.dmg", Size: 400},
					},
				},
			})
		case "/single-unmatched":
			singleHits++
			_ = json.NewEncoder(w).Encode(ReleaseInfo{
				TagName: "999.0.0.1",
				Assets: []ReleaseAsset{
					{Name: "ungoogled-chromium_999.0.0.1_windows_x64.zip", DownloadURL: ts.URL + "/dl/win.zip", Size: 100},
				},
			})
		case "/dl/appimage":
			_, _ = w.Write([]byte("#!/bin/sh\necho ok\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// 1. Default ReleasesAPIURL path on macOS arm64 matches 148.0.7778.215 and records LatestTag=150.0.7871.186.
	macMgr := &KernelManager{
		GOOS:           "darwin",
		GOARCH:         "arm64",
		HTTPClient:     ts.Client(),
		ReleasesAPIURL: ts.URL + "/releases",
	}
	macRel, err := macMgr.FetchLatestRelease(context.Background())
	if err != nil {
		t.Fatalf("FetchLatestRelease(darwin list): %v", err)
	}
	if macRel.TagName != "148.0.7778.215" || macRel.LatestTag != "150.0.7871.186" ||
		macRel.Matched == nil || macRel.Matched.Name != "ungoogled-chromium_148.0.7778.215-1.1_macos.dmg" {
		t.Fatalf("macOS release mismatch: %+v", macRel)
	}

	// 2. Production NewKernelManager with customized LatestAPIURL=/releases/latest must fall back
	//    to the SAME test server's /releases (NOT leak out to live UpstreamReleasesAPI!).
	hitsBefore := listHits
	macFallbackMgr := NewKernelManager(t.TempDir(), "")
	macFallbackMgr.GOOS = "darwin"
	macFallbackMgr.GOARCH = "arm64"
	macFallbackMgr.HTTPClient = ts.Client()
	macFallbackMgr.LatestAPIURL = ts.URL + "/releases/latest"
	macRel2, err := macFallbackMgr.FetchLatestRelease(context.Background())
	if err != nil {
		t.Fatalf("FetchLatestRelease(NewKernelManager darwin fallback): %v", err)
	}
	if listHits != hitsBefore+1 {
		t.Fatalf("expected fallback to hit custom test server /releases once; hits delta = %d", listHits-hitsBefore)
	}
	if macRel2.TagName != "148.0.7778.215" || macRel2.LatestTag != "150.0.7871.186" || macRel2.Matched == nil {
		t.Fatalf("macOS fallback release mismatch: %+v", macRel2)
	}

	// 2b. Production NewKernelManager with non-/releases/latest custom LatestAPIURL must NOT leak to live GitHub.
	macSingleMgr := NewKernelManager(t.TempDir(), "")
	macSingleMgr.GOOS = "darwin"
	macSingleMgr.GOARCH = "arm64"
	macSingleMgr.HTTPClient = ts.Client()
	macSingleMgr.LatestAPIURL = ts.URL + "/single-unmatched"
	macSingle, err := macSingleMgr.FetchLatestRelease(context.Background())
	if err != nil {
		t.Fatalf("FetchLatestRelease(single unmatched): %v", err)
	}
	if singleHits != 1 || macSingle.TagName != "999.0.0.1" || macSingle.Matched != nil {
		t.Fatalf("non-standard custom LatestAPIURL should stay on custom server without fallback: %+v", macSingle)
	}

	// 3. Linux amd64 with BOTH .AppImage and _linux.tar.xz prefers .AppImage (ignoring .zsync),
	//    DownloadRelease sets +x, and DetectAll discovers it.
	dataDir := t.TempDir()
	linuxMgr := &KernelManager{
		DataDir:        dataDir,
		GOOS:           "linux",
		GOARCH:         "amd64",
		Getenv:         func(string) string { return "" },
		RunVersion:     func(context.Context, string) (string, error) { return "Chromium 150 fingerprint", nil },
		HTTPClient:     ts.Client(),
		ReleasesAPIURL: ts.URL + "/releases",
	}
	linuxRel, err := linuxMgr.FetchLatestRelease(context.Background())
	if err != nil {
		t.Fatalf("FetchLatestRelease(linux): %v", err)
	}
	if linuxRel.TagName != "150.0.7871.186" || linuxRel.LatestTag != "" ||
		linuxRel.Matched == nil || linuxRel.Matched.Name != "ungoogled-chromium-150.0.7871.186-1-x86_64.AppImage" {
		t.Fatalf("linux release mismatch (expected .AppImage over .tar.xz): %+v", linuxRel)
	}
	dlPath, err := linuxMgr.DownloadRelease(context.Background(), *linuxRel.Matched, linuxMgr.ManagedKernelDir(), nil)
	if err != nil {
		t.Fatalf("DownloadRelease(AppImage): %v", err)
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(dlPath); err != nil || fi.Mode().Perm()&0o111 == 0 {
			t.Fatalf("AppImage missing executable permission: mode=%v err=%v", fi.Mode(), err)
		}
	}
	pref, err := linuxMgr.ResolvePreferred()
	if err != nil || pref.Path != dlPath || pref.Source != "managed" {
		t.Fatalf("ResolvePreferred did not discover managed AppImage: pref=%+v err=%v", pref, err)
	}

	// 4. Linux arm64 must reject x86_64-only Linux assets, and Windows must reject darwin .zip assets.
	linuxArmMgr := &KernelManager{
		GOOS:           "linux",
		GOARCH:         "arm64",
		HTTPClient:     ts.Client(),
		ReleasesAPIURL: ts.URL + "/releases",
	}
	armRel, err := linuxArmMgr.FetchLatestRelease(context.Background())
	if err != nil {
		t.Fatalf("FetchLatestRelease(linux/arm64): %v", err)
	}
	if armRel.Matched != nil {
		t.Fatalf("linux/arm64 must not match x86_64 Linux binary: %+v", armRel.Matched)
	}
	if got := SelectReleaseAsset([]ReleaseAsset{{Name: "chromium_darwin_arm64.zip"}}, "windows", "amd64"); got != nil {
		t.Fatalf("windows must not match darwin .zip asset: %+v", got)
	}
}
