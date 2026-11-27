package fingerprintkernel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ManagedBrowser owns the installed browser in Dir. Setting Dir is sufficient
// for a new application. The zero values of the other fields use the official
// source and the current platform.
type ManagedBrowser struct {
	Dir string
	// Base optionally identifies a browser installed by an older host. The
	// host supplies its verified executable to keep existing data in place.
	Base InstalledRelease
	// Manager optionally supplies the host's HTTP client and release policy.
	Manager *KernelManager
	// Pinned optionally supplies an offline archive or a custom transport for
	// first installation. The pinned manifest always supplies the digest.
	Pinned PinnedSource
}

// PinnedSource customizes how Ensure obtains the pinned archive. An offline
// archive takes priority and stays owned by the caller. Each transport writes the complete archive to destination.
// Ensure tries the next transport if the transfer or digest check fails.
type PinnedSource struct {
	Archive   string
	Downloads []func(ctx context.Context, url, destination string) error
}

func (b ManagedBrowser) manager() *KernelManager {
	if b.Manager != nil {
		return b.Manager
	}
	return NewKernelManager("", "")
}

func (b ManagedBrowser) validDir() bool {
	return strings.TrimSpace(b.Dir) != "" && filepath.Clean(b.Dir) != "."
}

// Installed returns the usable installation without contacting the network.
func (b ManagedBrowser) Installed() (InstalledRelease, bool) {
	if !b.validDir() {
		return InstalledRelease{}, false
	}
	active, ok := ActiveRelease(b.Dir)
	if b.Base.Executable == "" {
		return active, ok
	}
	fi, err := os.Stat(b.Base.Executable)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() == 0 {
		return active, ok
	}
	if ok {
		baseVersion, err := versionParts(b.Base.Version)
		activeVersion, _ := versionParts(active.Version)
		if err != nil || slices.Compare(activeVersion, baseVersion) >= 0 {
			return active, true
		}
	}
	return b.Base, true
}

// Ensure reuses an installed browser offline. On first use it installs the
// pinned build, or a compatible stable release when no pin fits the platform
// and policy. Progress is optional.
func (b ManagedBrowser) Ensure(ctx context.Context, progress func(done, total int64)) (InstalledRelease, error) {
	if err := ctx.Err(); err != nil {
		return InstalledRelease{}, err
	}
	if !b.validDir() {
		return InstalledRelease{}, errors.New("缺少浏览器安装目录")
	}
	if current, ok := b.Installed(); ok {
		return current, nil
	}
	m := b.manager()
	build, supported := PinnedRelease(m.goos(), m.goarch())
	if !supported || (m.AcceptRelease != nil && !m.AcceptRelease(PinnedVersion)) || !strings.HasSuffix(strings.ToLower(build.File), strings.ToLower(m.ArchiveSuffix)) {
		if b.Pinned.Archive != "" || len(b.Pinned.Downloads) > 0 {
			return InstalledRelease{}, fmt.Errorf("no compatible pinned browser for %s/%s", m.goos(), m.goarch())
		}
		return b.Update(ctx, progress)
	}
	active, _, err := activate(ctx, b.Dir, "", PinnedVersion, func(stage string) (string, error) {
		return build.install(ctx, m, stage, b.Pinned, progress)
	})
	return active, err
}

func (build PinnedBuild) install(ctx context.Context, m *KernelManager, stage string, source PinnedSource, progress func(int64, int64)) (string, error) {
	asset := build.Asset()
	var archive string
	var err error
	switch {
	case source.Archive != "":
		// Snapshot a caller-owned file so the bytes we unpack are the same
		// bytes we verified even if the original path is replaced.
		archive, err = asset.SaveDownload(ctx, stage, func(dest string) error {
			src, err := os.Open(source.Archive)
			if err != nil {
				return err
			}
			defer src.Close()
			f, err := os.OpenFile(dest, os.O_WRONLY|os.O_TRUNC, 0)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, contextReader{ctx, src})
			return errors.Join(err, f.Close())
		})
	case len(source.Downloads) == 0:
		archive, err = m.DownloadRelease(ctx, asset, stage, progress)
	default:
		for _, download := range source.Downloads {
			if download == nil {
				return "", errors.New("missing browser download transport")
			}
			archive, err = asset.SaveDownload(ctx, stage, func(dest string) error {
				return download(ctx, asset.DownloadURL, dest)
			})
			if err == nil || ctx.Err() != nil {
				break
			}
		}
	}
	if err != nil {
		return "", err
	}
	if err := build.Unpack(ctx, archive, stage); err != nil {
		return "", err
	}
	if err := os.Remove(archive); err != nil {
		return "", err
	}
	return filepath.Join(stage, filepath.FromSlash(build.RelExe)), nil
}

// Update obtains and installs the latest compatible stable release.
func (b ManagedBrowser) Update(ctx context.Context, progress func(done, total int64)) (InstalledRelease, error) {
	if !b.validDir() {
		return InstalledRelease{}, errors.New("缺少浏览器安装目录")
	}
	m := b.manager()
	rel, err := m.FetchInstallableRelease(ctx)
	if err != nil {
		return InstalledRelease{}, err
	}
	b.Manager = m
	return b.Install(ctx, rel, progress)
}

// Install activates exactly the release selected by an interactive host.
func (b ManagedBrowser) Install(ctx context.Context, release *ReleaseInfo, progress func(done, total int64)) (InstalledRelease, error) {
	if !b.validDir() {
		return InstalledRelease{}, errors.New("缺少浏览器安装目录")
	}
	current, _ := b.Installed()
	active, _, err := b.manager().ActivateRelease(ctx, b.Dir, current.Version, release, progress)
	if err == nil && active.Executable == "" {
		return current, nil
	}
	return active, err
}

// DownloadedAsset describes the result of an explicitly selected download.
// Path is the retained archive, or the active executable for a managed version.
type DownloadedAsset struct {
	Path string
	InstalledRelease
	Unpacked bool
}

// Download handles a catalog selection for a download UI. A matched stable
// browser is installed through the versioned store. Other explicitly chosen
// assets keep their original filename and use the manual archive installer.
func (b ManagedBrowser) Download(ctx context.Context, shown *ReleaseInfo, asset ReleaseAsset, progress func(done, total int64)) (DownloadedAsset, error) {
	if !b.validDir() {
		return DownloadedAsset{}, errors.New("缺少浏览器安装目录")
	}
	m := b.manager()
	b.Manager = m
	if shown != nil {
		matched := shown.Matched != nil && shown.Matched.Name == asset.Name
		if matched {
			asset = *shown.Matched
		}
		if asset.ChecksumURL == "" {
			if checksum := FindChecksumAsset(shown.Assets, asset.Name); checksum != nil {
				asset.ChecksumURL = checksum.DownloadURL
			}
		}
		if matched && shown.TagName != "" && CanInstallArchive(m.goos(), asset.Name) {
			chosen := *shown
			chosen.Matched = &asset
			active, err := b.Install(ctx, &chosen, progress)
			return DownloadedAsset{Path: active.Executable, InstalledRelease: active}, err
		}
	}
	path, err := m.DownloadRelease(ctx, asset, b.Dir, progress)
	if err != nil {
		return DownloadedAsset{}, err
	}
	result := DownloadedAsset{Path: path}
	if CanInstallArchive(m.goos(), path) {
		result.Executable, err = InstallReleaseArchive(ctx, path, b.Dir)
		result.Unpacked = err == nil
	}
	return result, err
}
