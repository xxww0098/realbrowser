package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/xxww0098/realbrowser/fingerprintkernel/internal/filestore"
)

// InstalledRelease identifies the next browser to launch. Old release directories
// are retained so currently running processes can continue loading their resources.
type InstalledRelease struct {
	Version    string
	Executable string
}

type activation struct {
	Version string `json:"version"`
	Path    string `json:"path"`
}

func versionParts(tag string) ([]int, error) {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(tag), "v"), ".")
	if len(parts) == 0 || len(parts) > 4 {
		return nil, fmt.Errorf("invalid Chromium version %q", tag)
	}
	result := make([]int, 4)
	for i, part := range parts {
		n, err := strconv.ParseUint(part, 10, 31)
		if err != nil {
			return nil, fmt.Errorf("invalid Chromium version %q", tag)
		}
		result[i] = int(n)
	}
	return result, nil
}

func ActiveRelease(dir string) (InstalledRelease, bool) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return InstalledRelease{}, false
	}
	defer root.Close()
	raw, err := root.ReadFile("active.json")
	var marker activation
	if err != nil || json.Unmarshal(raw, &marker) != nil {
		return InstalledRelease{}, false
	}
	if _, err := versionParts(marker.Version); err != nil {
		return InstalledRelease{}, false
	}
	relative := filepath.FromSlash(marker.Path)
	if !strings.HasPrefix(marker.Path, "versions/") || filepath.ToSlash(filepath.Clean(relative)) != marker.Path {
		return InstalledRelease{}, false
	}
	exe, ok := ManagedExecutable(dir, relative)
	if !ok {
		return InstalledRelease{}, false
	}
	return InstalledRelease{Version: marker.Version, Executable: exe}, true
}

// Update installs a newer compatible stable release and atomically activates it.
// current is the version supplied by an existing installation, or empty on first use.
func (m *KernelManager) Update(ctx context.Context, dir, current string, progress func(int64, int64)) (InstalledRelease, bool, error) {
	rel, err := m.FetchInstallableRelease(ctx)
	if err != nil {
		return InstalledRelease{}, false, err
	}
	return m.ActivateRelease(ctx, dir, current, rel, progress)
}

// ActivateRelease installs exactly the version the host displayed to its user.
func (m *KernelManager) ActivateRelease(ctx context.Context, dir, current string, rel *ReleaseInfo, progress func(int64, int64)) (InstalledRelease, bool, error) {
	if rel == nil || rel.Matched == nil || rel.Draft || rel.Prerelease {
		return InstalledRelease{}, false, errors.New("没有适用于当前平台的稳定内核")
	}
	if m.AcceptRelease != nil && !m.AcceptRelease(rel.TagName) {
		return InstalledRelease{}, false, errors.New("内核版本与当前应用不兼容")
	}
	return activate(ctx, dir, current, rel.TagName, func(stage string) (string, error) {
		asset := *rel.Matched
		if asset.emptyXZArchive() {
			return "", errors.New("发行包过小，无法包含浏览器")
		}
		if !asset.hasChecksum() {
			return "", errors.New("发行包缺少 SHA-256 校验值")
		}
		archive, err := m.DownloadRelease(ctx, asset, stage, progress)
		if err != nil {
			return "", err
		}
		exe, err := InstallReleaseArchive(ctx, archive, stage)
		if err != nil {
			return "", err
		}
		if exe == "" {
			return "", errors.New("发行包中没有可启动的浏览器")
		}
		if exe != archive {
			if err := os.Remove(archive); err != nil {
				return "", err
			}
		}
		return exe, nil
	})
}

func activate(ctx context.Context, dir, current, tag string, install func(string) (string, error)) (InstalledRelease, bool, error) {
	target, err := versionParts(tag)
	if err != nil {
		return InstalledRelease{}, false, err
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return InstalledRelease{}, false, err
	}
	lock, err := AcquireInstallLock(ctx, dir, nil)
	if err != nil {
		return InstalledRelease{}, false, err
	}
	defer ReleaseInstallLock(lock)
	active, _ := ActiveRelease(dir)
	latest := InstalledRelease{Version: current}
	if active.Version != "" {
		a, _ := versionParts(active.Version)
		c, cerr := versionParts(current)
		if current == "" || (cerr == nil && slices.Compare(a, c) >= 0) {
			latest = active
		}
	}
	if latest.Version != "" {
		v, err := versionParts(latest.Version)
		if err != nil {
			return InstalledRelease{}, false, err
		}
		if slices.Compare(target, v) <= 0 {
			return latest, false, nil
		}
	}
	versions := filepath.Join(dir, "versions")
	if err := os.MkdirAll(versions, 0o700); err != nil {
		return InstalledRelease{}, false, err
	}
	if info, err := os.Lstat(versions); err != nil || !info.IsDir() {
		return InstalledRelease{}, false, errors.New("内核版本目录无效")
	}
	stage, err := os.MkdirTemp(versions, "release-")
	if err != nil {
		return InstalledRelease{}, false, err
	}
	activated := false
	defer func() {
		if !activated {
			_ = os.RemoveAll(stage)
		}
	}()
	exe, err := install(stage)
	if err != nil {
		return InstalledRelease{}, false, err
	}
	path, err := filepath.Rel(dir, exe)
	if err != nil {
		return InstalledRelease{}, false, err
	}
	if _, ok := ManagedExecutable(dir, path); !ok {
		return InstalledRelease{}, false, errors.New("内核可执行文件无效")
	}
	if err := ctx.Err(); err != nil {
		return InstalledRelease{}, false, err
	}
	data, err := json.Marshal(activation{Version: tag, Path: filepath.ToSlash(path)})
	if err != nil {
		return InstalledRelease{}, false, err
	}
	if err := filestore.WriteAtomic(dir, "active.json", data); err != nil {
		return InstalledRelease{}, false, err
	}
	activated = true
	return InstalledRelease{Version: tag, Executable: exe}, true, nil
}

// CheckUpdate returns the newest compatible version if it is newer than current.
func (m *KernelManager) CheckUpdate(ctx context.Context, current string) (string, error) {
	rel, err := m.newerRelease(ctx, current)
	if err != nil || rel == nil {
		return "", err
	}
	return rel.TagName, nil
}

// AvailableUpdate returns the installable release newer than current, or nil
// when none is available. Pass an empty current version for a first install.
// It resolves the selected asset's SHA-256 (fetching a checksum file if needed)
// without downloading the browser. Keep the result and pass it to ActivateRelease
// after user confirmation to install it without querying the release list again.
func (m *KernelManager) AvailableUpdate(ctx context.Context, current string) (*ReleaseInfo, error) {
	rel, err := m.newerRelease(ctx, current)
	if errors.Is(err, errNoReleases) {
		return nil, nil
	}
	if err != nil || rel == nil {
		return nil, err
	}
	sha, err := rel.Matched.expectedSHA256(ctx, m.HTTPClient)
	if err != nil {
		return nil, err
	}
	rel.Matched.SHA256 = sha
	return rel, nil
}

func (m *KernelManager) newerRelease(ctx context.Context, current string) (*ReleaseInfo, error) {
	rel, err := m.FetchInstallableRelease(ctx)
	if err != nil || rel == nil || rel.Matched == nil {
		return nil, err
	}
	if current == "" {
		return rel, nil
	}
	next, err := versionParts(rel.TagName)
	if err != nil {
		return nil, err
	}
	old, err := versionParts(current)
	if err != nil {
		return nil, err
	}
	if slices.Compare(next, old) > 0 {
		return rel, nil
	}
	return nil, nil
}
