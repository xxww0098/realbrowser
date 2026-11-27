// Package filestore provides cross-process locking and atomic file replacement.
package filestore

import (
	"context"
	"os"
	"time"
)

// Acquire locks a stable file until Release or process exit.
func Acquire(ctx context.Context, path string, onWait func()) (*os.File, error) {
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

func Release(file *os.File) {
	if file != nil {
		_ = unlockInstallFile(file)
		_ = file.Close()
	}
}
