package fingerprintkernel_test

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

	kernel "github.com/xxww0098/realbrowser/fingerprintkernel"
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
			asset := kernel.ReleaseAsset{Name: "windows-x64.zip", DownloadURL: "http://" + r.Host + "/" + tag, ChecksumURL: "http://" + r.Host + "/sum"}
			_ = json.NewEncoder(w).Encode([]kernel.ReleaseInfo{{TagName: tag, Assets: []kernel.ReleaseAsset{asset}}})
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
	manager := kernel.KernelManager{GOOS: "windows", GOARCH: "amd64", ReleasesAPIURL: server.URL + "/releases", HTTPClient: server.Client()}
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
	if _, ok := kernel.ActiveRelease(fresh); ok {
		t.Fatal("activated the replaced archive")
	}
}

func TestAvailableUpdateEmptyCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "[]")
	}))
	manager := kernel.KernelManager{ReleasesAPIURL: server.URL, HTTPClient: server.Client()}
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

func TestMergeFingerprintJSON(t *testing.T) {
	const original = `{"lang":"en","timezone":"UTC","later":9007199254740993,"permissions":{"later":true}}`
	spec := kernel.FingerprintSpec{Lang: "fr-FR"}
	raw, err := kernel.MergeFingerprintJSON(original, spec)
	var saved struct {
		Lang, Timezone string
		Later          int64
		Permissions    struct{ Later bool }
	}
	if err != nil || json.Unmarshal([]byte(raw), &saved) != nil || saved.Lang != "fr-FR" || saved.Timezone != "" || saved.Later != 9007199254740993 || !saved.Permissions.Later {
		t.Fatalf("merged: %s, %v", raw, err)
	}
	if fresh, err := kernel.MergeFingerprintJSON("", spec); err != nil || !json.Valid([]byte(fresh)) {
		t.Fatalf("new document: %q, %v", fresh, err)
	}
	if raw, err := kernel.MergeFingerprintJSON("{", spec); err == nil || raw != "" {
		t.Fatalf("malformed source: %q, %v", raw, err)
	}
	spec.HardwareConcurrency = -1
	if raw, err := kernel.MergeFingerprintJSON(original, spec); err == nil || raw != "" {
		t.Fatalf("invalid spec: %q, %v", raw, err)
	}
}
