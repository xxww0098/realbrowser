package browserkernel

import (
	"archive/tar"
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

const maxArchiveFileBytes int64 = 2 << 30
const maxArchiveEntries = 50000

var maxArchiveExtractBytes int64 = 4 << 30

// A pinned build has one enclosing directory and a known executable.
// Release archives retain their complete upstream layout, including bundle links.
type archiveOutput struct {
	root      *os.Root
	pinned    bool
	remaining int64
	entries   int
	best      string
	score     int
	links     []string
}

func (out *archiveOutput) name(raw string, allowWrapperOnly bool) (string, error) {
	out.entries++
	if out.entries > maxArchiveEntries {
		return "", errors.New("browserkernel: archive exceeds maximum entry count")
	}
	raw = strings.ReplaceAll(raw, "\\", "/")
	for strings.HasPrefix(raw, "./") {
		raw = strings.TrimPrefix(raw, "./")
	}
	if raw == "" || raw == "." {
		return "", nil
	}
	name := filepath.FromSlash(raw)
	if !filepath.IsLocal(name) || strings.Contains(strings.SplitN(raw, "/", 2)[0], ":") {
		return "", fmt.Errorf("browserkernel: invalid archive path %q", raw)
	}
	if out.pinned {
		_, rel, _ := strings.Cut(raw, "/")
		if rel == "" && allowWrapperOnly {
			return "", nil
		}
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			return "", fmt.Errorf("browserkernel: missing or invalid path inside archive directory %q", raw)
		}
		name = filepath.FromSlash(rel)
	}
	return filepath.Clean(name), nil
}

func (out *archiveOutput) file(ctx context.Context, name string, size int64, mode os.FileMode, src io.Reader) error {
	if size < 0 || size > maxArchiveFileBytes || size > out.remaining {
		return errors.New("browserkernel: archive exceeds maximum extraction size limit")
	}
	if mode == 0 {
		mode = 0o644
	}
	if out.pinned {
		mode &= 0o755
	} else {
		mode |= 0o200
	}
	if err := out.root.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return err
	}
	f, err := out.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	written, err := io.Copy(f, io.LimitReader(contextReader{ctx, src}, size+1))
	if err = errors.Join(err, f.Close()); err != nil {
		return err
	}
	if written != size {
		return fmt.Errorf("browserkernel: wrong size for archive entry %q", name)
	}
	out.remaining -= written
	if sc := scoreExtractedExecutable(name); sc > out.score {
		out.best, out.score = name, sc
	}
	return nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func (out *archiveOutput) symlink(name, target string) error {
	target = strings.TrimSpace(target)
	if target == "" || strings.HasPrefix(target, "/") || strings.ContainsAny(target, "\\:") || filepath.IsAbs(target) ||
		!filepath.IsLocal(filepath.Join(filepath.Dir(name), filepath.FromSlash(target))) {
		return fmt.Errorf("browserkernel: invalid archive link %q -> %q", name, target)
	}
	if err := out.root.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return err
	}
	if err := out.root.Symlink(filepath.FromSlash(target), name); err != nil {
		return err
	}
	out.links = append(out.links, name)
	return nil
}

func (out *archiveOutput) zip(ctx context.Context, archive string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, entry := range z.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		dir := entry.FileInfo().IsDir()
		name, err := out.name(entry.Name, true)
		if err != nil {
			return err
		}
		if name == "" {
			continue
		}
		if dir {
			if err := out.root.MkdirAll(name, 0o700); err != nil {
				return err
			}
			continue
		}
		r, err := entry.Open()
		if err != nil {
			return err
		}
		switch {
		case entry.Mode().IsRegular():
			err = out.file(ctx, name, int64(entry.UncompressedSize64), entry.Mode().Perm(), r)
		case entry.Mode()&os.ModeSymlink != 0 && !out.pinned && entry.UncompressedSize64 <= 4096:
			var link []byte
			link, err = io.ReadAll(io.LimitReader(r, 4097))
			if err == nil {
				err = out.symlink(name, string(link))
			}
		default:
			err = fmt.Errorf("browserkernel: unsupported zip entry %q", name)
		}
		if err = errors.Join(err, r.Close()); err != nil {
			return err
		}
	}
	return nil
}

