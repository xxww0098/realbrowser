package browserkernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// PinnedVersion pins the official adryfish/fingerprint-chromium release
// that ships Windows x64, macOS Apple Silicon, and Linux x64 builds without
// the 150.x canvas readback renderer crash.
const PinnedVersion = "148.0.7778.215"

const pinnedReleaseBase = UpstreamRepoURL + "/releases/download/" + PinnedVersion

// PinnedBuild describes the checked release and the executable in its installed layout.
type PinnedBuild struct {
	File   string
	SHA256 string
	RelExe string
}

// SHA-256 digests published on github.com/adryfish/fingerprint-chromium/releases/tag/148.0.7778.215.
var pinnedBuilds = map[string]PinnedBuild{
	"darwin/arm64": {
		File:   "ungoogled-chromium_148.0.7778.215-1.1_macos.dmg",
		SHA256: "b72f091e2e1a7583eed389c4b8e3534ed355e568af8c8bbf8fc30a25e23ca679",
		RelExe: "Chromium.app/Contents/MacOS/Chromium",
	},
	"windows/amd64": {
		File:   "ungoogled-chromium_148.0.7778.215-1.1_windows_x64.zip",
		SHA256: "9ef3f471b7a6641b4224532522b29141ce3746e27d55788d88e2fd951f362579",
		RelExe: "chrome.exe",
	},
	"linux/amd64": {
		File:   "ungoogled-chromium-148.0.7778.215-1-x86_64_linux.tar.xz",
		SHA256: "70d239830332e5820aa34dfcb284161cac0429eee25da642830afe04bda717f4",
		RelExe: "chrome",
	},
}

// PinnedRelease returns the managed build for an explicit OS and architecture.
func PinnedRelease(goos, goarch string) (PinnedBuild, bool) {
	b, ok := pinnedBuilds[goos+"/"+goarch]
	return b, ok
}

func (b PinnedBuild) Asset() ReleaseAsset {
	return ReleaseAsset{Name: b.File, DownloadURL: pinnedReleaseBase + "/" + b.File, SHA256: b.SHA256}
}

// VerifyFileSHA256 checks a local release archive against its published digest.
func VerifyFileSHA256(ctx context.Context, path, expectedHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, contextReader{ctx, f}); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(expectedHex)) {
		return fmt.Errorf("browserkernel: sha256 mismatch (got %s, want %s)", got, expectedHex)
	}
	return nil
}
