//go:build darwin

package fingerprintkernel

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func quarantineTargetPath(binPath string) string {
	p := filepath.Clean(strings.TrimSpace(binPath))
	if p == "" {
		return ""
	}
	// Resolve .../Foo.app/Contents/MacOS/Foo -> .../Foo.app so quarantine checks cover the entire bundle.
	lower := strings.ToLower(p)
	if idx := strings.Index(lower, ".app/"); idx > 0 {
		return p[:idx+4]
	}
	return p
}

func detectMacQuarantine(binPath string) bool {
	target := quarantineTargetPath(binPath)
	if target == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "xattr", "-p", "com.apple.quarantine", target).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

// ClearMacQuarantine removes macOS com.apple.quarantine extended attributes from
// an app bundle or executable (solves macOS Gatekeeper "Chromium.app is damaged" launches).
func ClearMacQuarantine(ctx context.Context, binPath string) error {
	target := quarantineTargetPath(binPath)
	if target == "" {
		return nil
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}
	return exec.CommandContext(ctx, "xattr", "-dr", "com.apple.quarantine", target).Run()
}
