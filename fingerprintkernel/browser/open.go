// Package browser prepares and opens managed fingerprint browser sessions.
package browser

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/xxww0098/realbrowser/fingerprintkernel/engine"
	"github.com/xxww0098/realbrowser/fingerprintkernel/internal/filestore"
	"github.com/xxww0098/realbrowser/fingerprintkernel/process"
	"github.com/xxww0098/realbrowser/fingerprintkernel/profile"
)

// ManagedBrowser opens profiles using the installations in its Store.
type ManagedBrowser struct {
	engine.Store
}

// New manages the browser installation in dir. Profiles use separate directories.
func New(dir string) ManagedBrowser {
	return ManagedBrowser{Store: engine.Store{Dir: dir}}
}

// Open prepares the installed browser and the profile, then opens the supplied
// startup URLs. On first use it installs the browser and creates the profile.
// ctx covers preparation and startup. The returned Process owns the running
// session and can be waited on or terminated independently.
func (b ManagedBrowser) Open(ctx context.Context, profileDir string, urls ...string) (*process.Process, error) {
	cmd, err := b.openCommand(ctx, profileDir, urls)
	if err != nil {
		return nil, err
	}
	return process.StartProcess(cmd, process.DetachedBrowser)
}

func (b ManagedBrowser) openCommand(ctx context.Context, profileDir string, urls []string) (*exec.Cmd, error) {
	if !filestore.ValidDir(b.Dir) || !filestore.ValidDir(profileDir) {
		return nil, errors.New("缺少浏览器安装目录或资料目录")
	}
	for _, raw := range urls {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || !u.IsAbs() {
			return nil, fmt.Errorf("无效的启动 URL %q", raw)
		}
	}
	dir, err := filepath.Abs(profileDir)
	if err != nil {
		return nil, err
	}
	spec, err := profile.EnsureProfile(ctx, dir, profile.EnsureOptions{})
	if err != nil {
		return nil, err
	}
	active, err := b.Ensure(ctx, nil)
	if err != nil {
		return nil, err
	}
	path, err := filepath.Abs(active.Executable)
	if err != nil {
		return nil, err
	}
	options := LaunchOptions{UserDataDir: dir, Fingerprint: spec, StartURLs: urls}
	args, env, err := options.Prepare(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Env = make([]string, 0, len(env))
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	slices.Sort(cmd.Env)
	return cmd, nil
}
