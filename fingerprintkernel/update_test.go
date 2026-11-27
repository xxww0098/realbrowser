package fingerprintkernel

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

func TestFingerprintPreferencesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	f, err := ReadProfile(filepath.Join(dir, "new"))
	if err != nil || f.Lang != "" {
		t.Fatalf("legacy defaults: %+v, %v", f, err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".browserkernel.json"), []byte(`{"lang":"en-GB"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if f, err = ReadProfile(dir); err != nil || f.Lang != "en-GB" {
		t.Fatalf("existing profile: %+v, %v", f, err)
	}
	f.Timezone, f.WindowWidth, f.WindowHeight = "Europe/London", 720, 480
	f.ColorScheme = ColorSchemeDark
	if err = WriteProfile(dir, f); err != nil {
		t.Fatal(err)
	}
	got, err := ReadProfile(dir)
	if err != nil || got.Lang != "en-GB" || got.Timezone != f.Timezone || got.WindowWidth != 720 || got.ColorScheme != ColorSchemeDark {
		t.Fatalf("read preferences: %+v %v", got, err)
	}
	f.WindowHeight = -1
	if err = WriteProfile(dir, f); err == nil {
		t.Fatal("saved invalid dimensions")
	}
	if got, err = ReadProfile(dir); err != nil || got.WindowHeight != 480 {
		t.Fatalf("previous preferences lost: %+v %v", got, err)
	}
}

func TestAutomaticUpdateSelectsAnInstallableArchitecture(t *testing.T) {
	win64 := []ReleaseAsset{{Name: "chromium_windows_x64.zip"}}
	releases := []ReleaseInfo{
		{TagName: "153", Assets: []ReleaseAsset{{Name: "chromium_windows_x64.exe"}}},
		{TagName: "latest-build", Assets: win64},
		{TagName: "150.0.9", Assets: win64},
		{TagName: "154", Draft: true, Assets: win64},
		{TagName: "155", Prerelease: true, Assets: win64},
		{TagName: "152", Assets: []ReleaseAsset{{Name: "chromium_windows_arm64.zip"}}},
		{TagName: "151.0.2", Assets: win64},
		{TagName: "v151.0.10", Assets: []ReleaseAsset{{Name: "chromium_windows_x64.zip"}, {Name: "SHA256SUMS", DownloadURL: "https://example.test/sums"}}},
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
