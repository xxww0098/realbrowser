package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func browserZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	entry, err := w.Create("browser/chrome.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("browser fixture")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestManagedBrowserEnsureAndUpdate(t *testing.T) {
	archive := browserZip(t)
	var downloads atomic.Int32
	var version atomic.Int32
	version.Store(151)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/archive" {
			downloads.Add(1)
			if version.Load() == 152 {
				_, _ = w.Write([]byte("damaged"))
			} else {
				_, _ = w.Write(archive)
			}
			return
		}
		asset := ReleaseAsset{Name: "chromium_windows_arm64.zip", DownloadURL: "http://" + r.Host + "/archive", Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(archive))}
		_ = json.NewEncoder(w).Encode([]ReleaseInfo{{TagName: fmt.Sprint(version.Load()), Assets: []ReleaseAsset{asset}}})
	}))
	defer srv.Close()
	b := Store{Dir: t.TempDir(), Manager: &KernelManager{GOOS: "windows", GOARCH: "arm64", ReleasesAPIURL: srv.URL, HTTPClient: srv.Client()}}
	ctx := t.Context()
	first, err := b.Ensure(ctx, nil)
	if err != nil || first.Version != "151" || first.Executable == "" {
		t.Fatalf("first install: %+v, %v", first, err)
	}
	version.Store(152)
	if _, err := b.Update(ctx, nil); err == nil {
		t.Fatal("corrupt upgrade accepted")
	}
	if current, ok := b.Installed(); !ok || current != first {
		t.Fatalf("last good installation lost: %+v", current)
	}
	srv.Close()
	if cached, err := b.Ensure(ctx, nil); err != nil || cached != first || downloads.Load() != 2 {
		t.Fatalf("offline reuse: %+v, %v, downloads %d", cached, err, downloads.Load())
	}
	legacyExe := filepath.Join(t.TempDir(), "legacy")
	if err := os.WriteFile(legacyExe, []byte("legacy"), 0o700); err != nil {
		t.Fatal(err)
	}
	b.Base = InstalledRelease{Version: "155", Executable: legacyExe}
	if current, err := b.Ensure(ctx, nil); err != nil || current != b.Base {
		t.Fatalf("newer legacy browser lost: %+v, %v", current, err)
	}
	if current, err := b.Install(ctx, &ReleaseInfo{TagName: "154", Matched: &ReleaseAsset{}}, nil); err != nil || current != b.Base {
		t.Fatalf("refused downgrade lost executable: %+v, %v", current, err)
	}
	if _, err := (Store{}).Ensure(ctx, nil); err == nil {
		t.Fatal("accepted an empty install directory")
	}
}

func TestManagedBrowserPinnedSources(t *testing.T) {
	data := browserZip(t)
	build := PinnedBuild{File: "browser.zip", RelExe: "chrome.exe", SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
	ctx := t.Context()
	first := func(_ context.Context, _, dest string) error { return os.WriteFile(dest, []byte("wrong file"), 0o600) }
	second := func(_ context.Context, _, dest string) error { return os.WriteFile(dest, data, 0o600) }
	source := PinnedSource{Downloads: []func(context.Context, string, string) error{first, second}}
	stage := t.TempDir()
	exe, err := build.install(ctx, &KernelManager{}, stage, source, nil)
	if err != nil || exe != filepath.Join(stage, "chrome.exe") {
		t.Fatalf("verified transport fallback: %q, %v", exe, err)
	}
	if _, err := os.Stat(filepath.Join(stage, build.File)); !os.IsNotExist(err) {
		t.Fatalf("download archive retained: %v", err)
	}
	offline := filepath.Join(t.TempDir(), "offline-archive")
	if err := os.WriteFile(offline, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := build.install(ctx, &KernelManager{}, t.TempDir(), PinnedSource{Archive: offline}, nil); err != nil {
		t.Fatal(err)
	}
	if saved, err := os.ReadFile(offline); err != nil || !bytes.Equal(saved, data) {
		t.Fatal("caller-owned offline archive changed")
	}
	b := Store{Dir: t.TempDir(), Manager: &KernelManager{GOOS: "windows", GOARCH: "amd64"}, Pinned: PinnedSource{Archive: offline}}
	if _, err := b.Ensure(ctx, nil); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("unverified pin: %v", err)
	}
	if _, ok := b.Installed(); ok {
		t.Fatal("activated unverified pin")
	}
}
