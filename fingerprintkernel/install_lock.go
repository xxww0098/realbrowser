package fingerprintkernel

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// The launcher shares its browser and fingerprint cache across app processes.
// Lock a stable file, rather than the temporary install directory, so a process
// crash releases the OS lock and concurrent installers do not remove each
// other's partially extracted files.
func AcquireInstallLock(ctx context.Context, root string, onWait func()) (*os.File, error) {
	return acquireFileLock(ctx, filepath.Join(root, ".install.lock"), onWait)
}

func acquireFileLock(ctx context.Context, path string, onWait func()) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	waited := false
	for {
		if err := ctx.Err(); err != nil {
			_ = file.Close()
			return nil, err
		}
		locked, err := tryInstallLock(file)
		if err != nil {
			_ = file.Close()
			return nil, err
		}
		if locked {
			return file, nil
		}
		if !waited && onWait != nil {
			onWait()
		}
		waited = true
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = file.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func ReleaseInstallLock(file *os.File) {
	if file != nil {
		_ = unlockInstallFile(file)
		_ = file.Close()
	}
}