func (out *archiveOutput) tarXZ(ctx context.Context, archive string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	xr, err := xz.NewReader(contextReader{ctx, f})
	if err != nil {
		return err
	}
	tr := tar.NewReader(xr)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if strings.Contains(h.Name, "\\") {
			return fmt.Errorf("browserkernel: invalid tar path %q", h.Name)
		}
		name, err := out.name(h.Name, h.Typeflag == tar.TypeDir)
		if err != nil {
			return err
		}
		if name == "" {
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			err = out.root.MkdirAll(name, 0o700)
		case tar.TypeReg, tar.TypeRegA:
			err = out.file(ctx, name, h.Size, h.FileInfo().Mode().Perm(), tr)
		case tar.TypeSymlink:
			err = out.symlink(name, h.Linkname)
		case tar.TypeLink:
			target := filepath.FromSlash(h.Linkname)
			if out.pinned || !filepath.IsLocal(target) {
				return fmt.Errorf("browserkernel: invalid hard link %q", h.Name)
			}
			info, err := out.root.Lstat(target)
			if err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("browserkernel: invalid hard link target %q", target)
			}
			if err := out.root.MkdirAll(filepath.Dir(name), 0o700); err != nil {
				return err
			}
			if err := out.root.Link(target, name); err != nil {
				return err
			}
		default:
			return fmt.Errorf("browserkernel: unsupported tar entry %q", name)
		}
		if err != nil {
			return err
		}
	}
}

func unpack(ctx context.Context, archive, formatName, dest string, pinned bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return "", err
	}
	if strings.HasSuffix(strings.ToLower(formatName), ".dmg") {
		return unpackDMG(ctx, archive, dest)
	}
	root, err := os.OpenRoot(dest)
	if err != nil {
		return "", err
	}
	defer root.Close()
	out := archiveOutput{root: root, pinned: pinned, remaining: maxArchiveExtractBytes}
	if pinned {
		out.remaining = min(out.remaining, maxArchiveFileBytes)
	}
	switch {
	case strings.HasSuffix(strings.ToLower(formatName), ".zip"):
		err = out.zip(ctx, archive)
	case strings.HasSuffix(strings.ToLower(formatName), ".tar.xz"):
		err = out.tarXZ(ctx, archive)
	default:
		return "", fmt.Errorf("browserkernel: unsupported archive %q", formatName)
	}
	if err != nil {
		return "", err
	}
	for _, link := range out.links {
		if _, err := root.Stat(link); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if out.best == "" {
		return "", nil
	}
	info, err := root.Stat(out.best)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(out.best), ".exe") && info.Mode().Perm()&0o111 == 0 {
		if err := root.Chmod(out.best, info.Mode().Perm()|0o755); err != nil {
			return "", err
		}
	}
	return filepath.Join(dest, out.best), nil
}

// Unpack extracts an already verified pinned archive into a fresh staging directory.
// The format comes from the release manifest, even for an extensionless offline archive.
func (b PinnedBuild) Unpack(ctx context.Context, archive, dest string) error {
	if _, err := unpack(ctx, archive, b.File, dest, true); err != nil {
		return err
	}
	if _, ok := ManagedExecutable(dest, filepath.FromSlash(b.RelExe)); !ok {
		return fmt.Errorf("browserkernel: archive is missing %s", b.RelExe)
	}
	return nil
}

// ManagedExecutable resolves a non-empty binary inside an application's real managed directory.
func ManagedExecutable(dir, relative string) (string, bool) {
	if !filepath.IsLocal(relative) {
		return "", false
	}
	st, err := os.Lstat(dir)
	if err != nil || !st.IsDir() {
		return "", false
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", false
	}
	defer root.Close()
	st, err = root.Stat(relative)
	if err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
		return "", false
	}
	return filepath.Join(dir, relative), true
}

func scoreExtractedExecutable(path string) int {
	slash := "/" + filepath.ToSlash(path)
	lowerSlash := strings.ToLower(slash)
	// Skip macOS helper sub-bundles inside Frameworks/.
	if strings.Contains(lowerSlash, "/frameworks/") {
		return 0
	}
	base := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasSuffix(lowerSlash, "/contents/macos/chromium") || strings.HasSuffix(lowerSlash, "/contents/macos/fingerprint chromium"):
		return 100
	case base == "chrome.exe" || base == "chromium.exe":
		return 90
	case base == "fingerprint-chromium":
		return 85
	case base == "chromium":
		return 80
	case base == "chrome":
		return 75
	default:
		return 0
	}
}
