package engine

import (
	"context"
	"os"
	"path/filepath"

	"github.com/xxww0098/realbrowser/fingerprintkernel/internal/filestore"
)

// The launcher shares its browser and fingerprint cache across app processes.
// Lock a stable file, rather than the temporary install directory, so a process
// crash releases the OS lock and concurrent installers do not remove each
// other's partially extracted files.
func AcquireInstallLock(ctx context.Context, root string, onWait func()) (*os.File, error) {
	return filestore.Acquire(ctx, filepath.Join(root, ".install.lock"), onWait)
}

func ReleaseInstallLock(file *os.File) { filestore.Release(file) }
