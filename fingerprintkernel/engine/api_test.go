package engine_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/xxww0098/realbrowser/fingerprintkernel/engine"
)

func TestAvailableUpdateInstallsTheReviewedAsset(t *testing.T) {
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	entry, err := w.Create("chrome.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("browser fixture")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(archive.Bytes()))
	var version, checks, checksums, downloads atomic.Int32
	var corrupt atomic.Bool
	version.Store(152)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			checks.Add(1)
			tag := fmt.Sprint(version.Load())
			asset := engine.ReleaseAsset{Name: "windows-x64.zip", DownloadURL: "http://" + r.Host + "/" + tag, ChecksumURL: "http://" + r.Host + "/sum"}
			_ = json.NewEncoder(w).Encode([]engine.ReleaseInfo{{TagName: tag, Assets: []engine.ReleaseAsset{asset}}})
		case "/sum":
			checksums.Add(1)
			if version.Load() != 152 {
				t.Error("checksum fetched after confirmation")
			}
			fmt.Fprintln(w, digest, " windows-x64.zip")
		case "/152":
			downloads.Add(1)
			if corrupt.Load() {
				fmt.Fprint(w, "replaced bytes")
				return
			}
			_, _ = w.Write(archive.Bytes())
		default:
			t.Errorf("unexpected download: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	manager := engine.KernelManager{GOOS: "windows", GOARCH: "amd64", ReleasesAPIURL: server.URL + "/releases", HTTPClient: server.Client()}
	ctx := t.Context()
	if version, err := manager.CheckUpdate(ctx, "151"); err != nil || version != "152" || checksums.Load() != 0 {
		t.Fatalf("legacy metadata check: %s, %v", version, err)
	}
	for _, current := range []string{"v152.0", "153"} {
		if release, err := manager.AvailableUpdate(ctx, current); err != nil || release != nil {
			t.Fatalf("current %s: release=%+v err=%v", current, release, err)
		}
	}
	if _, err := manager.AvailableUpdate(ctx, "invalid"); err == nil {
		t.Fatal("accepted an invalid installed version")
	}
	release, err := manager.AvailableUpdate(ctx, "")
	if err != nil || release == nil || release.TagName != "152" || release.Matched == nil || release.Matched.SHA256 != digest || downloads.Load() != 0 || checksums.Load() != 1 {
		t.Fatalf("check: release=%+v, downloads=%d, err=%v", release, downloads.Load(), err)
	}
	checked := checks.Load()
	version.Store(153)
	dir := t.TempDir()
	installed, changed, err := manager.ActivateRelease(ctx, dir, "151", release, nil)
	if err != nil || !changed || installed.Version != "152" || checks.Load() != checked || downloads.Load() != 1 {
		t.Fatalf("apply: %+v changed=%v checks=%d downloads=%d err=%v", installed, changed, checks.Load(), downloads.Load(), err)
	}
	corrupt.Store(true)
	fresh := t.TempDir()
	if _, changed, err := manager.ActivateRelease(ctx, fresh, "", release, nil); err == nil || changed {
		t.Fatalf("accepted replacement bytes: changed=%v err=%v", changed, err)
	}
	if _, ok := engine.ActiveRelease(fresh); ok {
		t.Fatal("activated the replaced archive")
	}
}

func TestAvailableUpdateEmptyCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "[]")
	}))
	manager := engine.KernelManager{ReleasesAPIURL: server.URL, HTTPClient: server.Client()}
	defer server.Close()
	if rel, err := manager.AvailableUpdate(t.Context(), "151"); rel != nil || err != nil {
		t.Fatalf("empty catalog: %+v, %v", rel, err)
	}
	if _, err := manager.CheckUpdate(t.Context(), "151"); err == nil {
		t.Fatal("changed legacy empty-catalog error")
	}
	server.Close()
	if _, err := manager.AvailableUpdate(t.Context(), "151"); err == nil {
		t.Fatal("hid an unavailable release feed")
	}
}
