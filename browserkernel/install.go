package browserkernel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CanInstallArchive reports whether this host can install a release automatically.
func CanInstallArchive(goos, name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar.xz") ||
		strings.HasSuffix(lower, ".appimage") || (goos == "darwin" && strings.HasSuffix(lower, ".dmg"))
}

// InstallReleaseArchive installs a release in its upstream layout. Unknown formats
// are left for manual installation. The returned path is ready to launch.
func InstallReleaseArchive(ctx context.Context, archive, dest string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	lower := strings.ToLower(archive)
	switch {
	case strings.HasSuffix(lower, ".appimage"):
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return archive, os.Chmod(archive, 0o755)
	case strings.HasSuffix(lower, ".zip"), strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".dmg"):
		return installArchive(ctx, archive, lower, dest)
	default:
		return "", nil
	}
}

func ExtractZip(archive, dest string) (string, error) {
	return installArchive(context.Background(), archive, ".zip", dest)
}

func ExtractTarXZ(archive, dest string) (string, error) {
	return installArchive(context.Background(), archive, ".tar.xz", dest)
}

func InstallDMG(ctx context.Context, archive, dest string) (string, error) {
	return installArchive(ctx, archive, ".dmg", dest)
}

func installArchive(ctx context.Context, archive, format, dest string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(dest) == "" {
		return "", errors.New("browserkernel: install directory required")
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(dest, ".install-")
	if err != nil {
		return "", err
	}
	payload := filepath.Join(stage, "payload")
	// If rollback itself fails, keep the previous version in the named recovery directory.
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(stage)
		}
	}()
	exe, err := unpack(ctx, archive, format, payload, false)
	if err != nil {
		return "", err
	}
	if exe != "" {
		_ = ClearMacQuarantine(ctx, exe)
		exe, err = filepath.Rel(payload, exe)
		if err != nil || !filepath.IsLocal(exe) {
			return "", errors.New("browserkernel: invalid installed executable")
		}
		exe = filepath.Join(dest, exe)
	}
	entries, err := os.ReadDir(payload)
	if err != nil {
		return "", err
	}
	archiveAbs, err := filepath.Abs(archive)
	if err != nil {
		return "", err
	}
	for _, ent := range entries {
		name := ent.Name()
		target, err := filepath.Abs(filepath.Join(dest, name))
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(strings.ToLower(name), ".install") || strings.EqualFold(target, archiveAbs) {
			return "", fmt.Errorf("browserkernel: reserved install path %q", name)
		}
	}
	lock, err := AcquireInstallLock(ctx, dest, nil)
	if err != nil {
		return "", err
	}
	defer ReleaseInstallLock(lock)
	root, err := os.OpenRoot(dest)
	if err != nil {
		return "", err
	}
	defer root.Close()
	prefix := filepath.Base(stage)
	previous := filepath.Join(prefix, "previous")
	payloadRel := filepath.Join(prefix, "payload")
	var moved, saved []string
	err = filepath.WalkDir(payload, func(src string, ent os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name, err := filepath.Rel(payload, src)
		if err != nil || name == "." {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err := root.Lstat(name)
		if err == nil {
			// Merge an existing release directory without dropping unmentioned files.
			// A DMG app bundle is replaced as a unit.
			if ent.IsDir() && current.IsDir() && !strings.HasSuffix(strings.ToLower(format), ".dmg") {
				return nil
			}
			if current.IsDir() != ent.IsDir() {
				return fmt.Errorf("browserkernel: conflicting file type at %s", name)
			}
			old := filepath.Join(previous, name)
			if err := root.MkdirAll(filepath.Dir(old), 0o700); err != nil {
				return err
			}
			if err := root.Rename(name, old); err != nil {
				return err
			}
			saved = append(saved, name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := root.Rename(filepath.Join(payloadRel, name), name); err != nil {
			return err
		}
		moved = append(moved, name)
		if ent.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	if err == nil {
		return exe, nil
	}
	var restore error
	for i := len(moved) - 1; i >= 0; i-- {
		name := moved[i]
		restore = errors.Join(restore, root.Rename(name, filepath.Join(payloadRel, name)))
	}
	for i := len(saved) - 1; i >= 0; i-- {
		name := saved[i]
		restore = errors.Join(restore, root.Rename(filepath.Join(previous, name), name))
	}
	if restore != nil {
		cleanup = false
		return "", errors.Join(err, fmt.Errorf("browserkernel: recover previous files from %s: %w", filepath.Join(dest, previous), restore))
	}
	return "", err
}
