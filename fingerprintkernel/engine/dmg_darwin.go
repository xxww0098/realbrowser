//go:build darwin

package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// unpackDMG mounts a macOS .dmg disk image read-only without opening Finder,
// copies the embedded Chromium/Fingerprint Chromium .app bundle into destDir via ditto,
// detaches the image cleanly, clears com.apple.quarantine, and returns the executable path.
func unpackDMG(ctx context.Context, dmgPath, destDir string) (string, error) {
	if strings.TrimSpace(dmgPath) == "" || strings.TrimSpace(destDir) == "" {
		return "", errors.New("fpchromium: dmg path and destination dir are required")
	}
	fi, err := os.Stat(dmgPath)
	if err != nil || fi.IsDir() {
		return "", fmt.Errorf("fpchromium: dmg file unavailable: %s", dmgPath)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cleanDest := filepath.Clean(destDir)
	if err := os.MkdirAll(cleanDest, 0o755); err != nil {
		return "", fmt.Errorf("fpchromium: create install dir: %w", err)
	}

	mountDir, err := os.MkdirTemp("", "fingerprintkernel-dmg-*")
	if err != nil {
		return "", fmt.Errorf("fpchromium: create dmg mount point: %w", err)
	}
	defer func() { _ = os.RemoveAll(mountDir) }()

	attachCmd := exec.CommandContext(ctx, "hdiutil", "attach",
		"-nobrowse", "-readonly", "-noautoopen", "-noverify",
		"-mountpoint", mountDir, dmgPath)
	attachCmd.Stdin = strings.NewReader("Y\n")
	if out, err := attachCmd.CombinedOutput(); err != nil {
		// If canceled right after mounting, detach defensively before RemoveAll(mountDir).
		if ctx.Err() != nil {
			dctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = exec.CommandContext(dctx, "hdiutil", "detach", "-force", mountDir).Run()
			cancel()
		}
		return "", fmt.Errorf("fpchromium: hdiutil attach %s: %w (%s)", filepath.Base(dmgPath), err, strings.TrimSpace(string(out)))
	}
	defer func() {
		dctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := exec.CommandContext(dctx, "hdiutil", "detach", mountDir).Run(); err != nil {
			_ = exec.CommandContext(dctx, "hdiutil", "detach", "-force", mountDir).Run()
		}
	}()

	srcApp, err := findAppBundleInDir(mountDir)
	if err != nil {
		return "", err
	}

	appBase := filepath.Base(srcApp)
	dstApp := filepath.Join(cleanDest, appBase)
	ditto := exec.CommandContext(ctx, "ditto", srcApp, dstApp)
	if out, err := ditto.CombinedOutput(); err != nil {
		return "", fmt.Errorf("fingerprintkernel: copy %s: %w (%s)", appBase, err, strings.TrimSpace(string(out)))
	}
	exe := NormalizeBinaryPath("darwin", dstApp)
	rel, err := filepath.Rel(dstApp, exe)
	if err != nil {
		return "", err
	}
	if _, ok := ManagedExecutable(dstApp, rel); !ok {
		return "", fmt.Errorf("fingerprintkernel: installed bundle %s missing executable", appBase)
	}
	if fi, err := os.Stat(exe); err == nil && fi.Mode().Perm()&0o111 == 0 {
		_ = os.Chmod(exe, fi.Mode().Perm()|0o755)
	}
	// Automatically remove macOS Gatekeeper quarantine from the installed .app bundle.
	_ = ClearMacQuarantine(ctx, exe)
	return exe, nil
}

func findAppBundleInDir(root string) (string, error) {
	var (
		bestPath  string
		bestScore int
	)
	checkEntry := func(p string) {
		name := filepath.Base(p)
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(strings.ToLower(name), ".app") {
			return
		}
		exe := NormalizeBinaryPath("darwin", p)
		fi, err := os.Stat(exe)
		if err != nil || fi.IsDir() {
			return
		}
		ln := strings.ToLower(name)
		sc := 50
		switch {
		case ln == "fingerprint chromium.app":
			sc = 100
		case ln == "chromium.app":
			sc = 95
		case strings.Contains(ln, "fingerprint") || strings.Contains(ln, "chromium"):
			sc = 85
		}
		if sc > bestScore {
			bestScore = sc
			bestPath = p
		}
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("fpchromium: read mounted dmg: %w", err)
	}
	for _, ent := range entries {
		if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
			continue
		}
		full := filepath.Join(root, ent.Name())
		if strings.HasSuffix(strings.ToLower(ent.Name()), ".app") {
			checkEntry(full)
			continue
		}
		// Inspect one level deeper in case the DMG wraps Chromium.app in a versioned folder.
		if subEntries, serr := os.ReadDir(full); serr == nil {
			for _, sub := range subEntries {
				if sub.IsDir() && strings.HasSuffix(strings.ToLower(sub.Name()), ".app") {
					checkEntry(filepath.Join(full, sub.Name()))
				}
			}
		}
	}
	if bestPath == "" {
		return "", errors.New("fpchromium: no .app browser bundle found inside .dmg")
	}
	return bestPath, nil
}
