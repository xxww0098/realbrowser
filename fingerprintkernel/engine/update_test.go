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
	"testing"
)

func TestUpdateKeepsLastGoodVersion(t *testing.T) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	entry, err := z.Create("version/chrome.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = entry.Write([]byte("fixture executable")); err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	var tag = "149.0.0.1"
	digest := fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))
	downloads := 0
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/asset" {
			downloads++
			_, _ = w.Write(buf.Bytes())
			return
		}
		assets := []ReleaseAsset{{Name: "chromium_windows_x64.zip", DownloadURL: srv.URL + "/asset", Digest: "sha256:" + digest}}
		backport := ReleaseInfo{TagName: "147.9.0.8", Assets: assets}
		if r.URL.Path == "/releases/latest" {
			_ = json.NewEncoder(w).Encode(backport)
			return
		}
		_ = json.NewEncoder(w).Encode([]ReleaseInfo{backport, {TagName: tag, Assets: assets}})
	}))
	defer srv.Close()
	m := KernelManager{GOOS: "windows", GOARCH: "amd64", HTTPClient: srv.Client(), ReleasesAPIURL: srv.URL}
	dir := t.TempDir()
	for _, url := range []string{"", srv.URL + "/releases/latest"} {
		m.LatestAPIURL = url
		if available, err := m.CheckUpdate(t.Context(), "148.0.0.1"); err != nil || available != tag || downloads != 0 {
			t.Fatalf("check: %q %v", available, err)
		}
	}
	old, changed, err := m.Update(t.Context(), dir, "148.0.0.1", nil)
	if err != nil || !changed || old.Version != tag {
		t.Fatalf("first update: %+v %v %v", old, changed, err)
	}
	if _, changed, err = m.Update(t.Context(), dir, "148.0.0.1", nil); err != nil || changed || downloads != 1 {
		t.Fatalf("repeat update: %v %v downloads=%d", changed, err, downloads)
	}
	tag = "151.0.0.1"
	valid := digest
	digest = fmt.Sprintf("%064x", 1)
	if _, _, err = m.Update(t.Context(), dir, old.Version, nil); err == nil {
		t.Fatal("accepted damaged update")
	}
	if got, ok := ActiveRelease(dir); !ok || got != old {
		t.Fatalf("lost active release: %+v", got)
	}
	digest = valid
	next, changed, err := m.Update(t.Context(), dir, old.Version, nil)
	if err != nil || !changed || next.Executable == old.Executable {
		t.Fatalf("retry: %+v %v", next, err)
	}
	if _, err = os.Stat(old.Executable); err != nil {
		t.Fatal("removed executable belonging to existing instances")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err = m.Update(ctx, dir, next.Version, nil); err == nil {
		t.Fatal("cancelled update succeeded")
	}
	if got, ok := ActiveRelease(dir); !ok || got != next {
		t.Fatalf("cancel changed active version: %+v", got)
	}
}

func TestAutomaticUpdateSelectsAnInstallableArchitecture(t *testing.T) {
	win64 := []ReleaseAsset{{Name: "chromium_windows_x64.zip", DownloadURL: "https://example.test/browser.zip", Digest: fmt.Sprintf("sha256:%064x", 1)}}
	releases := []ReleaseInfo{
		{TagName: "153", Assets: []ReleaseAsset{{Name: "chromium_windows_x64.exe"}}},
		{TagName: "latest-build", Assets: win64},
		{TagName: "150.0.9", Assets: win64},
		{TagName: "154", Draft: true, Assets: win64},
		{TagName: "155", Prerelease: true, Assets: win64},
		{TagName: "152", Assets: []ReleaseAsset{{Name: "chromium_windows_arm64.zip"}}},
		{TagName: "151.0.2", Assets: win64},
		{TagName: "v151.0.10", Assets: []ReleaseAsset{{Name: "chromium_windows_x64.zip", DownloadURL: "https://example.test/browser.zip"}, {Name: "SHA256SUMS", DownloadURL: "https://example.test/sums"}}},
	}
	m := KernelManager{GOOS: "windows", GOARCH: "amd64"}
	auto, ok := m.pickRelease(releases, true)
	if !ok || auto.TagName != "v151.0.10" || auto.LatestTag != "153" || auto.Matched.ChecksumURL == "" {
		t.Fatalf("automatic candidate = %+v", auto)
	}
	manual, ok := m.pickRelease(releases, false)
	if !ok || manual.TagName != "153" {
		t.Fatalf("manual candidate = %+v", manual)
	}
}

func TestAutomaticUpdateRejectsUnusableAssets(t *testing.T) {
	digest := fmt.Sprintf("sha256:%064x", 1)
	asset := func(name, checksum string, size int64) ReleaseAsset {
		return ReleaseAsset{Name: name, DownloadURL: "https://example.test/" + name, Digest: checksum, Size: size}
	}
	m := KernelManager{GOOS: "linux", GOARCH: "arm64"}
	releases := []ReleaseInfo{
		{TagName: "155", Assets: []ReleaseAsset{asset("chromium-linux-arm64.tar.xz", digest, 32)}},
		{TagName: "154", Assets: []ReleaseAsset{asset("chromium-linux.tar.xz", digest, 0)}},
		{TagName: "153", Assets: []ReleaseAsset{asset("chromium-linux-arm64.tar.xz", "", 0)}},
		{TagName: "152", Assets: []ReleaseAsset{asset("chromium-linux-arm64.AppImage", "", 0), asset("chromium-linux-aarch64.tar.xz", "", 0)}, Body: "chromium-linux-aarch64.tar.xz " + digest},
	}
	rel, ok := m.pickRelease(releases, true)
	if !ok || rel.TagName != "152" || rel.LatestTag != "155" || rel.Matched.Name != "chromium-linux-aarch64.tar.xz" {
		t.Fatalf("update candidate: %+v", rel)
	}
	if rel, ok := m.pickRelease(releases[:3], true); ok || rel == nil || rel.Matched != nil {
		t.Fatalf("unexpected automatic download: %+v", rel)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("attempted to download a known empty archive")
	}))
	defer srv.Close()
	empty := releases[0]
	empty.Matched = &empty.Assets[0]
	empty.Matched.DownloadURL = srv.URL
	if _, changed, err := m.ActivateRelease(t.Context(), t.TempDir(), "", &empty, nil); err == nil || changed {
		t.Fatalf("activated an empty archive: changed=%v err=%v", changed, err)
	}

	pinned, _ := PinnedRelease("darwin", "arm64")
	m.GOOS, m.GOARCH = "darwin", "arm64"
	releases = []ReleaseInfo{{TagName: PinnedVersion, Assets: []ReleaseAsset{pinned.Asset()}}}
	releases[0].Assets[0].SHA256 = ""
	if rel, ok := m.pickRelease(releases, true); !ok || rel.Matched.SHA256 != pinned.SHA256 {
		t.Fatal("lost verified macOS ARM64 build with an unqualified filename")
	}
	releases[0].Assets[0].Digest = digest
	if _, ok := m.pickRelease(releases, true); ok {
		t.Fatal("accepted changed bytes for a pinned build")
	}
	releases[0].Assets[0] = pinned.Asset()
	m.GOARCH = "amd64"
	if _, ok := m.pickRelease(releases, true); ok {
		t.Fatal("advertised an ARM64 pinned build to macOS x64")
	}
	releases[0].Assets = []ReleaseAsset{asset("chromium-macos-universal2.dmg", digest, 0)}
	if _, ok := m.pickRelease(releases, true); !ok {
		t.Fatal("lost an explicitly universal macOS build")
	}
}
